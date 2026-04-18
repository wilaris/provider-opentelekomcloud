package controller

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"go.wilaris.de/provider-opentelekomcloud/internal/controller/config"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/elasticip"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/natgateway"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/securitygroup"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/securitygrouprule"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/snatrule"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/subnet"
	"go.wilaris.de/provider-opentelekomcloud/internal/controller/vpc"
)

// SetupGated creates all OpenTelekomCloud controllers with safe-start support and adds them to
// the supplied manager.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		config.Setup,
		elasticip.SetupGated,
		natgateway.SetupGated,
		securitygroup.SetupGated,
		securitygrouprule.SetupGated,
		snatrule.SetupGated,
		subnet.SetupGated,
		vpc.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
