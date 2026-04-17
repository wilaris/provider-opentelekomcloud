package vpc

import (
	"context"
	"maps"
	"net"
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
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/vpcs"
	vpcsv3 "github.com/opentelekomcloud/gophertelekomcloud/openstack/vpc/v3/vpcs"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/clients"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

const (
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetClient         = "cannot get OTC provider client"
	errCreateV1Client    = "cannot create Network v1 client"
	errCreateV2Client    = "cannot create Network v2 client"
	errCreateV3Client    = "cannot create VPC v3 client"
	errValidateSpec      = "invalid VPC spec"
	errEmptyExternalName = "external name is empty"
	errObserveVPC        = "cannot observe VPC"
	errObserveTags       = "cannot observe VPC tags"
	errObserveCIDR       = "cannot observe VPC secondary CIDR"
	errCreateVPC         = "cannot create VPC"
	errUpdateVPC         = "cannot update VPC"
	errDeleteVPC         = "cannot delete VPC"
	errManySecondary     = "multiple secondary CIDRs are not supported"
)

// SetupGated adds a controller that reconciles VPC managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup VPC controller"))
		}
	}, networkv1alpha1.VPCGroupVersionKind)
	return nil
}

// Setup adds a controller that reconciles VPC managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(networkv1alpha1.VPCGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithTypedExternalConnector[*networkv1alpha1.VPC](&connector{
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
			&networkv1alpha1.VPCList{},
			o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(
				err,
				"cannot register MR state metrics recorder for kind networkv1alpha1.VPCList",
			)
		}
	}

	r := managed.NewReconciler(
		mgr,
		resource.ManagedKind(networkv1alpha1.VPCGroupVersionKind),
		opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&networkv1alpha1.VPC{}).
		Watches(&apisv1alpha1.ProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Watches(&apisv1alpha1.ClusterProviderConfig{}, &resource.EnqueueRequestForProviderConfig{}).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

var _ managed.TypedExternalConnector[*networkv1alpha1.VPC] = (*connector)(nil)

type connector struct {
	kube        client.Client
	usage       *resource.ProviderConfigUsageTracker
	clientCache *clients.Cache
}

func (c *connector) Connect(
	ctx context.Context,
	mg *networkv1alpha1.VPC,
) (managed.TypedExternalClient[*networkv1alpha1.VPC], error) {
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
	v3Client, err := openstack.NewVpcV3(providerClient.ProviderClient, endpointOpts)
	if err != nil {
		return nil, errors.Wrap(err, errCreateV3Client)
	}

	return &external{
		networkV1Client: v1Client,
		networkV2Client: v2Client,
		vpcV3Client:     v3Client,
	}, nil
}

var _ managed.TypedExternalClient[*networkv1alpha1.VPC] = (*external)(nil)

type external struct {
	networkV1Client *golangsdk.ServiceClient
	networkV2Client *golangsdk.ServiceClient
	vpcV3Client     *golangsdk.ServiceClient
}

func (e *external) Observe(
	_ context.Context,
	cr *networkv1alpha1.VPC,
) (managed.ExternalObservation, error) {
	if err := validateVPCParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observed, err := vpcs.Get(e.networkV1Client, externalName).Extract()
	if err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveVPC)
	}

	observedSecondaryCIDR, err := e.observeSecondaryCIDR(externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveCIDR)
	}

	observedTags, err := e.observeTags(externalName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveTags)
	}

	// set observation
	cr.Status.AtProvider = networkv1alpha1.VPCObservation{
		ID:            observed.ID,
		Name:          observed.Name,
		Description:   observed.Description,
		CIDR:          observed.CIDR,
		SecondaryCIDR: observedSecondaryCIDR,
		Status:        observed.Status,
		Tags:          maps.Clone(observedTags),
	}

	// set conditions
	e.setConditions(cr, observed.Status)

	li := resource.NewLateInitializer()
	lateInitializeVPC(cr, observed, observedSecondaryCIDR, observedTags, li)

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: isVPCUpToDate(
			cr.Spec.ForProvider,
			observed,
			observedSecondaryCIDR,
			observedTags,
		),
		ResourceLateInitialized: li.IsChanged(),
	}, nil
}

