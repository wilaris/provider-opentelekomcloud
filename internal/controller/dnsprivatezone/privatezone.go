package dnsprivatezone

import (
	"context"
	"maps"
	"slices"
	"sort"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/dns/v2/zones"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage    = "cannot track ProviderConfig usage"
	errGetClient       = "cannot get OTC provider client"
	errCreateDNSClient = "cannot create DNS v2 client"
	errValidateSpec    = "invalid PrivateZone spec"
	errEmptyExtName    = "external name is empty"
	errObserveZone     = "cannot observe PrivateZone"
	errObserveTags     = "cannot observe PrivateZone tags"
	errCreateZone      = "cannot create PrivateZone"
	errUpdateZone      = "cannot update PrivateZone"
	errDeleteZone      = "cannot delete PrivateZone"
	errAssociateVPC    = "cannot associate VPC with PrivateZone"
	errDisassocVPC     = "cannot disassociate VPC from PrivateZone"

	tagServiceType = "DNS-private_zone"
)

// SetupGated adds a controller that reconciles PrivateZone managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup PrivateZone controller"))
		}
	}, dnsv1alpha1.PrivateZoneGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles PrivateZone managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(dnsv1alpha1.PrivateZoneGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*dnsv1alpha1.PrivateZone](&connector{
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
			&dnsv1alpha1.PrivateZoneList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind dnsv1alpha1.PrivateZoneList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(dnsv1alpha1.PrivateZoneGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&dnsv1alpha1.PrivateZone{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*dnsv1alpha1.PrivateZone] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *dnsv1alpha1.PrivateZone,
) (managed.TypedExternalClient[*dnsv1alpha1.PrivateZone], error) {
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
	dnsV2Client, err := openstack.NewDNSV2(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateDNSClient)
	}

	return &external{
		dnsV2Client: dnsV2Client,
	}, nil
}

var _ managed.TypedExternalClient[*dnsv1alpha1.PrivateZone] = (*external)(nil)

type external struct {
	dnsV2Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *dnsv1alpha1.PrivateZone,
) (managed.ExternalObservation, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := zones.Get(e.dnsV2Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveZone)
	}

	observedTags, err := e.observeTags(externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveTags)
	}

	// set observation
	cr.Status.AtProvider = dnsv1alpha1.PrivateZoneObservation{
		ID:          observed.ID,
		Name:        observed.Name,
		Email:       observed.Email,
		TTL:         observed.TTL,
		Description: observed.Description,
		Status:      observed.Status,
		Masters:     observed.Masters,
		VPCs:        toVPCObservations(observed.Routers),
		Tags:        maps.Clone(observedTags),
	}

	// set conditions
	e.setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializePrivateZone(cr, observed, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isPrivateZoneUpToDate(cr.Spec.ForProvider, observed, observedTags),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *dnsv1alpha1.PrivateZone,
) (managed.ExternalCreation, error) {
	if err := validatePrivateZoneParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts := buildPrivateZoneCreateOpts(cr.Spec.ForProvider)

	created, err := zones.Create(e.dnsV2Client, createOpts).Extract()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateZone)
	}
	meta.SetExternalName(cr, created.ID)

	// Associate additional VPCs beyond the first (which was included in CreateOpts).
	for i := 1; i < len(cr.Spec.ForProvider.VPCs); i++ {
		v := cr.Spec.ForProvider.VPCs[i]
		routerOpts := zones.RouterOpts{
			RouterID: pointer.Deref(v.VPCID, ""),
		}
		_, err := zones.AssociateZone(e.dnsV2Client, created.ID, routerOpts).Extract()
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errAssociateVPC)
		}
	}

	err = e.reconcileTags(
		created.ID,
		map[string]string{},
		cr.Spec.ForProvider.Tags,
	)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateZone)
	}

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *dnsv1alpha1.PrivateZone,
) (managed.ExternalUpdate, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExtName)
	}

	observed, observedTags, err := e.observeCurrentState(externalName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := e.update(externalName, cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateZone)
	}

	err = e.reconcileVPCs(
		externalName,
		cr.Spec.ForProvider.VPCs,
		observed.Routers,
	)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := e.reconcileTags(externalName, observedTags, cr.Spec.ForProvider.Tags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateZone)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *dnsv1alpha1.PrivateZone,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	_, err := zones.Delete(e.dnsV2Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteZone)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildPrivateZoneCreateOpts(spec dnsv1alpha1.PrivateZoneParameters) zones.CreateOpts {
	opts := zones.CreateOpts{
		Name:     spec.Name,
		ZoneType: "private",
	}
	if spec.Email != nil {
		opts.Email = *spec.Email
	}
	if spec.TTL != nil {
		opts.TTL = *spec.TTL
	}
	if spec.Description != nil {
		opts.Description = *spec.Description
	}
	// First VPC is included in create opts.
	if len(spec.VPCs) > 0 {
		opts.Router = &zones.RouterOpts{
			RouterID: pointer.Deref(spec.VPCs[0].VPCID, ""),
		}
	}
	return opts
}

func (e *external) update(
	id string,
	spec dnsv1alpha1.PrivateZoneParameters,
	observed zones.Zone,
) error {
	opts, needsUpdate := buildPrivateZoneUpdateOpts(spec, observed)
	if !needsUpdate {
		return nil
	}

	_, err := zones.Update(e.dnsV2Client, id, opts).Extract()
	return err
}

