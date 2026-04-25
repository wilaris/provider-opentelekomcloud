package v1alpha1

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// ElasticIPAddress returns an ExtractValueFn that reads the allocated public IP
// address from an ElasticIP managed resource's status.
func ElasticIPAddress() reference.ExtractValueFn {
	return func(mg resource.Managed) string {
		eip, ok := mg.(*ElasticIP)
		if !ok {
			return ""
		}
		return eip.Status.AtProvider.PublicIPAddress
	}
}
