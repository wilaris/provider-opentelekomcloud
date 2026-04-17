package subnet

import (
	"context"
	"maps"
	"net"
	"slices"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/common/tags"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/subnets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetClient         = "cannot get OTC provider client"
	errCreateV1Client    = "cannot create Network v1 client"
	errCreateV2Client    = "cannot create Network v2 client"
	errValidateSpec      = "invalid Subnet spec"
	errEmptyExternalName = "external name is empty"
	errObserveSubnet     = "cannot observe Subnet"
	errObserveTags       = "cannot observe Subnet tags"
	errResolveVPCID      = "cannot resolve vpcId for Subnet"
	errCreateSubnet      = "cannot create Subnet"
	errUpdateSubnet      = "cannot update Subnet"
	errDeleteSubnet      = "cannot delete Subnet"
	errEmptyVPCID        = "resolved vpcId is empty"
)

// SetupGated adds a controller that reconciles Subnet managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Subnet controller"))
		}
	}, networkv1alpha1.SubnetGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles Subnet managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1alpha1.SubnetGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*networkv1alpha1.Subnet](&connector{
			kube: mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(
				mgr.GetClient(),
				&apisv1alpha1.ProviderConfigUsage{},
			),
			clientCache: clients.NewCache(mgr.GetClient()),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		//nolint:staticcheck // controller-runtime recorder type mismatch with event.NewAPIRecorder.
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithCreationGracePeriod(30 * time.Second),
		managed.WithPollJitterHook(30 * time.Second),
		managed.WithTimeout(5 * time.Minute),
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
			&networkv1alpha1.SubnetList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind networkv1alpha1.SubnetList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(networkv1alpha1.SubnetGroupVersionKind),
		opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1alpha1.Subnet{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*networkv1alpha1.Subnet] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *networkv1alpha1.Subnet,
) (managed.TypedExternalClient[*networkv1alpha1.Subnet], error) {
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
	v1Client, err := openstack.NewNetworkV1(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateV1Client)
	}
	v2Client, err := openstack.NewNetworkV2(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateV2Client)
	}

	return &external{
		networkV1Client: v1Client,
		networkV2Client: v2Client,
	}, nil
}

var _ managed.TypedExternalClient[*networkv1alpha1.Subnet] = (*external)(nil)