func buildPrivateZoneUpdateOpts(
	spec dnsv1alpha1.PrivateZoneParameters,
	observed zones.Zone,
) (zones.UpdateOpts, bool) {
	var opts zones.UpdateOpts
	needsUpdate := false

	if spec.Email != nil && *spec.Email != observed.Email {
		opts.Email = *spec.Email
		needsUpdate = true
	}
	if spec.TTL != nil && *spec.TTL != observed.TTL {
		opts.TTL = *spec.TTL
		needsUpdate = true
	}
	if spec.Description != nil && *spec.Description != observed.Description {
		opts.Description = *spec.Description
		needsUpdate = true
	}

	return opts, needsUpdate
}

func (e *external) reconcileVPCs(
	zoneID string,
	desired []dnsv1alpha1.VPC,
	observed []zones.RouterResult,
) error {
	// Build a set of observed VPC IDs.
	observedSet := make(map[string]struct{}, len(observed))
	for _, r := range observed {
		observedSet[r.RouterID] = struct{}{}
	}

	// Build a set of desired VPC IDs.
	desiredSet := make(map[string]struct{}, len(desired))
	for _, v := range desired {
		desiredSet[pointer.Deref(v.VPCID, "")] = struct{}{}
	}

	// Associate VPCs that are desired but not observed.
	for _, v := range desired {
		vpcID := pointer.Deref(v.VPCID, "")
		if _, exists := observedSet[vpcID]; !exists {
			routerOpts := zones.RouterOpts{
				RouterID: vpcID,
			}
			if _, err := zones.AssociateZone(e.dnsV2Client, zoneID, routerOpts).
				Extract(); err != nil {
				return errors.Wrap(err, errAssociateVPC)
			}
		}
	}

	// Disassociate VPCs that are observed but not desired.
	for _, r := range observed {
		if _, exists := desiredSet[r.RouterID]; !exists {
			routerOpts := zones.RouterOpts{
				RouterID: r.RouterID,
			}
			if _, err := zones.DisassociateZone(e.dnsV2Client, zoneID, routerOpts).
				Extract(); err != nil {
				return errors.Wrap(err, errDisassocVPC)
			}
		}
	}

	return nil
}

func (e *external) observeCurrentState(
	id string,
) (zones.Zone, map[string]string, error) {
	observed, err := zones.Get(e.dnsV2Client, id).Extract()
	if err != nil {
		return zones.Zone{}, nil, errors.Wrap(err, errObserveZone)
	}

	observedTags, err := e.observeTags(id)
	if err != nil {
		return zones.Zone{}, nil, errors.Wrap(err, errObserveTags)
	}

	return *observed, observedTags, nil
}

func (e *external) observeTags(id string) (map[string]string, error) {
	list, err := tags.Get(e.dnsV2Client, tagServiceType, id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func (e *external) setConditions(cr *dnsv1alpha1.PrivateZone, observedStatus string) {
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
		err := tags.Create(e.dnsV2Client, tagServiceType, id, util.MapToResourceTags(toCreate)).
			ExtractErr()
		if err != nil {
			return err
		}
	}

	toDelete := util.TagDiff(current, desired)
	if len(toDelete) > 0 {
		err := tags.Delete(e.dnsV2Client, tagServiceType, id, util.MapToResourceTags(toDelete)).
			ExtractErr()
		if err != nil {
			return err
		}
	}

	return nil
}

func validatePrivateZoneParameters(p dnsv1alpha1.PrivateZoneParameters) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if len(p.VPCs) == 0 {
		return errors.New("at least one VPC is required")
	}
	for i, v := range p.VPCs {
		if pointer.Deref(v.VPCID, "") == "" {
			return errors.Errorf("vpcs[%d].vpcId is required", i)
		}
	}
	return nil
}

func lateInitializePrivateZone(
	cr *dnsv1alpha1.PrivateZone,
	observed *zones.Zone,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Email = util.LateInitPtrIfNonZero(p.Email, observed.Email, li)
	p.TTL = util.LateInitPtrIfNonZero(p.TTL, observed.TTL, li)
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isPrivateZoneUpToDate(
	spec dnsv1alpha1.PrivateZoneParameters,
	observed *zones.Zone,
	observedTags map[string]string,
) bool {
	return util.IsOptionalUpToDate(spec.Email, observed.Email) &&
		util.IsOptionalUpToDate(spec.TTL, observed.TTL) &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalMapUpToDate(spec.Tags, observedTags) &&
		isVPCsUpToDate(spec.VPCs, observed.Routers)
}

func isVPCsUpToDate(desired []dnsv1alpha1.VPC, observed []zones.RouterResult) bool {
	if len(desired) != len(observed) {
		return false
	}

	// Sort both by VPC ID for comparison.
	desiredIDs := make([]string, len(desired))
	for i, v := range desired {
		desiredIDs[i] = pointer.Deref(v.VPCID, "")
	}
	sort.Strings(desiredIDs)

	observedIDs := make([]string, len(observed))
	for i, r := range observed {
		observedIDs[i] = r.RouterID
	}
	sort.Strings(observedIDs)

	return slices.Equal(desiredIDs, observedIDs)
}

func toVPCObservations(routers []zones.RouterResult) []dnsv1alpha1.VPCObservation {
	if len(routers) == 0 {
		return nil
	}
	result := make([]dnsv1alpha1.VPCObservation, len(routers))
	for i, r := range routers {
		result[i] = dnsv1alpha1.VPCObservation{
			VPCID:  r.RouterID,
			Status: r.Status,
		}
	}
	return result
}