func (e *external) Create(
	_ context.Context,
	cr *networkv1alpha1.VPC,
) (managed.ExternalCreation, error) {
	if err := validateVPCParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errValidateSpec)
	}

	createOpts := buildVPCCreateOpts(cr.Spec.ForProvider)

	created, err := vpcs.Create(e.networkV1Client, createOpts).Extract()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateVPC)
	}
	meta.SetExternalName(cr, created.ID)

	if err := e.reconcileTags(
		created.ID,
		map[string]string{},
		cr.Spec.ForProvider.Tags,
	); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateVPC)
	}
	if err := e.reconcileSecondaryCIDR(
		created.ID,
		"",
		cr.Spec.ForProvider.SecondaryCIDR,
	); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateVPC)
	}

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(
	_ context.Context,
	cr *networkv1alpha1.VPC,
) (managed.ExternalUpdate, error) {
	if err := validateVPCParameters(cr.Spec.ForProvider); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalUpdate{}, errors.New(errEmptyExternalName)
	}

	observed, observedSecondaryCIDR, observedTags, err := e.observeCurrentState(externalName)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if err := validateImmutableVPCFields(cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errValidateSpec)
	}

	if err := e.update(externalName, cr.Spec.ForProvider, observed); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVPC)
	}

	if err := e.reconcileTags(externalName, observedTags, cr.Spec.ForProvider.Tags); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVPC)
	}
	if err := e.reconcileSecondaryCIDR(
		externalName,
		observedSecondaryCIDR,
		cr.Spec.ForProvider.SecondaryCIDR,
	); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateVPC)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(
	_ context.Context,
	cr *networkv1alpha1.VPC,
) (managed.ExternalDelete, error) {
	externalName := meta.GetExternalName(cr)
	if externalName == "" {
		return managed.ExternalDelete{}, nil
	}

	if err := vpcs.Delete(e.networkV1Client, externalName).ExtractErr(); err != nil {
		if util.IsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteVPC)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(context.Context) error {
	return nil
}

func buildVPCCreateOpts(spec networkv1alpha1.VPCParameters) vpcs.CreateOpts {
	createOpts := vpcs.CreateOpts{
		Name: spec.Name,
		CIDR: spec.CIDR,
	}
	if spec.Description != nil {
		createOpts.Description = *spec.Description
	}

	return createOpts
}

func (e *external) update(id string, spec networkv1alpha1.VPCParameters, observed *vpcs.Vpc) error {
	opts, needsUpdate := buildVPCUpdateOpts(spec, observed)
	if !needsUpdate {
		return nil
	}

	_, err := vpcs.Update(e.networkV1Client, id, opts).Extract()
	return err
}

func buildVPCUpdateOpts(
	spec networkv1alpha1.VPCParameters,
	observed *vpcs.Vpc,
) (vpcs.UpdateOpts, bool) {
	opts := vpcs.UpdateOpts{Name: spec.Name}
	needsUpdate := spec.Name != observed.Name
	if applyDescriptionIfChanged(&opts, spec.Description, observed.Description) {
		needsUpdate = true
	}

	return opts, needsUpdate
}

func applyDescriptionIfChanged(opts *vpcs.UpdateOpts, desired *string, observed string) bool {
	if desired == nil || *desired == observed {
		return false
	}
	description := *desired
	opts.Description = &description
	return true
}

func (e *external) observeCurrentState(id string) (*vpcs.Vpc, string, map[string]string, error) {
	observed, err := vpcs.Get(e.networkV1Client, id).Extract()
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errObserveVPC)
	}

	secondaryCIDR, err := e.observeSecondaryCIDR(id)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errObserveCIDR)
	}

	observedTags, err := e.observeTags(id)
	if err != nil {
		return nil, "", nil, errors.Wrap(err, errObserveTags)
	}

	return observed, secondaryCIDR, observedTags, nil
}

