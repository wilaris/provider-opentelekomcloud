package ccecluster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/url"
	"strings"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/cce/v3/clusters"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/cce/v3/nodepools"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/security/groups"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ccev1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/cce/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetClient         = "cannot get OTC provider client"
	errCreateCCEClient   = "cannot create CCE v3 client"
	errCreateNetV2Client = "cannot create Networking v2 client"
	errListSecGroups     = "cannot list security groups"
	errValidateSpec      = "invalid Cluster spec"
	errEmptyExternalName = "external name is empty"
	errEmptyVPCID        = "resolved vpcId is empty"
	errEmptySubnetID     = "resolved subnetId is empty"
	errObserveCluster    = "cannot observe Cluster"
	errCreateCluster     = "cannot create Cluster"
	errUpdateCluster     = "cannot update Cluster"
	errUpdateMasterIP    = "cannot update cluster master IP"
	errEIPIDRequired     = "EIP id is required when binding an EIP to the cluster"
	errUpdateConfig      = "cannot update cluster component configurations"
	errDeleteCluster     = "cannot delete Cluster"
)

// annotationLastAppliedComponentConfigs records a SHA-256 hash of the
// componentConfigurations we last applied to the cluster. The CCE GET API
// does not echo back configurationsOverride, so direct drift comparison is
// impossible.
const annotationLastAppliedComponentConfigs = "cce.opentelekomcloud.crossplane.io/last-applied-component-configs"

// SetupGated adds a controller that reconciles Cluster managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Cluster controller"))
		}
	}, ccev1alpha1.ClusterGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles CCE Cluster managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(ccev1alpha1.ClusterGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*ccev1alpha1.Cluster](&connector{
			kube: mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(
				mgr.GetClient(),
				&apisv1alpha1.ProviderConfigUsage{},
			),
			clientCache: clients.SharedCache(mgr.GetClient()),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		//nolint:staticcheck // controller-runtime recorder type mismatch with event.NewAPIRecorder.
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithCreationGracePeriod(5 * time.Minute),
		managed.WithPollIntervalHook(cceClusterPollInterval),
		managed.WithTimeout(35 * time.Minute),
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(),
			o.Logger,
			o.MetricOptions.MRStateMetrics,
			&ccev1alpha1.ClusterList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind ccev1alpha1.ClusterList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(ccev1alpha1.ClusterGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&ccev1alpha1.Cluster{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

func cceClusterPollInterval(mg resource.Managed, pollInterval time.Duration) time.Duration {
	cr, ok := mg.(*ccev1alpha1.Cluster)
	if !ok {
		return 30 * time.Second
	}
	if cr.GetDeletionTimestamp() != nil {
		return 30 * time.Second
	}
	if cr.Status.AtProvider.Status != "Available" {
		return 30 * time.Second
	}
	return pollInterval
}

var _ managed.TypedExternalConnector[*ccev1alpha1.Cluster] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *ccev1alpha1.Cluster,
) (managed.TypedExternalClient[*ccev1alpha1.Cluster], error) {
	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	providerConfig, cacheKey, err := clients.GetProviderConfigSpec(ctx, c.kube, mg)
	if err != nil {
		return nil, err
	}

	providerClient, err := c.clientCache.GetClient(ctx, cacheKey, providerConfig)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	endpointOpts := golangsdk.EndpointOpts{Region: providerClient.Region}
	cceClient, err := openstack.NewCCE(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateCCEClient)
	}
	networkV2Client, err := openstack.NewNetworkV2(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateNetV2Client)
	}

	return &external{
		cceClient:       cceClient,
		networkV2Client: networkV2Client,
	}, nil
}

var _ managed.TypedExternalClient[*ccev1alpha1.Cluster] = (*external)(nil)

type external struct {
	cceClient       *golangsdk.ServiceClient
	networkV2Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *ccev1alpha1.Cluster,
) (managed.ExternalObservation, error) {
	if err := validateClusterSpec(cr.Spec.ForProvider); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := clusters.Get(e.cceClient, externalName)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveCluster)
	}

	// set observation
	cr.Status.AtProvider = buildObservation(observed)

	// Discover CCE-managed control-plane and worker security groups. CCE
	// creates these out-of-band and tags them with the cluster ID in the
	// description; they aren't exposed on the cluster response directly.
	if err := e.populateSecurityGroups(&cr.Status.AtProvider, externalName); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errListSecGroups)
	}

	// set conditions
	e.setConditions(cr, observed.Status.Phase)

	// Fetch certificates only when the cluster is available. We treat transient
	// cert-endpoint errors as recoverable: return empty connection details and
	// let the next reconcile retry instead of blocking observation.
	var connDetails managed.ConnectionDetails
	if observed.Status.Phase == "Available" {
		if cert, err := clusters.GetCert(e.cceClient, externalName); err == nil {
			connDetails = buildConnectionDetails(
				cert,
				cr.Status.AtProvider.ExternalEndpoint,
				cr.Status.AtProvider.InternalEndpoint,
			)
		}
	}

	li := resource.NewLateInitializer()
	lateInitializeCluster(cr, observed, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isClusterUpToDate(
			cr.Spec.ForProvider,
			observed,
			cr.GetAnnotations()[annotationLastAppliedComponentConfigs],
		),
		ResourceLateInitialized: li.IsChanged(),
		ConnectionDetails:       connDetails,
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *ccev1alpha1.Cluster,
) (managed.ExternalCreation, error) {
	if meta.GetExternalName(cr) != "" {
		return managed.ExternalCreation{}, nil
	}

	spec := cr.Spec.ForProvider

	if err := validateClusterSpec(spec); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts := buildCreateOpts(spec)

	created, err := clusters.Create(e.cceClient, createOpts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateCluster)
	}

	meta.SetExternalName(cr, created.Metadata.Id)
	meta.AddAnnotations(cr, map[string]string{
		annotationLastAppliedComponentConfigs: componentConfigsHash(spec.ComponentConfigurations),
	})

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *ccev1alpha1.Cluster,
) (managed.ExternalUpdate, error) {
	spec := cr.Spec.ForProvider

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	if err := e.updateDescription(externalName, spec); err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := e.updateEIP(externalName, spec, cr.Status.AtProvider); err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := e.updateComponentConfigurations(externalName, spec); err != nil {
		return managed.ExternalUpdate{}, err
	}

	// Record the applied configs hash so the next Observe sees the resource as
	// up-to-date.
	meta.AddAnnotations(
		cr,
		map[string]string{
			annotationLastAppliedComponentConfigs: componentConfigsHash(
				spec.ComponentConfigurations,
			),
		},
	)

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *ccev1alpha1.Cluster,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	queryParams := buildDeleteQueryParams(cr.Spec.ForProvider)

	err := clusters.Delete(e.cceClient, externalName, queryParams)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteCluster)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

