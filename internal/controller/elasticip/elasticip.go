package elasticip

import (
	"context"
	"maps"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/bandwidths"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/eips"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

// API-to-SDK enum mappings
var (
	ipTypeToSDK    = map[string]string{"BGP": "5_bgp"}
	shareTypeToSDK = map[string]string{"Dedicated": "PER"}
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetClient         = "cannot get OTC provider client"
	errCreateV1Client    = "cannot create Network v1 client"
	errCreateV2Client    = "cannot create Network v2 client"
	errValidateSpec      = "invalid ElasticIP spec"
	errEmptyExternalName = "external name is empty"
	errObserveEIP        = "cannot observe ElasticIP"
	errObserveBandwidth  = "cannot observe ElasticIP bandwidth"
	errObserveTags       = "cannot observe ElasticIP tags"
	errCreateEIP         = "cannot create ElasticIP"
	errUpdateEIP         = "cannot update ElasticIP"
	errDeleteEIP         = "cannot delete ElasticIP"
)

// SetupGated adds a controller that reconciles ElasticIP managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup ElasticIP controller"))
		}
	}, networkv1alpha1.ElasticIPGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles ElasticIP managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1alpha1.ElasticIPGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*networkv1alpha1.ElasticIP](&connector{
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
			&networkv1alpha1.ElasticIPList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind networkv1alpha1.ElasticIPList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(networkv1alpha1.ElasticIPGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1alpha1.ElasticIP{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*networkv1alpha1.ElasticIP] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *networkv1alpha1.ElasticIP,
) (managed.TypedExternalClient[*networkv1alpha1.ElasticIP], error) {
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

var _ managed.TypedExternalClient[*networkv1alpha1.ElasticIP] = (*external)(nil)

type external struct {
	networkV1Client *golangsdk.ServiceClient
	networkV2Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *networkv1alpha1.ElasticIP,
) (managed.ExternalObservation, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := eips.Get(e.networkV1Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveEIP)
	}

	observedBW, err := bandwidths.Get(e.networkV1Client, observed.BandwidthID).Extract()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveBandwidth)
	}

	observedTags, err := e.observeTags(externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveTags)
	}

	// set observation
	cr.Status.AtProvider = networkv1alpha1.ElasticIPObservation{
		ID:                  observed.ID,
		Status:              observed.Status,
		PublicIPAddress:     observed.PublicAddress,
		PrivateIPAddress:    observed.PrivateAddress,
		PortID:              observed.PortID,
		BandwidthID:         observed.BandwidthID,
		BandwidthName:       observedBW.Name,
		BandwidthSize:       observedBW.Size,
		BandwidthShareType:  observedBW.ShareType,
		BandwidthChargeMode: observedBW.ChargeMode,
		IPVersion:           observed.IpVersion,
		Tags:                maps.Clone(observedTags),
	}

	// set conditions
	e.setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializeElasticIP(cr, observed, &observedBW, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isElasticIPUpToDate(
			cr.Spec.ForProvider,
			observed,
			&observedBW,
			observedTags,
		),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *networkv1alpha1.ElasticIP,
) (managed.ExternalCreation, error) {
	createOpts := buildCreateOpts(cr.Spec.ForProvider)

	created, err := eips.Apply(e.networkV1Client, createOpts).Extract()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateEIP)
	}
	meta.SetExternalName(cr, created.ID)

	// Bind port
	if portID := pointer.Deref(cr.Spec.ForProvider.PublicIP.PortID, ""); portID != "" {
		if err := e.bindPort(created.ID, portID); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errUpdateEIP)
		}
	}

	err = e.reconcileTags(
		created.ID,
		map[string]string{},
		cr.Spec.ForProvider.Tags,
	)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateEIP)
	}

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *networkv1alpha1.ElasticIP,
) (managed.ExternalUpdate, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	observed, observedBW, observedTags, err := e.observeCurrentState(externalName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := validateImmutableFields(cr.Spec.ForProvider, observed, observedBW); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	err = e.updateBandwidth(
		observed.BandwidthID,
		cr.Spec.ForProvider.Bandwidth,
		observedBW,
	)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateEIP)
	}

	err = e.updatePortBinding(
		externalName,
		cr.Spec.ForProvider.PublicIP.PortID,
		observed.PortID,
	)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateEIP)
	}

	if err := e.reconcileTags(externalName, observedTags, cr.Spec.ForProvider.Tags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateEIP)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *networkv1alpha1.ElasticIP,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	observed, err := eips.Get(e.networkV1Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteEIP)
	}

	// Unbind port
	if observed.PortID != "" {
		_, err := eips.Update(e.networkV1Client, externalName, eips.UpdateOpts{}).Extract()
		if err != nil {
			return managed.ExternalDelete{}, errors.Wrap(err, errDeleteEIP)
		}
	}

	if err := eips.Delete(e.networkV1Client, externalName).ExtractErr(); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteEIP)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildCreateOpts(spec networkv1alpha1.ElasticIPParameters) eips.ApplyOpts {
	ipOpts := eips.PublicIpOpts{
		Type: ipTypeToSDK[spec.PublicIP.Type],
	}
	if spec.PublicIP.IPAddress != nil {
		ipOpts.Address = *spec.PublicIP.IPAddress
	}
	if spec.PublicIP.Name != nil {
		ipOpts.Name = *spec.PublicIP.Name
	}

	bwOpts := eips.BandwidthOpts{
		Name:      spec.Bandwidth.Name,
		Size:      spec.Bandwidth.Size,
		ShareType: shareTypeToSDK[spec.Bandwidth.ShareType],
	}
	if spec.Bandwidth.ChargeMode != nil {
		bwOpts.ChargeMode = *spec.Bandwidth.ChargeMode
	}

	return eips.ApplyOpts{
		IP:        ipOpts,
		Bandwidth: bwOpts,
	}
}

