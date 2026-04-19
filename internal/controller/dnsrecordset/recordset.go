package dnsrecordset

import (
	"context"
	"fmt"
	"maps"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/dns/v2/recordsets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage     = "cannot track ProviderConfig usage"
	errGetClient        = "cannot get OTC provider client"
	errCreateDNSClient  = "cannot create DNS v2 client"
	errValidateSpec     = "invalid RecordSet spec"
	errEmptyExtName     = "external name is empty"
	errNoZoneID         = "neither privateZoneId nor publicZoneId is set"
	errObserveRecordSet = "cannot observe RecordSet"
	errObserveTags      = "cannot observe RecordSet tags"
	errCreateRecordSet  = "cannot create RecordSet"
	errUpdateRecordSet  = "cannot update RecordSet"
	errDeleteRecordSet  = "cannot delete RecordSet"
)

// SetupGated adds a controller that reconciles RecordSet managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup RecordSet controller"))
		}
	}, dnsv1alpha1.RecordSetGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles RecordSet managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(dnsv1alpha1.RecordSetGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*dnsv1alpha1.RecordSet](&connector{
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
			&dnsv1alpha1.RecordSetList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind dnsv1alpha1.RecordSetList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(dnsv1alpha1.RecordSetGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&dnsv1alpha1.RecordSet{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*dnsv1alpha1.RecordSet] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *dnsv1alpha1.RecordSet,
) (managed.TypedExternalClient[*dnsv1alpha1.RecordSet], error) {
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

var _ managed.TypedExternalClient[*dnsv1alpha1.RecordSet] = (*external)(nil)

type external struct {
	dnsV2Client *golangsdk.ServiceClient
}

type zoneInfo struct {
	zoneID         string
	tagServiceType string
}

func getZoneInfo(spec dnsv1alpha1.RecordSetParameters) (zoneInfo, error) {
	if spec.PrivateZoneID != nil && *spec.PrivateZoneID != "" {
		return zoneInfo{
			zoneID:         *spec.PrivateZoneID,
			tagServiceType: "DNS-private_recordset",
		}, nil
	}
	if spec.PublicZoneID != nil && *spec.PublicZoneID != "" {
		return zoneInfo{
			zoneID:         *spec.PublicZoneID,
			tagServiceType: "DNS-public_recordset",
		}, nil
	}
	return zoneInfo{}, errors.New(errNoZoneID)
}

func (e *external) Observe(
	_ context.Context,
	cr *dnsv1alpha1.RecordSet,
) (managed.ExternalObservation, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	zi, err := getZoneInfo(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	observed, err := recordsets.Get(e.dnsV2Client, zi.zoneID, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveRecordSet)
	}

	observedTags, err := e.observeTags(zi.tagServiceType, externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveTags)
	}

	trimmedRecords := trimTXTQuotes(observed.Type, observed.Records)

	// set observation
	cr.Status.AtProvider = dnsv1alpha1.RecordSetObservation{
		ID:          observed.ID,
		Name:        observed.Name,
		Type:        observed.Type,
		Records:     trimmedRecords,
		Description: observed.Description,
		TTL:         observed.TTL,
		Status:      observed.Status,
		Tags:        maps.Clone(observedTags),
	}

	// set conditions
	setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializeRecordSet(cr, observed, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isRecordSetUpToDate(cr.Spec.ForProvider, observed, observedTags),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *dnsv1alpha1.RecordSet,
) (managed.ExternalCreation, error) {
	if err := validateRecordSetParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	zi, err := getZoneInfo(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	createOpts := buildRecordSetCreateOpts(cr.Spec.ForProvider)

	created, err := recordsets.Create(e.dnsV2Client, zi.zoneID, createOpts).Extract()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRecordSet)
	}
	meta.SetExternalName(cr, created.ID)

	err = e.reconcileTags(
		zi.tagServiceType,
		created.ID,
		map[string]string{},
		cr.Spec.ForProvider.Tags,
	)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRecordSet)
	}

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *dnsv1alpha1.RecordSet,
) (managed.ExternalUpdate, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExtName)
	}

	zi, err := getZoneInfo(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	observed, observedTags, err := e.observeCurrentState(zi, externalName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := e.update(zi, externalName, cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateRecordSet)
	}

	err = e.reconcileTags(
		zi.tagServiceType,
		externalName,
		observedTags,
		cr.Spec.ForProvider.Tags,
	)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateRecordSet)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *dnsv1alpha1.RecordSet,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	zi, err := getZoneInfo(cr.Spec.ForProvider)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	err = recordsets.Delete(e.dnsV2Client, zi.zoneID, externalName).ExtractErr()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteRecordSet)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildRecordSetCreateOpts(spec dnsv1alpha1.RecordSetParameters) recordsets.CreateOpts {
	opts := recordsets.CreateOpts{
		Name:    spec.Name,
		Type:    spec.Type,
		Records: quoteTXTRecords(spec.Type, spec.Records),
	}
	if spec.Description != nil {
		opts.Description = *spec.Description
	}
	if spec.TTL != nil {
		opts.TTL = *spec.TTL
	}
	return opts
}