func (e *external) setConditions(cr *ccev1alpha1.Cluster, observedStatus string) {
	switch observedStatus {
	case "Available":
		cr.Status.SetConditions(xpv1.Available())
	case "Creating":
		cr.Status.SetConditions(xpv1.Creating())
	case "Deleting":
		cr.Status.SetConditions(xpv1.Deleting())
	default:
		cr.Status.SetConditions(xpv1.Unavailable())
	}
}

func validateClusterSpec(spec ccev1alpha1.ClusterParameters) error {
	if spec.VPCID == nil || *spec.VPCID == "" {
		return errors.New(errEmptyVPCID)
	}
	if spec.SubnetID == nil || *spec.SubnetID == "" {
		return errors.New(errEmptySubnetID)
	}
	return validateMasterCount(spec)
}

// validateMasterCount enforces the OTC flavor → master-count contract:
// - "s1" flavors are single-master (exactly 1 AZ entry)
// - "s2" flavors are high-availability (exactly 3)
func validateMasterCount(spec ccev1alpha1.ClusterParameters) error {
	if len(spec.Masters) == 0 {
		return nil
	}
	switch {
	case strings.Contains(spec.FlavorID, "s1") && len(spec.Masters) != 1:
		return errors.Errorf(
			"flavor %q is single-master and requires exactly 1 master AZ, got %d",
			spec.FlavorID, len(spec.Masters),
		)
	case strings.Contains(spec.FlavorID, "s2") && len(spec.Masters) != 3:
		return errors.Errorf(
			"flavor %q is high-availability and requires exactly 3 master AZs, got %d",
			spec.FlavorID, len(spec.Masters),
		)
	}
	return nil
}

