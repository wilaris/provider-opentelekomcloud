package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// VPCParameters are the configurable fields of a VPC.
// +kubebuilder:validation:XValidation:rule="self.cidr == oldSelf.cidr",message="cidr is immutable after creation"
type VPCParameters struct {
	// Name is the VPC name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`

	// Description is the VPC description.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// CIDR is the primary CIDR block of the VPC.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=18
	CIDR string `json:"cidr"`

	// SecondaryCIDR is the optional secondary CIDR block.
	// Set to an empty string ("") to remove an existing secondary CIDR.
	// +optional
	// +kubebuilder:validation:MaxLength=18
	SecondaryCIDR *string `json:"secondaryCidr,omitempty"`

	// Tags are VPC tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// VPCObservation are the observable fields of a VPC.
type VPCObservation struct {
	// ID is the OTC VPC ID.
	ID string `json:"id,omitempty"`

	// Name is the observed VPC name.
	Name string `json:"name,omitempty"`

	// Description is the observed VPC description.
	Description string `json:"description,omitempty"`

	// CIDR is the observed primary CIDR.
	CIDR string `json:"cidr,omitempty"`

	// SecondaryCIDR is the observed secondary CIDR.
	SecondaryCIDR string `json:"secondaryCidr,omitempty"`

	// Status is the observed OTC VPC status.
	Status string `json:"status,omitempty"`

	// Tags are observed VPC tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A VPCSpec defines the desired state of a VPC.
type VPCSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              VPCParameters `json:"forProvider"`
}

// A VPCStatus represents the observed state of a VPC.
type VPCStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          VPCObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A VPC is a managed resource that represents an OpenTelekomCloud Virtual Private Cloud.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="CIDR",type="string",JSONPath=".spec.forProvider.cidr"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type VPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPCSpec   `json:"spec"`
	Status VPCStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VPCList contains a list of VPC
type VPCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPC `json:"items"`
}

// VPC type metadata.
var (
	VPCKind             = reflect.TypeOf(VPC{}).Name()
	VPCGroupKind        = schema.GroupKind{Group: Group, Kind: VPCKind}.String()
	VPCKindAPIVersion   = VPCKind + "." + SchemeGroupVersion.String()
	VPCGroupVersionKind = SchemeGroupVersion.WithKind(VPCKind)
)

func init() {
	SchemeBuilder.Register(&VPC{}, &VPCList{})
}
