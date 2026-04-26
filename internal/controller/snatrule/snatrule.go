package snatrule

import (
	"context"
	"strconv"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/snatrules"
	"k8s.io/apimachinery/pkg/util/sets"
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
	errObserveSNATRule   = "cannot observe SNATRule"
	errCreateSNATRule    = "cannot create SNATRule"
	errUpdateSNATRule    = "all SNATRule fields are immutable; updates are not supported"
	errDeleteSNATRule    = "cannot delete SNATRule"
	errEmptyNatGateway   = "resolved natGatewayId is empty"
	errEmptyElasticIP    = "resolved elasticIpId is empty"
	errMutuallyExclusive = "subnetId and cidr are mutually exclusive; specify at most one"
	errNeitherSubnetNor  = "one of subnetId or cidr is required"
)

// SetupGated adds a controller that reconciles SNATRule managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup SNATRule controller"))
		}
	}, natv1alpha1.SNATRuleGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles SNATRule managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(natv1alpha1.SNATRuleGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*natv1alpha1.SNATRule](&connector{
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
		opts = append(
			opts,
			managed.WithManagementPolicies(),
			managed.WithReconcilerSupportedManagementPolicies([]sets.Set[xpv1.ManagementAction]{
				sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
				sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionCreate,
					xpv1.ManagementActionDelete,
				),
			}),
		)
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
			&natv1alpha1.SNATRuleList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind natv1alpha1.SNATRuleList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(natv1alpha1.SNATRuleGroupVersionKind),
		opts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&natv1alpha1.SNATRule{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*natv1alpha1.SNATRule] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *natv1alpha1.SNATRule,
) (managed.TypedExternalClient[*natv1alpha1.SNATRule], error) {
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

	return &external{natV2Client: natV2Client}, nil
}

var _ managed.TypedExternalClient[*natv1alpha1.SNATRule] = (*external)(nil)

type external struct {
	natV2Client *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *natv1alpha1.SNATRule,
) (managed.ExternalObservation, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := snatrules.Get(e.natV2Client, externalName)
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveSNATRule)
	}

	sourceType := convertSourceType(observed.SourceType)

	// set observation
	cr.Status.AtProvider = natv1alpha1.SNATRuleObservation{
		ID:               observed.ID,
		NatGatewayID:     observed.NatGatewayID,
		SubnetID:         observed.NetworkID,
		ElasticIPID:      observed.FloatingIPID,
		ElasticIPAddress: observed.FloatingIPAddress,
		CIDR:             observed.Cidr,
		SourceType:       sourceType,
		Status:           observed.Status,
		AdminStateUp:     observed.AdminStateUp,
		TenantID:         observed.TenantID,
		Description:      observed.Description,
	}

	// set conditions
	e.setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializeSNATRule(cr, observed, sourceType, li)

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        isSNATRuleUpToDate(cr.Spec.ForProvider, observed, sourceType),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *natv1alpha1.SNATRule,
) (managed.ExternalCreation, error) {
	if err := validateSNATRuleParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, err
	}

	createOpts := buildCreateOpts(cr.Spec.ForProvider)

	created, err := snatrules.Create(e.natV2Client, createOpts)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSNATRule)
	}
	meta.SetExternalName(cr, created.ID)

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	_ *natv1alpha1.SNATRule,
) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, errors.New(errUpdateSNATRule)
}

func (e *external) Delete(
	_ context.Context,
	cr *natv1alpha1.SNATRule,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if err := snatrules.Delete(e.natV2Client, externalName); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteSNATRule)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func validateSNATRuleParameters(p natv1alpha1.SNATRuleParameters) error {
	if pointer.Deref(p.NatGatewayID, "") == "" {
		return errors.New(errEmptyNatGateway)
	}
	if pointer.Deref(p.ElasticIPID, "") == "" {
		return errors.New(errEmptyElasticIP)
	}

	hasSubnet := pointer.Deref(p.SubnetID, "") != ""
	hasCIDR := pointer.Deref(p.CIDR, "") != ""
	if hasSubnet && hasCIDR {
		return errors.New(errMutuallyExclusive)
	}
	if !hasSubnet && !hasCIDR {
		return errors.New(errNeitherSubnetNor)
	}

	return nil
}

func buildCreateOpts(spec natv1alpha1.SNATRuleParameters) snatrules.CreateOpts {
	opts := snatrules.CreateOpts{
		NatGatewayID: pointer.Deref(spec.NatGatewayID, ""),
		FloatingIPID: pointer.Deref(spec.ElasticIPID, ""),
		SourceType:   sourceTypeToInt(spec.SourceType),
	}
	if spec.SubnetID != nil {
		opts.NetworkID = *spec.SubnetID
	}
	if spec.CIDR != nil {
		opts.Cidr = *spec.CIDR
	}
	if spec.Description != nil {
		opts.Description = *spec.Description
	}
	return opts
}

func (e *external) setConditions(cr *natv1alpha1.SNATRule, observedStatus string) {
	switch observedStatus {
	case "ACTIVE":
		cr.Status.SetConditions(xpv1.Available())
	default:
		cr.Status.SetConditions(xpv1.Unavailable())
	}
}

func convertSourceType(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		i, _ := strconv.Atoi(t)
		return i
	case int:
		return t
	default:
		return 0
	}
}

func sourceTypeToInt(s *string) int {
	// Currently only "VPC" (0) is supported. When additional source types are
	// added (e.g. "DirectConnect" = 1), extend this mapping.
	return 0 //nolint:unparam
}

func sourceTypeIntToString(i int) string {
	switch i {
	case 0:
		return "VPC"
	default:
		return ""
	}
}

func lateInitializeSNATRule(
	cr *natv1alpha1.SNATRule,
	observed *snatrules.SnatRule,
	sourceType int,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.SubnetID = util.LateInitPtrIfNonZero(p.SubnetID, observed.NetworkID, li)
	p.CIDR = util.LateInitPtrIfNonZero(p.CIDR, observed.Cidr, li)
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)

	// SourceType needs custom handling: 0 is both the Go zero-value and valid "VPC".
	if p.SourceType == nil {
		if s := sourceTypeIntToString(sourceType); s != "" {
			p.SourceType = &s
			li.SetChanged()
		}
	}
}

func isSNATRuleUpToDate(
	spec natv1alpha1.SNATRuleParameters,
	observed *snatrules.SnatRule,
	sourceType int,
) bool {
	return util.IsOptionalUpToDate(spec.NatGatewayID, observed.NatGatewayID) &&
		util.IsOptionalUpToDate(spec.ElasticIPID, observed.FloatingIPID) &&
		util.IsOptionalUpToDate(spec.SubnetID, observed.NetworkID) &&
		util.IsOptionalUpToDate(spec.CIDR, observed.Cidr) &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		isSourceTypeUpToDate(spec.SourceType, sourceType)
}

func isSourceTypeUpToDate(desired *string, observed int) bool {
	if desired == nil {
		return true
	}
	return sourceTypeToInt(desired) == observed
}