func buildObservation(observed *clusters.Clusters) ccev1alpha1.ClusterObservation {
	obs := ccev1alpha1.ClusterObservation{
		ID:     observed.Metadata.Id,
		Status: observed.Status.Phase,
	}

	if len(observed.Status.Endpoints) > 0 {
		ep := observed.Status.Endpoints[0]
		obs.InternalEndpoint = ep.Internal
		obs.ExternalEndpoint = ep.External
		obs.ExternalOTCEndpoint = ep.ExternalOTC
	}

	obs.SupportIstio = observed.Spec.SupportIstio

	return obs
}

// populateSecurityGroups lists all security groups and matches those whose
// description contains the cluster ID together with either "master port"
// (control plane) or "node" (worker).
func (e *external) populateSecurityGroups(
	obs *ccev1alpha1.ClusterObservation,
	clusterID string,
) error {
	pages, err := groups.List(e.networkV2Client, groups.ListOpts{}).AllPages()
	if err != nil {
		return err
	}
	sgs, err := groups.ExtractGroups(pages)
	if err != nil {
		return err
	}

	for _, sg := range sgs {
		if obs.SecurityGroupControl != "" && obs.SecurityGroupNode != "" {
			break
		}
		if !strings.Contains(sg.Description, clusterID) {
			continue
		}
		switch {
		case strings.Contains(sg.Description, "master port"):
			obs.SecurityGroupControl = sg.ID
		case strings.Contains(sg.Description, "node"):
			obs.SecurityGroupNode = sg.ID
		}
	}
	return nil
}

func buildConnectionDetails(
	cert *clusters.Certificate,
	externalEndpoint, internalEndpoint string,
) managed.ConnectionDetails {
	if cert == nil {
		return nil
	}

	details := managed.ConnectionDetails{}

	endpoint := externalEndpoint
	if endpoint == "" {
		endpoint = internalEndpoint
	}
	if endpoint != "" {
		details["endpoint"] = []byte(endpoint)
	}

	if len(cert.Clusters) > 0 {
		details["certificate-authority-data"] = []byte(cert.Clusters[0].Cluster.CertAuthorityData)
	}
	if len(cert.Users) > 0 {
		details["client-certificate-data"] = []byte(cert.Users[0].User.ClientCertData)
		details["client-key-data"] = []byte(cert.Users[0].User.ClientKeyData)
	}

	// Marshal the full certificate struct as kubeconfig JSON
	kubeconfigJSON, err := json.Marshal(cert)
	if err == nil {
		details["kubeconfig"] = kubeconfigJSON
	}

	return details
}

func lateInitializeCluster(
	cr *ccev1alpha1.Cluster,
	observed *clusters.Clusters,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider

	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Spec.Description, li)
	p.ClusterVersion = util.LateInitPtrIfNonZero(p.ClusterVersion, observed.Spec.Version, li)
	p.AuthenticationMode = util.LateInitPtrIfNonZero(
		p.AuthenticationMode,
		observed.Spec.Authentication.Mode,
		li,
	)
	p.ContainerNetworkCIDR = util.LateInitPtrIfNonZero(
		p.ContainerNetworkCIDR,
		observed.Spec.ContainerNetwork.Cidr,
		li,
	)
	p.KubeProxyMode = util.LateInitPtrIfNonZero(p.KubeProxyMode, observed.Spec.KubeProxyMode, li)
	p.KubernetesSvcIPRange = util.LateInitPtrIfNonZero(
		p.KubernetesSvcIPRange,
		observed.Spec.KubernetesSvcIpRange,
		li,
	)
	p.Timezone = util.LateInitPtrIfNonZero(p.Timezone, observed.Metadata.Timezone, li)
	p.Labels = util.LateInitMapIfNonEmpty(p.Labels, observed.Metadata.Labels, li)
	p.Annotations = util.LateInitMapIfNonEmpty(p.Annotations, observed.Metadata.Annotations, li)
	p.VPCID = util.LateInitPtrIfNonZero(p.VPCID, observed.Spec.HostNetwork.VpcId, li)
	p.SubnetID = util.LateInitPtrIfNonZero(p.SubnetID, observed.Spec.HostNetwork.SubnetId, li)
	p.SecurityGroupID = util.LateInitPtrIfNonZero(
		p.SecurityGroupID,
		observed.Spec.HostNetwork.SecurityGroupId,
		li,
	)
	p.CustomSAN = util.LateInitSliceIfNonEmpty(p.CustomSAN, observed.Spec.CustomSan, li)
	p.HighwaySubnetID = util.LateInitPtrIfNonZero(
		p.HighwaySubnetID,
		observed.Spec.HostNetwork.HighwaySubnet,
		li,
	)
	p.BillingMode = util.LateInitPtr(p.BillingMode, observed.Spec.BillingMode, li)

	if p.EnableVolumeEncryption == nil && observed.Spec.EnableMasterVolumeEncryption != nil {
		v := *observed.Spec.EnableMasterVolumeEncryption
		p.EnableVolumeEncryption = &v
		li.SetChanged()
	}

	if observed.Spec.EniNetwork != nil {
		p.EniSubnetID = util.LateInitPtrIfNonZero(
			p.EniSubnetID,
			observed.Spec.EniNetwork.SubnetId,
			li,
		)
		p.EniSubnetCIDR = util.LateInitPtrIfNonZero(
			p.EniSubnetCIDR,
			observed.Spec.EniNetwork.Cidr,
			li,
		)
	}

	if len(p.Masters) == 0 && len(observed.Spec.Masters) > 0 {
		p.Masters = make([]ccev1alpha1.MasterSpec, len(observed.Spec.Masters))
		for i, m := range observed.Spec.Masters {
			p.Masters[i] = ccev1alpha1.MasterSpec{AvailabilityZone: m.AvailabilityZone}
		}
		li.SetChanged()
	}
}