func (e *external) bindPort(eipID, portID string) error {
	updateOps := eips.UpdateOpts{PortID: portID}
	_, err := eips.Update(e.networkV1Client, eipID, updateOps).Extract()
	return err
}

func (e *external) updateBandwidth(
	bandwidthID string,
	spec networkv1alpha1.ElasticIPBandwidthParameters,
	observed *bandwidths.BandWidth,
) error {
	opts, needsUpdate := buildBandwidthUpdateOpts(spec, observed)
	if !needsUpdate {
		return nil
	}

	_, err := bandwidths.Update(e.networkV1Client, bandwidthID, opts).Extract()
	return err
}

func buildBandwidthUpdateOpts(
	spec networkv1alpha1.ElasticIPBandwidthParameters,
	observed *bandwidths.BandWidth,
) (bandwidths.UpdateOpts, bool) {
	opts := bandwidths.UpdateOpts{}
	needsUpdate := false

	if spec.Name != observed.Name {
		opts.Name = spec.Name
		needsUpdate = true
	}
	if spec.Size != observed.Size {
		opts.Size = spec.Size
		needsUpdate = true
	}

	return opts, needsUpdate
}

func (e *external) updatePortBinding(
	eipID string,
	desiredPortID *string,
	observedPortID string,
) error {
	desired := pointer.Deref(desiredPortID, "")
	if desired == observedPortID {
		return nil
	}

	updateOpts := eips.UpdateOpts{PortID: desired}
	_, err := eips.Update(e.networkV1Client, eipID, updateOpts).Extract()
	return err
}

func (e *external) observeCurrentState(
	id string,
) (*eips.PublicIp, *bandwidths.BandWidth, map[string]string, error) {
	observed, err := eips.Get(e.networkV1Client, id).Extract()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, errObserveEIP)
	}

	observedBW, err := bandwidths.Get(e.networkV1Client, observed.BandwidthID).Extract()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, errObserveBandwidth)
	}

	observedTags, err := e.observeTags(id)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, errObserveTags)
	}

	return observed, &observedBW, observedTags, nil
}

func (e *external) observeTags(id string) (map[string]string, error) {
	list, err := tags.Get(e.networkV2Client, "publicips", id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func (e *external) setConditions(cr *networkv1alpha1.ElasticIP, observedStatus string) {
	switch observedStatus {
	case "ACTIVE", "DOWN", "ELB", "VPN":
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
		err := tags.Create(e.networkV2Client, "publicips", id, util.MapToResourceTags(toCreate)).
			ExtractErr()
		if err != nil {
			return err
		}
	}

	toDelete := util.TagDiff(current, desired)
	if len(toDelete) > 0 {
		err := tags.Delete(e.networkV2Client, "publicips", id, util.MapToResourceTags(toDelete)).
			ExtractErr()
		if err != nil {
			return err
		}
	}

	return nil
}

func validateImmutableFields(
	spec networkv1alpha1.ElasticIPParameters,
	observed *eips.PublicIp,
	observedBW *bandwidths.BandWidth,
) error {
	if ipTypeToSDK[spec.PublicIP.Type] != observed.Type {
		return errors.New("publicIp.type is immutable after creation")
	}
	if spec.PublicIP.IPAddress != nil && *spec.PublicIP.IPAddress != observed.PublicAddress {
		return errors.New("publicIp.ipAddress is immutable after creation")
	}
	if spec.PublicIP.Name != nil && *spec.PublicIP.Name != observed.Name {
		return errors.New("publicIp.name is immutable after creation")
	}
	if shareTypeToSDK[spec.Bandwidth.ShareType] != observedBW.ShareType {
		return errors.New("bandwidth.shareType is immutable after creation")
	}
	if spec.Bandwidth.ChargeMode != nil && *spec.Bandwidth.ChargeMode != observedBW.ChargeMode {
		return errors.New("bandwidth.chargeMode is immutable after creation")
	}
	return nil
}

func lateInitializeElasticIP(
	cr *networkv1alpha1.ElasticIP,
	observed *eips.PublicIp,
	observedBW *bandwidths.BandWidth,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.PublicIP.IPAddress = util.LateInitPtrIfNonZero(
		p.PublicIP.IPAddress,
		observed.PublicAddress,
		li,
	)
	p.PublicIP.Name = util.LateInitPtrIfNonZero(p.PublicIP.Name, observed.Name, li)
	p.PublicIP.PortID = util.LateInitPtrIfNonZero(p.PublicIP.PortID, observed.PortID, li)
	p.Bandwidth.ChargeMode = util.LateInitPtrIfNonZero(
		p.Bandwidth.ChargeMode,
		observedBW.ChargeMode,
		li,
	)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isElasticIPUpToDate(
	spec networkv1alpha1.ElasticIPParameters,
	observed *eips.PublicIp,
	observedBW *bandwidths.BandWidth,
	observedTags map[string]string,
) bool {
	return ipTypeToSDK[spec.PublicIP.Type] == observed.Type &&
		util.IsOptionalUpToDate(spec.PublicIP.PortID, observed.PortID) &&
		spec.Bandwidth.Name == observedBW.Name &&
		spec.Bandwidth.Size == observedBW.Size &&
		shareTypeToSDK[spec.Bandwidth.ShareType] == observedBW.ShareType &&
		util.IsOptionalUpToDate(spec.Bandwidth.ChargeMode, observedBW.ChargeMode) &&
		util.IsOptionalMapUpToDate(spec.Tags, observedTags)
}
