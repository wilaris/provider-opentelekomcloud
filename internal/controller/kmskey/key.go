package kmskey

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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/kms/v1/keys"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kmsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/kms/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetClient         = "cannot get OTC provider client"
	errCreateKMSClient   = "cannot create KMS v1 client"
	errEmptyExternalName = "external name is empty"
	errObserveKey        = "cannot observe Key"
	errObserveTags       = "cannot observe Key tags"
	errCreateKey         = "cannot create Key"
	errUpdateKey         = "cannot update Key"
	errDeleteKey         = "cannot delete Key"
	errImmutableKeyField = "immutable field changed"
	errValidateSpec      = "invalid Key spec"
)

const (
	keyStateWaitingForEnable = "WaitingForEnable"
	keyStateEnabled          = "Enabled"
	keyStateDisabled         = "Disabled"
	keyStatePendingDeletion  = "PendingDeletion"
	keyStateWaitingForImport = "WaitingForImport"
)

// KMS resource type used by the OTC tag service.
const tagResourceType = "kms"

// KMS origin value used by OTC for imported key material.
const keyOriginExternal = "external"

// Default deletion grace window when the user does not set spec.forProvider.pendingDays.
const defaultPendingDays = "7"

// keyStateFromAPI maps raw OTC key_state values ("1"-"5") to a user friendly form.
var keyStateFromAPI = map[string]string{
	"1": keyStateWaitingForEnable,
	"2": keyStateEnabled,
	"3": keyStateDisabled,
	"4": keyStatePendingDeletion,
	"5": keyStateWaitingForImport,
}

func mapKeyStateFromAPI(raw string) string {
	if v, ok := keyStateFromAPI[raw]; ok {
		return v
	}
	return raw
}

// keyUsageToAPI maps the user-facing KeyUsage enum to the OTC API form.
var keyUsageToAPI = map[string]string{
	"EncryptAndDecrypt": "Encrypt_Decrypt",
	"SignAndVerify":     "Sign_Verify",
}

