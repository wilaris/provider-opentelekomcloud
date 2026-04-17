package clients

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apisv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
)

const (
	errMissingPCRef      = "managed resource has no ProviderConfig reference"
	errGetProviderConfig = "cannot get ProviderConfig"
	errUnsupportedPCKind = "unsupported provider config kind"
)

// GetProviderConfigSpec retrieves the ProviderConfigSpec and a cache key for a managed resource.
func GetProviderConfigSpec(
	ctx context.Context,
	kube client.Client,
	mg resource.ModernManaged,
) (apisv1alpha1.ProviderConfigSpec, string, error) {
	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return apisv1alpha1.ProviderConfigSpec{}, "", errors.New(errMissingPCRef)
	}

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		nn := types.NamespacedName{Name: ref.Name, Namespace: mg.GetNamespace()}
		if err := kube.Get(ctx, nn, pc); err != nil {
			return apisv1alpha1.ProviderConfigSpec{}, "", errors.Wrap(err, errGetProviderConfig)
		}
		key := fmt.Sprintf("ProviderConfig/%s/%s", nn.Namespace, nn.Name)
		return pc.Spec, key, nil
	case "", "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		nn := types.NamespacedName{Name: ref.Name}
		if err := kube.Get(ctx, nn, cpc); err != nil {
			return apisv1alpha1.ProviderConfigSpec{}, "", errors.Wrap(err, errGetProviderConfig)
		}
		key := fmt.Sprintf("ClusterProviderConfig/%s", nn.Name)
		return cpc.Spec, key, nil
	default:
		return apisv1alpha1.ProviderConfigSpec{}, "", errors.Errorf(
			"%s: %s",
			errUnsupportedPCKind,
			ref.Kind,
		)
	}
}