func (e *external) observeSecondaryCIDR(id string) (string, error) {
	v, err := vpcsv3.Get(e.vpcV3Client, id)
	if err != nil {
		return "", err
	}
	if len(v.SecondaryCidrs) > 1 {
		return "", errors.New(errManySecondary)
	}
	if len(v.SecondaryCidrs) == 0 {
		return "", nil
	}
	return v.SecondaryCidrs[0], nil
}

func (e *external) observeTags(id string) (map[string]string, error) {
	list, err := tags.Get(e.networkV2Client, "vpcs", id).Extract()
	if err != nil {
		return nil, err
	}
	return util.ResourceTagsToMap(list), nil
}

func (e *external) setConditions(cr *networkv1alpha1.VPC, observedStatus string) {
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
		if err := tags.Create(e.networkV2Client, "vpcs", id, util.MapToResourceTags(toCreate)).
			ExtractErr(); err != nil {
			return err
		}
	}

	toDelete := util.TagDiff(current, desired)
	if len(toDelete) > 0 {
		if err := tags.Delete(e.networkV2Client, "vpcs", id, util.MapToResourceTags(toDelete)).
			ExtractErr(); err != nil {
			return err
		}
	}

	return nil
}

func (e *external) reconcileSecondaryCIDR(id string, current string, desired *string) error {
	if desired == nil || *desired == current {
		return nil
	}

	target := *desired
	if target == "" {
		if current == "" {
			return nil
		}
		_, err := vpcsv3.RemoveSecondaryCidr(e.vpcV3Client, id, vpcsv3.CidrOpts{
			Vpc: &vpcsv3.AddExtendCidrOption{
				ExtendCidrs: []string{current},
			},
		})
		return err
	}

	if current != "" {
		if _, err := vpcsv3.RemoveSecondaryCidr(e.vpcV3Client, id, vpcsv3.CidrOpts{
			Vpc: &vpcsv3.AddExtendCidrOption{
				ExtendCidrs: []string{current},
			},
		}); err != nil {
			return err
		}
	}

	_, err := vpcsv3.AddSecondaryCidr(e.vpcV3Client, id, vpcsv3.CidrOpts{
		Vpc: &vpcsv3.AddExtendCidrOption{
			ExtendCidrs: []string{target},
		},
	})
	return err
}

func validateVPCParameters(p networkv1alpha1.VPCParameters) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.CIDR == "" {
		return errors.New("cidr is required")
	}
	if _, _, err := net.ParseCIDR(p.CIDR); err != nil {
		return errors.Wrap(err, "cidr must be a valid CIDR")
	}
	if p.SecondaryCIDR != nil && *p.SecondaryCIDR != "" {
		if _, _, err := net.ParseCIDR(*p.SecondaryCIDR); err != nil {
			return errors.Wrap(err, "secondaryCidr must be a valid CIDR or empty")
		}
	}
	return nil
}

func validateImmutableVPCFields(spec networkv1alpha1.VPCParameters, observed *vpcs.Vpc) error {
	if spec.CIDR != observed.CIDR {
		return errors.New("cidr is immutable after creation")
	}
	return nil
}

func lateInitializeVPC(
	cr *networkv1alpha1.VPC,
	observed *vpcs.Vpc,
	secondaryCIDR string,
	observedTags map[string]string,
	li *resource.LateInitializer,
) {
	p := &cr.Spec.ForProvider
	p.Description = util.LateInitPtrIfNonZero(p.Description, observed.Description, li)
	p.SecondaryCIDR = util.LateInitPtrIfNonZero(p.SecondaryCIDR, secondaryCIDR, li)
	p.Tags = util.LateInitMapIfNonEmpty(p.Tags, observedTags, li)
}

func isVPCUpToDate(
	spec networkv1alpha1.VPCParameters,
	observed *vpcs.Vpc,
	observedSecondaryCIDR string,
	observedTags map[string]string,
) bool {
	return spec.Name == observed.Name &&
		spec.CIDR == observed.CIDR &&
		util.IsOptionalUpToDate(spec.Description, observed.Description) &&
		util.IsOptionalUpToDate(spec.SecondaryCIDR, observedSecondaryCIDR) &&
		util.IsOptionalMapUpToDate(spec.Tags, observedTags)
}
