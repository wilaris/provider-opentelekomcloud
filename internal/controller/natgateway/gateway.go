package natgateway

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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/natgateways"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	natv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/nat/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetClient         = "cannot get OTC provider client"
	errCreateNatClient   = "cannot create NAT v2 client"
	errValidateSpec      = "invalid Gateway spec"
	errEmptyExternalName = "external name is empty"
	errObserveGateway    = "cannot observe Gateway"
	errObserveTags       = "cannot observe Gateway tags"
	errCreateGateway     = "cannot create Gateway"
	errUpdateGateway     = "cannot update Gateway"
	errDeleteGateway     = "cannot delete Gateway"
	errEmptyVPCID        = "resolved vpcId is empty"
	errEmptySubnetID     = "resolved subnetId is empty"
)

// SetupGated adds a controller that reconciles Gateway managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Gateway controller"))
		}
	}, natv1alpha1.GatewayGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles Gateway managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(natv1alpha1.GatewayGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*natv1alpha1.Gateway](&connector{
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
			&natv1alpha1.GatewayList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind natv1alpha1.GatewayList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(natv1alpha1.GatewayGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&natv1alpha1.Gateway{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*natv1alpha1.Gateway] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *natv1alpha1.Gateway,
) (managed.TypedExternalClient[*natv1alpha1.Gateway], error) {
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
	natV2Client, err := openstack.NewNatV2(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateNatClient)
	}

	return &external{
		natV2Client: natV2Client,
	}, nil
}

var _ managed.TypedExternalClient[*natv1alpha1.Gateway] = (*external)(nil)

type external struct {
	natV2Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *natv1alpha1.Gateway,
) (managed.ExternalObservation, error) {
	if err := validateGatewayParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := natgateways.Get(e.natV2Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveGateway)
	}

	observedTags, err := e.observeTags(externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveTags)
	}

	// set observation
	cr.Status.AtProvider = natv1alpha1.GatewayObservation{
		ID:           observed.ID,
		Name:         observed.Name,
		Description:  observed.Description,
		Size:         observed.Spec,
		VPCID:        observed.RouterID,
		SubnetID:     observed.InternalNetworkID,
		Status:       observed.Status,
		AdminStateUp: observed.AdminStateUp,
		Tags:         maps.Clone(observedTags),
	}

	// set conditions
	e.setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializeGateway(cr, observed, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isGatewayUpToDate(
			cr.Spec.ForProvider,
			observed,
			observedTags,
		),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *natv1alpha1.Gateway,
) (managed.ExternalCreation, error) {
	if meta.GetExternalName(cr) != "" {
		return managed.ExternalCreation{}, nil
	}

	if err := validateGatewayParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts := buildGatewayCreateOpts(cr.Spec.ForProvider)

	created, err := natgateways.Create(e.natV2Client, createOpts).Extract()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateGateway)
	}
	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *natv1alpha1.Gateway,
) (managed.ExternalUpdate, error) {
	if err := validateGatewayParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	observed, observedTags, err := e.observeCurrentState(externalName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := validateImmutableGatewayFields(cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	if err := e.update(externalName, cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateGateway)
	}

	if err := e.reconcileTags(externalName, observedTags, cr.Spec.ForProvider.Tags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateGateway)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *natv1alpha1.Gateway,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if err := natgateways.Delete(e.natV2Client, externalName).ExtractErr(); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteGateway)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildGatewayCreateOpts(spec natv1alpha1.GatewayParameters) natgateways.CreateOpts {
	createOpts := natgateways.CreateOpts{
		Name:              spec.Name,
		Spec:              spec.Size,
		RouterID:          pointer.Deref(spec.VPCID, ""),
		InternalNetworkID: pointer.Deref(spec.SubnetID, ""),
	}
	if spec.Description != nil {
		createOpts.Description = *spec.Description
	}
	return createOpts
}

func (e *external) update(
	id string,
	spec natv1alpha1.GatewayParameters,
	observed natgateways.NatGateway,
) error {
	opts, needsUpdate := buildGatewayUpdateOpts(spec, observed)
	if !needsUpdate {
		return nil
	}

	_, err := natgateways.Update(e.natV2Client, id, opts).Extract()
	return err
}

func buildGatewayUpdateOpts(
	spec natv1alpha1.GatewayParameters,
	observed natgateways.NatGateway,
) (natgateways.UpdateOpts, bool) {
	opts := natgateways.UpdateOpts{
		Name: spec.Name,
		Spec: spec.Size,
	}
	needsUpdate := spec.Name != observed.Name || spec.Size != observed.Spec

	if spec.Description != nil && *spec.Description != observed.Description {
		opts.Description = *spec.Description
		needsUpdate = true
	}

	return opts, needsUpdate
}

func (e *external) observeCurrentState(
	id string,
) (natgateways.NatGateway, map[string]string, error) {
	observed, err := natgateways.Get(e.natV2Client, id).Extract()
	if err != nil {
		return natgateways.NatGateway{}, nil, errors.Wrap(err, errObserveGateway)
	}

	observedTags, err := e.observeTags(id)
	if err != nil {
		return natgateways.NatGateway{}, nil, errors.Wrap(err, errObserveTags)
	}

	return observed, observedTags, nil
}

func (e *external) observeTags(id string) (map[string]string, error) {
	list, err := tags.Get(e.natV2Client, "nat_gateways", id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func (e *external) setConditions(cr *natv1alpha1.Gateway, observedStatus string) {
	switch observedStatus {
	case "ACTIVE":
		cr.Status.SetConditions(xpv1.Available())
	case "PENDING_CREATE", "PENDING_UPDATE":
		cr.Status.SetConditions(xpv1.Creating())
	case "PENDING_DELETE":
		cr.Status.SetConditions(xpv1.Deleting())
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
		if err := tags.Create(e.natV2Client, "nat_gateways", id, util.MapToResourceTags(toCreate)).
			ExtractErr(); err != nil {
			return err
		}
	}

	toDelete := util.TagDiff(current, desired)
	if len(toDelete) > 0 {
		if err := tags.Delete(e.natV2Client, "nat_gateways", id, util.MapToResourceTags(toDelete)).
			ExtractErr(); err != nil {
			return err
		}
	}

	return nil
}

func validateGatewayParameters(p natv1alpha1.GatewayParameters) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.Size == "" {
		return errors.New("size is required")
	}
	if pointer.Deref(p.VPCID, "") == "" {
		return errors.New(errEmptyVPCID)
	}
	if pointer.Deref(p.SubnetID, "") == "" {
		return errors.New(errEmptySubnetID)
	}
	return nil
}

func validateImmutableGatewayFields(
	spec natv1alpha1.GatewayParameters,
	observed natgateways.NatGateway,
) error {
	if spec.VPCID != nil && *spec.VPCID != observed.RouterID {
		return errors.New("vpcId is immutable after creation")
	}
	if spec.SubnetID != nil && *spec.SubnetID != observed.InternalNetworkID {
		return errors.New("subnetId is immutable after creation")
	}
	return nil
}

func lateInitializeGateway(
	cr *natv1alpha1.Gateway,
	observed natgateways.NatGateway,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.VPCID = util.LateInitPtrIfNonZero(p.VPCID, observed.RouterID, li)
	p.SubnetID = util.LateInitPtrIfNonZero(p.SubnetID, observed.InternalNetworkID, li)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isGatewayUpToDate(
	spec natv1alpha1.GatewayParameters,
	observed natgateways.NatGateway,
	observedTags map[string]string,
) bool {
	return spec.Name == observed.Name &&
		spec.Size == observed.Spec &&
		util.IsOptionalUpToDate(spec.VPCID, observed.RouterID) &&
		util.IsOptionalUpToDate(spec.SubnetID, observed.InternalNetworkID) &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalMapUpToDate(spec.Tags, observedTags)
}
