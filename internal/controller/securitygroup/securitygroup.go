package securitygroup

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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/vpc/v3/security/group"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage         = "cannot track ProviderConfig usage"
	errGetClient            = "cannot get OTC provider client"
	errCreateV3Client       = "cannot create VPC v3 client"
	errValidateSpec         = "invalid SecurityGroup spec"
	errEmptyExternalName    = "external name is empty"
	errObserveSecurityGroup = "cannot observe SecurityGroup"
	errCreateSecurityGroup  = "cannot create SecurityGroup"
	errUpdateSecurityGroup  = "cannot update SecurityGroup"
	errDeleteSecurityGroup  = "cannot delete SecurityGroup"
)

// SetupGated adds a controller that reconciles SecurityGroup managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup SecurityGroup controller"))
		}
	}, networkv1alpha1.SecurityGroupGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles SecurityGroup managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1alpha1.SecurityGroupGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*networkv1alpha1.SecurityGroup](&connector{
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
			&networkv1alpha1.SecurityGroupList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind networkv1alpha1.SecurityGroupList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(networkv1alpha1.SecurityGroupGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1alpha1.SecurityGroup{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*networkv1alpha1.SecurityGroup] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *networkv1alpha1.SecurityGroup,
) (managed.TypedExternalClient[*networkv1alpha1.SecurityGroup], error) {
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
	v3Client, err := openstack.NewVpcV3(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateV3Client)
	}

	return &external{vpcV3Client: v3Client}, nil
}

var _ managed.TypedExternalClient[*networkv1alpha1.SecurityGroup] = (*external)(nil)

type external struct {
	vpcV3Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroup,
) (managed.ExternalObservation, error) {
	if err := validateSecurityGroupParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := group.Get(e.vpcV3Client, externalName)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveSecurityGroup)
	}

	observedTags := util.ResourceTagsToMap(observed.Tags)

	// set observation
	cr.Status.AtProvider = networkv1alpha1.SecurityGroupObservation{
		ID:                  observed.ID,
		Name:                observed.Name,
		Description:         observed.Description,
		ProjectID:           observed.ProjectID,
		EnterpriseProjectID: observed.EnterpriseProjectID,
		Tags:                maps.Clone(observedTags),
	}

	// SecurityGroup has no status field; available immediately after creation.
	cr.Status.SetConditions(xpv1.Available())

	li := resource.NewLateInitializer()
	lateInitializeSecurityGroup(cr, observed, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isSecurityGroupUpToDate(
			cr.Spec.ForProvider,
			observed,
			observedTags,
		),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroup,
) (managed.ExternalCreation, error) {
	if meta.GetExternalName(cr) != "" {
		return managed.ExternalCreation{}, nil
	}

	if err := validateSecurityGroupParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts := buildSecurityGroupCreateOpts(cr.Spec.ForProvider)

	created, err := group.Create(e.vpcV3Client, createOpts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSecurityGroup)
	}
	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroup,
) (managed.ExternalUpdate, error) {
	if err := validateSecurityGroupParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	observed, err := group.Get(e.vpcV3Client, externalName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errObserveSecurityGroup)
	}

	if err := validateImmutableSecurityGroupFields(cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	opts, needsUpdate := buildSecurityGroupUpdateOpts(cr.Spec.ForProvider, observed)
	if !needsUpdate {
		return managed.ExternalUpdate{}, nil
	}

	if _, err := group.Update(e.vpcV3Client, externalName, opts); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateSecurityGroup)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroup,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if err := group.Delete(e.vpcV3Client, externalName); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteSecurityGroup)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildSecurityGroupCreateOpts(spec networkv1alpha1.SecurityGroupParameters) group.CreateOpts {
	opts := group.CreateOpts{
		SecurityGroup: group.SecurityGroupOptions{
			Name: spec.Name,
		},
	}
	if spec.Description != nil {
		opts.SecurityGroup.Description = *spec.Description
	}
	if spec.EnterpriseProjectID != nil {
		opts.SecurityGroup.EnterpriseProjectId = *spec.EnterpriseProjectID
	}
	if len(spec.Tags) > 0 {
		opts.SecurityGroup.Tags = util.MapToResourceTags(spec.Tags)
	}
	return opts
}

func buildSecurityGroupUpdateOpts(
	spec networkv1alpha1.SecurityGroupParameters,
	observed *group.SecurityGroup,
) (group.UpdateOpts, bool) {
	updateOptions := group.SecurityGroupUpdateOptions{}
	needsUpdate := false

	if spec.Name != observed.Name {
		updateOptions.Name = spec.Name
		needsUpdate = true
	}
	if spec.Description != nil && *spec.Description != observed.Description {
		updateOptions.Description = *spec.Description
		needsUpdate = true
	}

	return group.UpdateOpts{SecurityGroup: updateOptions}, needsUpdate
}

func validateSecurityGroupParameters(p networkv1alpha1.SecurityGroupParameters) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func validateImmutableSecurityGroupFields(
	spec networkv1alpha1.SecurityGroupParameters,
	observed *group.SecurityGroup,
) error {
	if spec.EnterpriseProjectID != nil &&
		*spec.EnterpriseProjectID != observed.EnterpriseProjectID {
		return errors.New("enterpriseProjectId is immutable after creation")
	}
	observedTags := util.ResourceTagsToMap(observed.Tags)
	if spec.Tags != nil && !maps.Equal(spec.Tags, observedTags) {
		return errors.New("tags are immutable after creation")
	}
	return nil
}

func lateInitializeSecurityGroup(
	cr *networkv1alpha1.SecurityGroup,
	observed *group.SecurityGroup,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.EnterpriseProjectID = util.LateInitPtrIfNonZero(
		p.EnterpriseProjectID,
		observed.EnterpriseProjectID,
		li,
	)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isSecurityGroupUpToDate(
	spec networkv1alpha1.SecurityGroupParameters,
	observed *group.SecurityGroup,
	observedTags map[string]string,
) bool {
	return spec.Name == observed.Name &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalUpToDate(spec.EnterpriseProjectID, observed.EnterpriseProjectID) &&
		util.IsOptionalMapUpToDate(spec.Tags, observedTags)
}