// SetupGated adds a controller that reconciles Key managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup Key controller"))
		}
	}, kmsv1alpha1.KeyGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles KMS Key managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(kmsv1alpha1.KeyGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*kmsv1alpha1.Key](&connector{
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
		managed.WithPollIntervalHook(keyPollInterval),
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
			&kmsv1alpha1.KeyList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind kmsv1alpha1.KeyList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(kmsv1alpha1.KeyGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&kmsv1alpha1.Key{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// keyPollInterval polls fast while the CMK is transitioning, slow once stable.
func keyPollInterval(mg resource.Managed, pollInterval time.Duration) time.Duration {
	cr, ok := mg.(*kmsv1alpha1.Key)
	if !ok {
		return 30 * time.Second
	}
	if cr.GetDeletionTimestamp() != nil {
		return 30 * time.Second
	}
	switch cr.Status.AtProvider.KeyState {
	case keyStateEnabled, keyStateDisabled, keyStateWaitingForImport:
		return pollInterval
	default:
		return 30 * time.Second
	}
}

var _ managed.TypedExternalConnector[*kmsv1alpha1.Key] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *kmsv1alpha1.Key,
) (managed.TypedExternalClient[*kmsv1alpha1.Key], error) {
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
	kmsV1Client, err := openstack.NewKMSV1(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateKMSClient)
	}

	return &external{kmsV1Client: kmsV1Client}, nil
}

var _ managed.TypedExternalClient[*kmsv1alpha1.Key] = (*external)(nil)

type external struct {
	kmsV1Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *kmsv1alpha1.Key,
) (managed.ExternalObservation, error) {
	if err := validateKeyParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, observedTags, rotationStatus, err := e.observeCurrentState(
		externalName,
		cr.Spec.ForProvider,
	)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}

	state := mapKeyStateFromAPI(observed.KeyState)

	// set observation
	cr.Status.AtProvider = kmsv1alpha1.KeyObservation{
		KeyID:                 observed.KeyID,
		KeyAlias:              observed.KeyAlias,
		Description:           observed.KeyDescription,
		Realm:                 observed.Realm,
		KeyState:              state,
		DomainID:              observed.DomainID,
		ScheduledDeletionDate: observed.ScheduledDeletionDate,
		ExpirationTime:        observed.ExpirationTime,
		Tags:                  maps.Clone(observedTags),
	}

	if rotationStatus != nil {
		cr.Status.AtProvider.RotationEnabled = pointer.To(rotationStatus.Enabled)
		if rotationStatus.Interval != 0 {
			cr.Status.AtProvider.RotationInterval = pointer.To(rotationStatus.Interval)
		}
		cr.Status.AtProvider.Rotations = rotationStatus.NumberOfRotations
	}

	// set conditions
	setKeyConditions(cr, state)

	if state == keyStatePendingDeletion {
		// Treat out-of-band scheduled deletion as absent so Crossplane replaces
		// the key instead of trying to update a resource OTC is destroying.
		return managed.ExternalObservation{
			ResourceExists:   false,
			ResourceUpToDate: true,
		}, nil
	}

	li := resource.NewLateInitializer()
	lateInitializeKey(cr, observed, observedTags, rotationStatus, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isKeyUpToDate(
			cr.Spec.ForProvider,
			observed,
			observedTags,
			rotationStatus,
			state,
		),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *kmsv1alpha1.Key,
) (managed.ExternalCreation, error) {
	if meta.GetExternalName(cr) != "" && cr.Status.AtProvider.KeyState != keyStatePendingDeletion {
		return managed.ExternalCreation{}, nil
	}

	if err := validateKeyParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts := buildKeyCreateOpts(cr.Spec.ForProvider)
	created, err := keys.Create(e.kmsV1Client, createOpts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateKey)
	}

	meta.SetExternalName(cr, created.KeyID)
	cr.SetConditions(xpv1.Creating())

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *kmsv1alpha1.Key,
) (managed.ExternalUpdate, error) {
	if err := validateKeyParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	observed, observedTags, rotationStatus, err := e.observeCurrentState(
		externalName,
		cr.Spec.ForProvider,
	)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := validateImmutableKeyFields(cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errImmutableKeyField)
	}

	if err := e.updateAliasAndDescription(externalName, cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := e.reconcileEnabledState(
		externalName,
		cr.Spec.ForProvider.Enabled,
		mapKeyStateFromAPI(observed.KeyState),
	); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateKey)
	}

	if err := e.reconcileRotation(
		externalName,
		cr.Spec.ForProvider,
		rotationStatus,
		mapKeyStateFromAPI(observed.KeyState),
	); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateKey)
	}

	if err := e.reconcileTags(externalName, observedTags, cr.Spec.ForProvider.Tags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateKey)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *kmsv1alpha1.Key,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	cr.SetConditions(xpv1.Deleting())

	observed, err := keys.Get(e.kmsV1Client, externalName)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteKey)
	}

	state := mapKeyStateFromAPI(observed.KeyState)
	if state == keyStatePendingDeletion {
		return managed.ExternalDelete{}, nil
	}

	if canReconcileRotation(state) {
		rotationStatus, err := e.observeRotationStatus(externalName)
		if err != nil {
			if !isUnsupportedRotation(err, observed) {
				return managed.ExternalDelete{}, errors.Wrap(err, errDeleteKey)
			}
		} else if rotationStatus != nil && rotationStatus.Enabled {
			if err := keys.DisableKeyRotation(e.kmsV1Client, externalName); err != nil {
				return managed.ExternalDelete{}, errors.Wrap(err, errDeleteKey)
			}
		}
	}

	pendingDays := pointer.Deref(cr.Spec.ForProvider.PendingDays, defaultPendingDays)
	deleted, err := keys.Delete(
		e.kmsV1Client,
		keys.DeleteOpts{
			KeyID:       externalName,
			PendingDays: pendingDays,
		},
	)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteKey)
	}
	if deleted != nil && mapKeyStateFromAPI(deleted.KeyState) != keyStatePendingDeletion {
		return managed.ExternalDelete{}, errors.New(errDeleteKey)
	}
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildKeyCreateOpts(spec kmsv1alpha1.KeyParameters) keys.CreateOpts {
	return keys.CreateOpts{
		KeyAlias:       spec.KeyAlias,
		KeyDescription: pointer.Deref(spec.Description, ""),
		Realm:          pointer.Deref(spec.Realm, ""),
		KeyUsage:       mapKeyUsageToAPI(spec.KeyUsage),
	}
}

// mapKeyUsageToAPI translates the CRD enum value to the OTC API form.
// Returns "" when the spec value is nil so the SDK omits the field.
// Unknown values pass through unchanged (CEL admission already restricted them).
func mapKeyUsageToAPI(spec *string) string {
	if spec == nil {
		return ""
	}
	if v, ok := keyUsageToAPI[*spec]; ok {
		return v
	}
	return *spec
}

func validateImmutableKeyFields(spec kmsv1alpha1.KeyParameters, observed *keys.Key) error {
	if spec.Realm != nil && *spec.Realm != observed.Realm {
		return errors.New("realm is immutable after creation")
	}
	return nil
}