func (e *external) update(
	zi zoneInfo,
	id string,
	spec dnsv1alpha1.RecordSetParameters,
	observed recordsets.RecordSet,
) error {
	opts, needsUpdate := buildRecordSetUpdateOpts(spec, observed)
	if !needsUpdate {
		return nil
	}

	_, err := recordsets.Update(e.dnsV2Client, zi.zoneID, id, opts).Extract()
	return err
}

// buildRecordSetUpdateOpts compares the desired spec against the observed API
// state and returns UpdateOpts containing only the fields that changed.
//
// The DNS API requires the "records" field to be present in every update
// request, even when the records themselves have not changed. Omitting it causes
// the API to either reject the request or clear the existing records. Therefore,
// this function always populates opts.Records whenever any field triggers an
// update.
//
// For TXT record sets the API stores values wrapped in double-quotes
// (e.g. `"v=spf1 ~all"`), while users specify them without quotes. The
// comparison is done on the unquoted form (spec vs trimmed observed), and the
// quoted form is sent to the API.
func buildRecordSetUpdateOpts(
	spec dnsv1alpha1.RecordSetParameters,
	observed recordsets.RecordSet,
) (recordsets.UpdateOpts, bool) {
	var opts recordsets.UpdateOpts
	needsUpdate := false

	// Prepare the API-ready form of the desired records. For TXT record sets
	// this wraps each value in double-quotes; for all other types the slice is
	// returned as-is.
	quotedRecords := quoteTXTRecords(spec.Type, spec.Records)

	// Strip API-added quotes from observed records so both sides can be
	// compared in the same (unquoted) form. The API returns records as an
	// unordered set, so both sides are sorted before comparison.
	trimmedObserved := trimTXTQuotes(observed.Type, observed.Records)

	desiredSorted := slices.Sorted(slices.Values(spec.Records))
	observedSorted := slices.Sorted(slices.Values(trimmedObserved))
	if !slices.Equal(desiredSorted, observedSorted) {
		opts.Records = quotedRecords
		needsUpdate = true
	}

	if spec.Description != nil && *spec.Description != observed.Description {
		opts.Description = *spec.Description
		needsUpdate = true
	}
	if spec.TTL != nil && *spec.TTL != observed.TTL {
		opts.TTL = *spec.TTL
		needsUpdate = true
	}

	// Ensure records are always included when sending an update, even if only
	// description or TTL changed (OTC API requirement).
	if needsUpdate && opts.Records == nil {
		opts.Records = quotedRecords
	}

	return opts, needsUpdate
}

func (e *external) observeCurrentState(
	zi zoneInfo,
	id string,
) (recordsets.RecordSet, map[string]string, error) {
	observed, err := recordsets.Get(e.dnsV2Client, zi.zoneID, id).Extract()
	if err != nil {
		return recordsets.RecordSet{}, nil, errors.Wrap(err, errObserveRecordSet)
	}

	observedTags, err := e.observeTags(zi.tagServiceType, id)
	if err != nil {
		return recordsets.RecordSet{}, nil, errors.Wrap(err, errObserveTags)
	}

	return *observed, observedTags, nil
}

func (e *external) observeTags(tagServiceType, id string) (map[string]string, error) {
	list, err := tags.Get(e.dnsV2Client, tagServiceType, id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func setConditions(cr *dnsv1alpha1.RecordSet, observedStatus string) {
	switch observedStatus {
	case "ACTIVE":
		cr.Status.SetConditions(xpv1.Available())
	default:
		cr.Status.SetConditions(xpv1.Unavailable())
	}
}

func (e *external) reconcileTags(
	tagServiceType string,
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

func validateRecordSetParameters(p dnsv1alpha1.RecordSetParameters) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.Type == "" {
		return errors.New("type is required")
	}
	if len(p.Records) == 0 {
		return errors.New("records must contain at least one entry")
	}
	return nil
}

func lateInitializeRecordSet(
	cr *dnsv1alpha1.RecordSet,
	observed *recordsets.RecordSet,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.TTL = util.LateInitPtrIfNonZero(p.TTL, observed.TTL, li)
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isRecordSetUpToDate(
	spec dnsv1alpha1.RecordSetParameters,
	observed *recordsets.RecordSet,
	observedTags map[string]string,
) bool {
	trimmedRecords := trimTXTQuotes(observed.Type, observed.Records)

	desiredSorted := slices.Sorted(slices.Values(spec.Records))
	observedSorted := slices.Sorted(slices.Values(trimmedRecords))
	if !slices.Equal(desiredSorted, observedSorted) {
		return false
	}

	return util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalUpToDate(spec.TTL, observed.TTL) &&
		util.IsOptionalMapUpToDate(spec.Tags, observedTags)
}

func quoteTXTRecords(recordType string, records []string) []string {
	if recordType != "TXT" {
		return records
	}
	quoted := make([]string, len(records))
	for i, r := range records {
		quoted[i] = fmt.Sprintf(`"%s"`, r)
	}
	return quoted
}

func trimTXTQuotes(recordType string, records []string) []string {
	if recordType != "TXT" {
		return records
	}
	trimmed := make([]string, len(records))
	for i, r := range records {
		if len(r) >= 2 && r[0] == '"' && r[len(r)-1] == '"' {
			trimmed[i] = r[1 : len(r)-1]
		} else {
			trimmed[i] = r
		}
	}
	return trimmed
}
