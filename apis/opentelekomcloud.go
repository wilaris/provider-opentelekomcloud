// Package apis contains Kubernetes API for the OpenTelekomCloud provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	opentelekomcloudv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		networkv1alpha1.SchemeBuilder.AddToScheme,
		opentelekomcloudv1alpha1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