// isKeyUpToDate reports whether the spec matches the observed external state.
// state is the friendly form already translated by mapKeyStateFromAPI.
func isKeyUpToDate(
	spec kmsv1alpha1.KeyParameters,
	observed *keys.Key,
	observedTags map[string]string,
	rotationStatus *keys.KeyRotationResult,
	state string,
) bool {
	if spec.KeyAlias != observed.KeyAlias ||
		!util.IsOptionalUpToDate(spec.Description, observed.KeyDescription) {
		return false
	}
	if !isEnabledUpToDate(spec.Enabled, state) ||
		!util.IsOptionalMapUpToDate(spec.Tags, observedTags) {
		return false
	}
	return isRotationUpToDate(spec, rotationStatus, state)
}

func isRotationUpToDate(
	spec kmsv1alpha1.KeyParameters,
	rotationStatus *keys.KeyRotationResult,
	state string,
) bool {
	if !canReconcileRotation(state) {
		return true
	}
	if rotationStatus == nil {
		return true
	}
	if spec.RotationEnabled != nil && *spec.RotationEnabled != rotationStatus.Enabled {
		return false
	}
	if spec.RotationEnabled != nil && *spec.RotationEnabled && spec.RotationInterval != nil &&
		*spec.RotationInterval != rotationStatus.Interval {
		return false
	}
	return true
}

func canReconcileRotation(state string) bool {
	return state == keyStateEnabled || state == keyStateDisabled
}

func isRotationManaged(spec kmsv1alpha1.KeyParameters) bool {
	return spec.RotationEnabled != nil || spec.RotationInterval != nil
}

// isEnabledUpToDate reports whether the desired enabled flag matches the observed key state.
// A nil desired (user did not specify) is always considered up-to-date.
// Transitional states (WaitingForEnable, WaitingForImport) are also treated as
// up-to-date so the controller doesn't issue Enable/Disable while OTC is mid-transition.
func isEnabledUpToDate(desired *bool, keyState string) bool {
	if desired == nil {
		return true
	}
	switch keyState {
	case keyStateEnabled:
		return *desired
	case keyStateDisabled:
		return !*desired
	default:
		return true
	}
}

func (e *external) updateAliasAndDescription(
	keyID string,
	spec kmsv1alpha1.KeyParameters,
	observed *keys.Key,
) error {
	if spec.KeyAlias != observed.KeyAlias {
		_, err := keys.UpdateAlias(
			e.kmsV1Client,
			keys.UpdateAliasOpts{
				KeyID:    keyID,
				KeyAlias: spec.KeyAlias,
			},
		)
		if err != nil {
			return errors.Wrap(err, errUpdateKey)
		}
	}

	if desc := pointer.Deref(spec.Description, ""); desc != observed.KeyDescription {
		_, err := keys.UpdateDes(
			e.kmsV1Client,
			keys.UpdateDesOpts{
				KeyID:          keyID,
				KeyDescription: desc,
			},
		)
		if err != nil {
			return errors.Wrap(err, errUpdateKey)
		}
	}
	return nil
}

func (e *external) reconcileEnabledState(keyID string, desired *bool, observedState string) error {
	if desired == nil {
		return nil
	}
	switch observedState {
	case keyStateEnabled:
		if *desired {
			return nil
		}
		_, err := keys.DisableKey(e.kmsV1Client, keyID)
		return err
	case keyStateDisabled:
		if !*desired {
			return nil
		}
		_, err := keys.EnableKey(e.kmsV1Client, keyID)
		return err
	default:
		// WaitingForEnable, WaitingForImport, PendingDeletion. Observe will
		// re-run once the key stabilizes.
		return nil
	}
}

func (e *external) reconcileRotation(
	keyID string,
	spec kmsv1alpha1.KeyParameters,
	observedRotation *keys.KeyRotationResult,
	observedState string,
) error {
	if !canReconcileRotation(observedState) {
		return nil
	}
	if observedRotation == nil {
		return nil
	}

	desiredEnabled := pointer.Deref(spec.RotationEnabled, false)
	err := e.reconcileRotationEnabled(
		keyID,
		desiredEnabled,
		observedRotation.Enabled,
	)
	if err != nil {
		return err
	}

	return e.reconcileRotationInterval(keyID, spec, desiredEnabled, observedRotation.Interval)
}

