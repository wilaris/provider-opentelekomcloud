package securitygrouprule

import (
	"context"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/vpc/v3/security/rules"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage             = "cannot track ProviderConfig usage"
	errGetClient                = "cannot get OTC provider client"
	errCreateV3Client           = "cannot create VPC v3 client"
	errObserveSecurityGroupRule = "cannot observe SecurityGroupRule"
	errCreateSecurityGroupRule  = "cannot create SecurityGroupRule"
	errUpdateSecurityGroupRule  = "all SecurityGroupRule fields are immutable; updates are not supported"
	errDeleteSecurityGroupRule  = "cannot delete SecurityGroupRule"
)

// SetupGated adds a controller that reconciles SecurityGroupRule managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup SecurityGroupRule controller"))
		}
	}, networkv1alpha1.SecurityGroupRuleGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles SecurityGroupRule managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1alpha1.SecurityGroupRuleGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*networkv1alpha1.SecurityGroupRule](&connector{
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
			&networkv1alpha1.SecurityGroupRuleList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind networkv1alpha1.SecurityGroupRuleList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(networkv1alpha1.SecurityGroupRuleGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1alpha1.SecurityGroupRule{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*networkv1alpha1.SecurityGroupRule] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *networkv1alpha1.SecurityGroupRule,
) (managed.TypedExternalClient[*networkv1alpha1.SecurityGroupRule], error) {
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

var _ managed.TypedExternalClient[*networkv1alpha1.SecurityGroupRule] = (*external)(nil)

type external struct {
	vpcV3Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroupRule,
) (managed.ExternalObservation, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := rules.Get(e.vpcV3Client, externalName)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveSecurityGroupRule)
	}

	// set observation
	cr.Status.AtProvider = networkv1alpha1.SecurityGroupRuleObservation{
		ID:                   observed.ID,
		SecurityGroupID:      observed.SecurityGroupID,
		Direction:            observed.Direction,
		Description:          observed.Description,
		EtherType:            observed.Ethertype,
		Protocol:             observed.Protocol,
		MultiPort:            observed.Multiport,
		RemoteGroupID:        observed.RemoteGroupID,
		RemoteIPPrefix:       observed.RemoteIPPrefix,
		RemoteAddressGroupID: observed.RemoteAddressGroupID,
		Action:               observed.Action,
		Priority:             observed.Priority,
		ProjectID:            observed.ProjectID,
	}

	// SecurityGroupRule has no status field; available immediately after creation.
	cr.Status.SetConditions(xpv1.Available())

	li := resource.NewLateInitializer()
	lateInitializeSecurityGroupRule(cr, observed, li)

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isSecurityGroupRuleUpToDate(cr.Spec.ForProvider, observed),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroupRule,
) (managed.ExternalCreation, error) {
	createOpts := buildCreateOpts(cr.Spec.ForProvider)

	created, err := rules.Create(e.vpcV3Client, createOpts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSecurityGroupRule)
	}
	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	_ *networkv1alpha1.SecurityGroupRule,
) (managed.ExternalUpdate, error) {
	// The API does not support updating security group rules. All fields are immutable.
	return managed.ExternalUpdate{}, errors.New(errUpdateSecurityGroupRule)
}

func (e *external) Delete(
	_ context.Context,
	cr *networkv1alpha1.SecurityGroupRule,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if err := rules.Delete(e.vpcV3Client, externalName); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteSecurityGroupRule)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildCreateOpts(spec networkv1alpha1.SecurityGroupRuleParameters) rules.CreateOpts {
	ruleOpts := rules.SecurityGroupRuleOptions{
		SecurityGroupID: pointer.Deref(spec.SecurityGroupID, ""),
		Direction:       spec.Direction,
	}
	if spec.Description != nil {
		ruleOpts.Description = *spec.Description
	}
	if spec.EtherType != nil {
		ruleOpts.Ethertype = *spec.EtherType
	}
	if spec.Protocol != nil {
		ruleOpts.Protocol = *spec.Protocol
	}
	if spec.MultiPort != nil {
		ruleOpts.Multiport = *spec.MultiPort
	}
	if spec.RemoteIPPrefix != nil {
		ruleOpts.RemoteIPPrefix = *spec.RemoteIPPrefix
	}
	if spec.RemoteGroupID != nil {
		ruleOpts.RemoteGroupID = *spec.RemoteGroupID
	}
	if spec.Action != nil {
		ruleOpts.Action = *spec.Action
	}
	if spec.Priority != nil {
		ruleOpts.Priority = *spec.Priority
	}
	return rules.CreateOpts{SecurityGroupRule: ruleOpts}
}

func lateInitializeSecurityGroupRule(
	cr *networkv1alpha1.SecurityGroupRule,
	observed *rules.SecurityGroupRule,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.EtherType = util.LateInitPtrIfNonZero(p.EtherType, observed.Ethertype, li)
	p.Protocol = util.LateInitPtrIfNonZero(p.Protocol, observed.Protocol, li)
	p.MultiPort = util.LateInitPtrIfNonZero(p.MultiPort, observed.Multiport, li)
	p.RemoteGroupID = util.LateInitPtrIfNonZero(p.RemoteGroupID, observed.RemoteGroupID, li)
	p.RemoteIPPrefix = util.LateInitPtrIfNonZero(p.RemoteIPPrefix, observed.RemoteIPPrefix, li)
	p.Action = util.LateInitPtrIfNonZero(p.Action, observed.Action, li)
	p.Priority = util.LateInitPtrIfNonZero(p.Priority, observed.Priority, li)
}

func isSecurityGroupRuleUpToDate(
	spec networkv1alpha1.SecurityGroupRuleParameters,
	observed *rules.SecurityGroupRule,
) bool {
	return util.IsOptionalUpToDate(spec.SecurityGroupID, observed.SecurityGroupID) &&
		spec.Direction == observed.Direction &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalUpToDate(spec.EtherType, observed.Ethertype) &&
		util.IsOptionalUpToDate(spec.Protocol, observed.Protocol) &&
		util.IsOptionalUpToDate(spec.MultiPort, observed.Multiport) &&
		util.IsOptionalUpToDate(spec.RemoteGroupID, observed.RemoteGroupID) &&
		util.IsOptionalUpToDate(spec.RemoteIPPrefix, observed.RemoteIPPrefix) &&
		util.IsOptionalUpToDate(spec.Action, observed.Action) &&
		util.IsOptionalUpToDate(spec.Priority, observed.Priority)
}