func isClusterUpToDate(
	spec ccev1alpha1.ClusterParameters,
	observed *clusters.Clusters,
	lastAppliedConfigsHash string,
) bool {
	return util.IsOptionalUpToDate(spec.Description, observed.Spec.Description) &&
		isEIPUpToDate(spec, observed) &&
		componentConfigsHash(spec.ComponentConfigurations) == lastAppliedConfigsHash
}

func isEIPUpToDate(spec ccev1alpha1.ClusterParameters, observed *clusters.Clusters) bool {
	boundIP := observedBoundIP(observed)
	wantsEIP := spec.EIP != nil && *spec.EIP != ""
	if !wantsEIP {
		return boundIP == ""
	}
	return boundIP == *spec.EIP
}

// observedBoundIP extracts the public IP currently bound to the cluster master
// node from the external endpoint URL (e.g. "https://1.2.3.4:5443" → "1.2.3.4").
// Returns "" when no endpoint is published or the URL fails to parse.
func observedBoundIP(observed *clusters.Clusters) string {
	if len(observed.Status.Endpoints) == 0 {
		return ""
	}
	return parseEndpointHost(observed.Status.Endpoints[0].External)
}

func parseEndpointHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// componentConfigsHash returns a stable SHA-256 hash of the component
// configurations. Used to detect drift against the last-applied annotation
// since the CCE GET API does not return configurationsOverride.
// Returns "" for an empty/nil slice so the absent-on-both-sides case matches.
func componentConfigsHash(cfgs []ccev1alpha1.ComponentConfiguration) string {
	if len(cfgs) == 0 {
		return ""
	}
	b, err := json.Marshal(cfgs)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func buildCreateOpts(spec ccev1alpha1.ClusterParameters) clusters.CreateOpts {
	opts := clusters.CreateOpts{
		Kind:       "Cluster",
		ApiVersion: "v3",
		Metadata: clusters.CreateMetaData{
			Name:        spec.Name,
			Labels:      spec.Labels,
			Annotations: spec.Annotations,
			Timezone:    pointer.Deref(spec.Timezone, ""),
		},
		Spec: clusters.Spec{
			Type:   spec.ClusterType,
			Flavor: spec.FlavorID,
			HostNetwork: clusters.HostNetworkSpec{
				VpcId:    pointer.Deref(spec.VPCID, ""),
				SubnetId: pointer.Deref(spec.SubnetID, ""),
			},
			ContainerNetwork: clusters.ContainerNetworkSpec{
				Mode: spec.ContainerNetworkType,
			},
		},
	}

	applyOptionalCreateFields(&opts.Spec, spec)

	return opts
}

func applyOptionalCreateFields(s *clusters.Spec, spec ccev1alpha1.ClusterParameters) {
	s.Version = pointer.Deref(spec.ClusterVersion, "")
	s.Description = pointer.Deref(spec.Description, "")
	s.KubernetesSvcIpRange = pointer.Deref(spec.KubernetesSvcIPRange, "")
	s.KubeProxyMode = pointer.Deref(spec.KubeProxyMode, "")
	s.BillingMode = pointer.Deref(spec.BillingMode, 0)
	s.EnableMasterVolumeEncryption = spec.EnableVolumeEncryption
	s.DeletionProtection = spec.EnableDeletionProtection

	if spec.IPv6Enable != nil && *spec.IPv6Enable {
		s.Ipv6Enable = true
	}

	applyNetworkFields(s, spec)
	applyAuthentication(s, spec)
	applyMasters(s, spec)

	if len(spec.CustomSAN) > 0 {
		s.CustomSan = spec.CustomSAN
	}
	if len(spec.APIAccessTrustlist) > 0 {
		s.PublicAccess = &clusters.PublicAccess{Cidrs: spec.APIAccessTrustlist}
	}
	if len(spec.ComponentConfigurations) > 0 {
		s.ConfigurationsOverride = convertComponentConfigurations(spec.ComponentConfigurations)
	}

	applyExtendParam(s, spec)
}

func applyNetworkFields(s *clusters.Spec, spec ccev1alpha1.ClusterParameters) {
	if spec.SecurityGroupID != nil {
		s.HostNetwork.SecurityGroupId = *spec.SecurityGroupID
	}
	if spec.HighwaySubnetID != nil {
		s.HostNetwork.HighwaySubnet = *spec.HighwaySubnetID
	}
	if spec.ContainerNetworkCIDR != nil {
		s.ContainerNetwork.Cidr = *spec.ContainerNetworkCIDR
	}
	if spec.EniSubnetID != nil && spec.EniSubnetCIDR != nil {
		s.EniNetwork = &clusters.EniNetworkSpec{
			SubnetId: *spec.EniSubnetID,
			Cidr:     *spec.EniSubnetCIDR,
		}
	}
}

func applyAuthentication(s *clusters.Spec, spec ccev1alpha1.ClusterParameters) {
	if spec.AuthenticationMode == nil {
		return
	}
	s.Authentication = clusters.AuthenticationSpec{Mode: *spec.AuthenticationMode}
	if spec.AuthenticatingProxy != nil {
		s.Authentication.AuthenticatingProxy = map[string]string{
			"ca":         util.Base64IfNot(spec.AuthenticatingProxy.CA),
			"cert":       util.Base64IfNot(spec.AuthenticatingProxy.Cert),
			"privateKey": util.Base64IfNot(spec.AuthenticatingProxy.PrivateKey),
		}
	}
}

func applyMasters(s *clusters.Spec, spec ccev1alpha1.ClusterParameters) {
	if len(spec.Masters) == 0 {
		return
	}
	s.Masters = make([]clusters.MasterSpec, len(spec.Masters))
	for i, m := range spec.Masters {
		s.Masters[i] = clusters.MasterSpec{AvailabilityZone: m.AvailabilityZone}
	}
}

func applyExtendParam(s *clusters.Spec, spec ccev1alpha1.ClusterParameters) {
	extendParam := make(map[string]string)
	if spec.ExtendParam != nil {
		maps.Copy(extendParam, spec.ExtendParam)
	}
	if spec.NoAddons != nil && *spec.NoAddons {
		// NOTE(important): The alpha flag tells CCE to skip default addon
		// installation entirely. Risk: it's an alpha-tier extendParam, if OTC
		// removes/renames it we'd silently install addons.
		extendParam["alpha.installDefaultAddons"] = "false"
	}
	if spec.MultiAZ != nil && *spec.MultiAZ {
		extendParam["clusterAZ"] = "multi_az"
	}
	if spec.EIP != nil && *spec.EIP != "" {
		extendParam["clusterExternalIP"] = *spec.EIP
	}
	if len(extendParam) > 0 {
		s.ExtendParam = extendParam
	}
}

func convertComponentConfigurations(
	configs []ccev1alpha1.ComponentConfiguration,
) []clusters.PackageConfiguration {
	out := make([]clusters.PackageConfiguration, len(configs))
	for i, pkg := range configs {
		out[i] = clusters.PackageConfiguration{
			Name:           pkg.Name,
			Configurations: make([]clusters.Configuration, len(pkg.Configurations)),
		}
		for j, cfg := range pkg.Configurations {
			out[i].Configurations[j] = clusters.Configuration{
				Name:  cfg.Name,
				Value: util.ParseAnyType(cfg.Value),
			}
		}
	}
	return out
}

func (e *external) updateDescription(clusterID string, spec ccev1alpha1.ClusterParameters) error {
	if spec.Description == nil {
		return nil
	}
	_, err := clusters.Update(e.cceClient, clusterID, clusters.UpdateOpts{
		Spec: clusters.UpdateSpec{
			Description: *spec.Description,
		},
	})
	if err != nil {
		return errors.Wrap(err, errUpdateCluster)
	}
	return nil
}

func (e *external) updateEIP(
	clusterID string,
	spec ccev1alpha1.ClusterParameters,
	observed ccev1alpha1.ClusterObservation,
) error {
	wantsEIP := spec.EIP != nil && *spec.EIP != ""
	boundIP := parseEndpointHost(observed.ExternalEndpoint)

	switch {
	case wantsEIP && boundIP == "":
		return e.bindEIP(clusterID, spec)
	case !wantsEIP && boundIP != "":
		return e.unbindEIP(clusterID)
	case wantsEIP && boundIP != *spec.EIP:
		// Swap: We unbind the old IP first then bind the new one.
		if err := e.unbindEIP(clusterID); err != nil {
			return err
		}
		return e.bindEIP(clusterID, spec)
	}
	return nil
}

func (e *external) bindEIP(clusterID string, spec ccev1alpha1.ClusterParameters) error {
	if spec.EIPID == nil || *spec.EIPID == "" {
		return errors.New(errEIPIDRequired)
	}
	err := clusters.UpdateMasterIp(e.cceClient, clusterID, clusters.UpdateIpOpts{
		Action:    "bind",
		Spec:      clusters.IpSpec{ID: *spec.EIPID},
		ElasticIp: *spec.EIP,
	})
	return errors.Wrap(err, errUpdateMasterIP)
}

func (e *external) unbindEIP(clusterID string) error {
	err := clusters.UpdateMasterIp(e.cceClient, clusterID, clusters.UpdateIpOpts{
		Action: "unbind",
	})
	return errors.Wrap(err, errUpdateMasterIP)
}

func (e *external) updateComponentConfigurations(
	clusterID string,
	spec ccev1alpha1.ClusterParameters,
) error {
	// Always send the request, including with an empty Packages list, so that
	// removing all overrides from spec actually clears them on the cluster.
	// Returning early on len==0 would leave stale config in place while the
	// annotation hash claims everything is reconciled.
	_, err := nodepools.UpdateConfiguration(
		e.cceClient,
		clusterID,
		"master",
		nodepools.UpdateConfigurationOpts{
			Kind:       "Configuration",
			APIVersion: "v3",
			Metadata: nodepools.ConfigurationMetadata{
				Name: "configuration",
			},
			Spec: nodepools.ClusterConfigurationsSpec{
				Packages: convertComponentConfigurations(spec.ComponentConfigurations),
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, errUpdateConfig)
	}
	return nil
}

func buildDeleteQueryParams(spec ccev1alpha1.ClusterParameters) clusters.DeleteQueryParams {
	params := clusters.DeleteQueryParams{
		DeleteEfs: pointer.Deref(spec.DeleteEFS, ""),
		DeleteENI: pointer.Deref(spec.DeleteENI, ""),
		DeleteEvs: pointer.Deref(spec.DeleteEVS, ""),
		DeleteNet: pointer.Deref(spec.DeleteNet, ""),
		DeleteObs: pointer.Deref(spec.DeleteOBS, ""),
		DeleteSfs: pointer.Deref(spec.DeleteSFS, ""),
	}

	// Any non-"false" value is honored ("true" forces the delete, "try"
	// best-efforts it).
	if v := pointer.Deref(spec.DeleteAllStorage, ""); v != "" && v != "false" {
		params.DeleteEfs = v
		params.DeleteEvs = v
		params.DeleteObs = v
		params.DeleteSfs = v
	}
	if v := pointer.Deref(spec.DeleteAllNetwork, ""); v != "" && v != "false" {
		params.DeleteNet = v
		params.DeleteENI = v
	}

	return params
}