func (e *external) reconcileRotationEnabled(
	keyID string,
	desiredEnabled, observedEnabled bool,
) error {
	if desiredEnabled && !observedEnabled {
		if err := keys.EnableKeyRotation(e.kmsV1Client, keyID); err != nil {
			return err
		}
	} else if !desiredEnabled && observedEnabled {
		if err := keys.DisableKeyRotation(e.kmsV1Client, keyID); err != nil {
			return err
		}
	}
	return nil
}

func (e *external) reconcileRotationInterval(
	keyID string,
	spec kmsv1alpha1.KeyParameters,
	desiredEnabled bool,
	observedInterval int,
) error {
	if desiredEnabled && spec.RotationInterval != nil &&
		*spec.RotationInterval != observedInterval {

		err := keys.UpdateKeyRotationInterval(
			e.kmsV1Client,
			keys.RotationOpts{
				KeyID:    keyID,
				Interval: *spec.RotationInterval,
			},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateKeyParameters(spec kmsv1alpha1.KeyParameters) error {
	if spec.KeyAlias == "" {
		return errors.New("keyAlias is required")
	}
	if spec.RotationInterval != nil && (spec.RotationEnabled == nil || !*spec.RotationEnabled) {
		return errors.New("rotationInterval cannot be set when rotationEnabled is false or nil")
	}
	return nil
}

func lateInitializeKey(
	cr *kmsv1alpha1.Key,
	observed *keys.Key,
	observedTags map[string]string,
	rotationStatus *keys.KeyRotationResult,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.KeyDescription, li)
	p.Realm = util.LateInitPtrIfNonZero(p.Realm, observed.Realm, li)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)

	if rotationStatus != nil {
		p.RotationEnabled = util.LateInitPtr(p.RotationEnabled, rotationStatus.Enabled, li)
		if rotationStatus.Enabled {
			p.RotationInterval = util.LateInitPtrIfNonZero(
				p.RotationInterval,
				rotationStatus.Interval,
				li,
			)
		}
	}
}

func setKeyConditions(cr *kmsv1alpha1.Key, keyState string) {
	switch keyState {
	case keyStateEnabled:
		cr.SetConditions(xpv1.Available())
	case keyStateWaitingForEnable:
		cr.SetConditions(xpv1.Creating())
	case keyStatePendingDeletion:
		cr.SetConditions(xpv1.Deleting())
	default:
		// Disabled and WaitingForImport are valid but not "Ready".
		cr.SetConditions(xpv1.Unavailable())
	}
}

func (e *external) observeCurrentState(
	id string,
	spec kmsv1alpha1.KeyParameters,
) (*keys.Key, map[string]string, *keys.KeyRotationResult, error) {
	observed, err := keys.Get(e.kmsV1Client, id)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, errObserveKey)
	}
	state := mapKeyStateFromAPI(observed.KeyState)
	if state == keyStatePendingDeletion {
		return observed, map[string]string{}, nil, nil
	}
	observedTags, err := e.observeTags(id)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, errObserveTags)
	}
	if !canReconcileRotation(state) {
		return observed, observedTags, nil, nil
	}
	rotationStatus, err := e.observeRotationStatus(id)
	if err != nil {
		if !isRotationManaged(spec) {
			return observed, observedTags, nil, nil
		}
		return nil, nil, nil, errors.Wrap(err, errObserveKey)
	}
	return observed, observedTags, rotationStatus, nil
}

func (e *external) observeRotationStatus(
	id string,
) (*keys.KeyRotationResult, error) {
	rotationStatus, err := keys.GetKeyRotationStatus(e.kmsV1Client, keys.RotationOpts{KeyID: id})
	if err != nil {
		return nil, err
	}
	return rotationStatus, nil
}

func isUnsupportedRotation(err error, observed *keys.Key) bool {
	return observed != nil && observed.Origin == keyOriginExternal && util.IsBadRequest(err)
}

func (e *external) observeTags(id string) (map[string]string, error) {
	list, err := tags.Get(e.kmsV1Client, tagResourceType, id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func (e *external) reconcileTags(
	id string,
	current map[string]string,
	desired map[string]string,
) error {
	if desired == nil {
		return nil
	}

	if toCreate := util.TagDiff(desired, current); len(toCreate) > 0 {
		err := tags.Create(e.kmsV1Client, tagResourceType, id, util.MapToResourceTags(toCreate)).
			ExtractErr()
		if err != nil {
			return err
		}
	}

	if toDelete := util.TagDiff(current, desired); len(toDelete) > 0 {
		err := tags.Delete(e.kmsV1Client, tagResourceType, id, util.MapToResourceTags(toDelete)).
			ExtractErr()
		if err != nil {
			return err
		}
	}

	return nil
}