type external struct {
	networkV1Client *golangsdk.ServiceClient
	networkV2Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *networkv1alpha1.Subnet,
) (managed.ExternalObservation, error) {
	if err := validateSubnetParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := subnets.Get(e.networkV1Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveSubnet)
	}

	observedNTP := extractNTPAddress(observed.ExtraDHCPOpts)

	observedTags, err := e.observeTags(externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveTags)
	}

	// set observation
	cr.Status.AtProvider = networkv1alpha1.SubnetObservation{
		ID:               observed.ID,
		Name:             observed.Name,
		Description:      observed.Description,
		CIDR:             observed.CIDR,
		DNSList:          slices.Clone(observed.DNSList),
		GatewayIP:        observed.GatewayIP,
		DHCPEnable:       observed.EnableDHCP,
		IPv6Enable:       observed.EnableIpv6,
		CIDRIPv6:         observed.CidrV6,
		GatewayIPv6:      observed.GatewayIpV6,
		PrimaryDNS:       observed.PrimaryDNS,
		SecondaryDNS:     observed.SecondaryDNS,
		AvailabilityZone: observed.AvailabilityZone,
		VPCID:            observed.VpcID,
		SubnetID:         observed.SubnetID,
		NetworkID:        observed.NetworkID,
		NTPAddresses:     observedNTP,
		Status:           observed.Status,
		Tags:             maps.Clone(observedTags),
	}

	// set conditions
	e.setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializeSubnet(cr, observed, observedNTP, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isSubnetUpToDate(
			cr.Spec.ForProvider,
			observed,
			observedNTP,
			observedTags,
		),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *networkv1alpha1.Subnet,
) (managed.ExternalCreation, error) {
	if err := validateSubnetParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts, err := buildSubnetCreateOpts(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	created, err := subnets.Create(e.networkV1Client, createOpts).Extract()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSubnet)
	}
	meta.SetExternalName(cr, created.ID)

	if err := e.reconcileTags(
		created.ID,
		map[string]string{},
		cr.Spec.ForProvider.Tags,
	); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateSubnet)
	}

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *networkv1alpha1.Subnet,
) (managed.ExternalUpdate, error) {
	if err := validateSubnetParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	observed, observedNTP, observedTags, err := e.observeCurrentState(externalName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := validateImmutableSubnetFields(cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	if err := e.update(externalName, cr.Spec.ForProvider, observed, observedNTP); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateSubnet)
	}

	if err := e.reconcileTags(externalName, observedTags, cr.Spec.ForProvider.Tags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateSubnet)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *networkv1alpha1.Subnet,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	vpcID, err := e.resolveVPCIDForDelete(cr, externalName)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errResolveVPCID)
	}
	if vpcID == "" {
		return managed.ExternalDelete{}, nil
	}

	if err := subnets.Delete(e.networkV1Client, vpcID, externalName).ExtractErr(); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteSubnet)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildSubnetCreateOpts(spec networkv1alpha1.SubnetParameters) (subnets.CreateOpts, error) {
	vpcID := pointer.Deref(spec.VPCID, "")
	if vpcID == "" {
		return subnets.CreateOpts{}, errors.New(errEmptyVPCID)
	}

	enableDHCP := pointer.Deref(spec.DHCPEnable, true)
	enableIPv6 := pointer.Deref(spec.IPv6Enable, false)
	primaryDNS, secondaryDNS, dnsList := desiredDNSValues(spec)

	createOpts := subnets.CreateOpts{
		Name:         spec.Name,
		CIDR:         spec.CIDR,
		DNSList:      dnsList,
		EnableIpv6:   &enableIPv6,
		GatewayIP:    spec.GatewayIP,
		EnableDHCP:   &enableDHCP,
		PrimaryDNS:   primaryDNS,
		SecondaryDNS: secondaryDNS,
		VpcID:        vpcID,
	}
	if spec.Description != nil {
		createOpts.Description = *spec.Description
	}
	if spec.AvailabilityZone != nil {
		createOpts.AvailabilityZone = *spec.AvailabilityZone
	}
	if spec.NTPAddresses != nil && *spec.NTPAddresses != "" {
		createOpts.ExtraDHCPOpts = []subnets.ExtraDHCPOpt{{
			OptName:  "ntp",
			OptValue: *spec.NTPAddresses,
		}}
	}

	return createOpts, nil
}

func desiredDNSValues(spec networkv1alpha1.SubnetParameters) (string, string, []string) {
	primaryDNS := pointer.Deref(spec.PrimaryDNS, "")
	secondaryDNS := pointer.Deref(spec.SecondaryDNS, "")
	dnsList := slices.Clone(spec.DNSList)

	return primaryDNS, secondaryDNS, dnsList
}

func (e *external) update(
	id string,
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
	observedNTP string,
) error {
	opts, needsUpdate := buildSubnetUpdateOpts(spec, observed, observedNTP)
	if !needsUpdate {
		return nil
	}

	_, err := subnets.Update(e.networkV1Client, observed.VpcID, id, opts).Extract()
	return err
}

func buildSubnetUpdateOpts(
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
	observedNTP string,
) (subnets.UpdateOpts, bool) {
	opts := subnets.UpdateOpts{Name: spec.Name}
	needsUpdate := spec.Name != observed.Name
	if applyDescriptionIfChanged(&opts, spec.Description, observed.Description) {
		needsUpdate = true
	}
	if applyPrimaryDNSIfChanged(&opts, spec.PrimaryDNS, observed.PrimaryDNS) {
		needsUpdate = true
	}
	if applySecondaryDNSIfChanged(&opts, spec.SecondaryDNS, observed.SecondaryDNS) {
		needsUpdate = true
	}
	if applyDNSListIfChanged(&opts, spec.DNSList, observed.DNSList) {
		needsUpdate = true
	}
	if applyDHCPIfChanged(&opts, spec.DHCPEnable, observed.EnableDHCP) {
		needsUpdate = true
	}
	if applyNTPIfChanged(&opts, spec.NTPAddresses, observedNTP) {
		needsUpdate = true
	}

	return opts, needsUpdate
}

func applyDescriptionIfChanged(opts *subnets.UpdateOpts, desired *string, observed string) bool {
	if desired == nil || *desired == observed {
		return false
	}
	description := *desired
	opts.Description = &description
	return true
}

func applyPrimaryDNSIfChanged(opts *subnets.UpdateOpts, desired *string, observed string) bool {
	if desired == nil || *desired == observed {
		return false
	}
	opts.PrimaryDNS = *desired
	return true
}

func applySecondaryDNSIfChanged(opts *subnets.UpdateOpts, desired *string, observed string) bool {
	if desired == nil || *desired == observed {
		return false
	}
	opts.SecondaryDNS = *desired
	return true
}

func applyDNSListIfChanged(opts *subnets.UpdateOpts, desired []string, observed []string) bool {
	if desired == nil || slices.Equal(desired, observed) {
		return false
	}
	opts.DNSList = slices.Clone(desired)
	return true
}

func applyDHCPIfChanged(opts *subnets.UpdateOpts, desired *bool, observed bool) bool {
	if desired == nil || *desired == observed {
		return false
	}
	enableDHCP := *desired
	opts.EnableDHCP = &enableDHCP
	return true
}

func applyNTPIfChanged(opts *subnets.UpdateOpts, desired *string, observed string) bool {
	if desired == nil || *desired == observed {
		return false
	}
	opts.ExtraDhcpOpts = []subnets.ExtraDHCPOpt{{
		OptName:  "ntp",
		OptValue: *desired,
	}}
	return true
}

func (e *external) observeCurrentState(
	id string,
) (*subnets.Subnet, string, map[string]string, error) {
	observed, err := subnets.Get(e.networkV1Client, id).Extract()
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errObserveSubnet)
	}

	observedTags, err := e.observeTags(id)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errObserveTags)
	}

	return observed, extractNTPAddress(observed.ExtraDHCPOpts), observedTags, nil
}

func (e *external) observeTags(id string) (map[string]string, error) {
	list, err := tags.Get(e.networkV2Client, "subnets", id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func (e *external) setConditions(cr *networkv1alpha1.Subnet, observedStatus string) {
	switch observedStatus {
	case "ACTIVE":
		cr.Status.SetConditions(xpv1.Available())
	default:
		cr.Status.SetConditions(xpv1.Unavailable())
	}
}

func (e *external) reconcileTags(
	id string,
	current map[string]string,
	desired map[string]string,
) error {
	if desired == nil {
		return nil
	}

	toCreate := util.TagDiff(desired, current)
	if len(toCreate) > 0 {
		if err := tags.Create(e.networkV2Client, "subnets", id, util.MapToResourceTags(toCreate)).
			ExtractErr(); err != nil {
			return err
		}
	}

	toDelete := util.TagDiff(current, desired)
	if len(toDelete) > 0 {
		if err := tags.Delete(e.networkV2Client, "subnets", id, util.MapToResourceTags(toDelete)).
			ExtractErr(); err != nil {
			return err
		}
	}

	return nil
}

func (e *external) resolveVPCIDForDelete(
	cr *networkv1alpha1.Subnet,
	externalName string,
) (string, error) {
	if vpcID := pointer.Deref(cr.Spec.ForProvider.VPCID, ""); vpcID != "" {
		return vpcID, nil
	}
	if cr.Status.AtProvider.VPCID != "" {
		return cr.Status.AtProvider.VPCID, nil
	}

	observed, err := subnets.Get(e.networkV1Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if observed.VpcID == "" {
		return "", errors.New(errEmptyVPCID)
	}
	return observed.VpcID, nil
}

func validateSubnetParameters(p networkv1alpha1.SubnetParameters) error {
	if err := validateRequiredSubnetFields(p); err != nil {
		return err
	}
	if err := validateOptionalDNSServers(p); err != nil {
		return err
	}
	return validateDNSList(p.DNSList)
}

func validateRequiredSubnetFields(p networkv1alpha1.SubnetParameters) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.CIDR == "" {
		return errors.New("cidr is required")
	}
	if _, _, err := net.ParseCIDR(p.CIDR); err != nil {
		return errors.Wrap(err, "cidr must be a valid CIDR")
	}
	if p.GatewayIP == "" {
		return errors.New("gatewayIp is required")
	}
	if !isValidIPv4(p.GatewayIP) {
		return errors.New("gatewayIp must be a valid IPv4 address")
	}
	if p.VPCID == nil || *p.VPCID == "" {
		return errors.New("vpcId is required")
	}
	return nil
}

func validateOptionalDNSServers(p networkv1alpha1.SubnetParameters) error {
	if p.PrimaryDNS != nil && *p.PrimaryDNS != "" && !isValidIPv4(*p.PrimaryDNS) {
		return errors.New("primaryDns must be a valid IPv4 address")
	}
	if p.SecondaryDNS != nil && *p.SecondaryDNS != "" && !isValidIPv4(*p.SecondaryDNS) {
		return errors.New("secondaryDns must be a valid IPv4 address")
	}
	return nil
}

func validateDNSList(dnsList []string) error {
	for _, dns := range dnsList {
		if !isValidIPv4(dns) {
			return errors.Errorf("dnsList contains an invalid IPv4 address: %s", dns)
		}
	}
	return nil
}

func validateImmutableSubnetFields(
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
) error {
	if spec.CIDR != observed.CIDR {
		return errors.New("cidr is immutable after creation")
	}
	if spec.GatewayIP != observed.GatewayIP {
		return errors.New("gatewayIp is immutable after creation")
	}
	if spec.IPv6Enable != nil && *spec.IPv6Enable != observed.EnableIpv6 {
		return errors.New("ipv6Enable is immutable after creation")
	}
	if spec.AvailabilityZone != nil && *spec.AvailabilityZone != observed.AvailabilityZone {
		return errors.New("availabilityZone is immutable after creation")
	}
	if spec.VPCID != nil && *spec.VPCID != observed.VpcID {
		return errors.New("vpcId is immutable after creation")
	}
	return nil
}

func isValidIPv4(v string) bool {
	ip := net.ParseIP(v)
	return ip != nil && ip.To4() != nil
}

func extractNTPAddress(opts []subnets.ExtraDHCP) string {
	for _, opt := range opts {
		if opt.OptName == "ntp" {
			return opt.OptValue
		}
	}
	return ""
}

func lateInitializeSubnet(
	cr *networkv1alpha1.Subnet,
	observed *subnets.Subnet,
	observedNTP string,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.DHCPEnable = util.LateInitPtr(p.DHCPEnable, observed.EnableDHCP, li)
	p.IPv6Enable = util.LateInitPtr(p.IPv6Enable, observed.EnableIpv6, li)
	p.PrimaryDNS = util.LateInitPtrIfNonZero(p.PrimaryDNS, observed.PrimaryDNS, li)
	p.SecondaryDNS = util.LateInitPtrIfNonZero(p.SecondaryDNS, observed.SecondaryDNS, li)
	p.DNSList = util.LateInitSliceIfNonEmpty(p.DNSList, observed.DNSList, li)
	p.AvailabilityZone = util.LateInitPtrIfNonZero(
		p.AvailabilityZone,
		observed.AvailabilityZone,
		li,
	)
	p.VPCID = util.LateInitPtrIfNonZero(p.VPCID, observed.VpcID, li)
	p.NTPAddresses = util.LateInitPtrIfNonZero(p.NTPAddresses, observedNTP, li)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isSubnetUpToDate(
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
	observedNTP string,
	observedTags map[string]string,
) bool {
	return subnetCoreFieldsUpToDate(spec, observed) &&
		subnetOptionalNetworkFieldsUpToDate(spec, observed) &&
		subnetOptionalDNSFieldsUpToDate(spec, observed, observedNTP) &&
		subnetOptionalMetaFieldsUpToDate(spec, observedTags)
}

func subnetCoreFieldsUpToDate(
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
) bool {
	return spec.Name == observed.Name &&
		spec.CIDR == observed.CIDR &&
		spec.GatewayIP == observed.GatewayIP
}

func subnetOptionalNetworkFieldsUpToDate(
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
) bool {
	return util.IsOptionalUpToDate(spec.DHCPEnable, observed.EnableDHCP) &&
		util.IsOptionalUpToDate(spec.IPv6Enable, observed.EnableIpv6) &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalUpToDate(spec.AvailabilityZone, observed.AvailabilityZone) &&
		util.IsOptionalUpToDate(spec.VPCID, observed.VpcID)
}

func subnetOptionalDNSFieldsUpToDate(
	spec networkv1alpha1.SubnetParameters,
	observed *subnets.Subnet,
	observedNTP string,
) bool {
	return util.IsOptionalUpToDate(spec.PrimaryDNS, observed.PrimaryDNS) &&
		util.IsOptionalUpToDate(spec.SecondaryDNS, observed.SecondaryDNS) &&
		util.IsOptionalSliceUpToDate(spec.DNSList, observed.DNSList) &&
		util.IsOptionalUpToDate(spec.NTPAddresses, observedNTP)
}

func subnetOptionalMetaFieldsUpToDate(
	spec networkv1alpha1.SubnetParameters,
	observedTags map[string]string,
) bool {
	return util.IsOptionalMapUpToDate(spec.Tags, observedTags)
}
