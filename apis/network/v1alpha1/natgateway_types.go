package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// NATGatewayParameters are the configurable fields of a NATGateway.
// +kubebuilder:validation:XValidation:rule="has(self.vpcId) || has(self.vpcIdRef) || has(self.vpcIdSelector)",message="one of vpcId, vpcIdRef or vpcIdSelector is required"
// +kubebuilder:validation:XValidation:rule="oldSelf.vpcId == null || self.vpcId == oldSelf.vpcId",message="vpcId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="has(self.subnetId) || has(self.subnetIdRef) || has(self.subnetIdSelector)",message="one of subnetId, subnetIdRef or subnetIdSelector is required"
// +kubebuilder:validation:XValidation:rule="oldSelf.subnetId == null || self.subnetId == oldSelf.subnetId",message="subnetId is immutable after creation"
type NATGatewayParameters struct {
	// Name is the NAT gateway name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`

	// Description is the NAT gateway description.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// Size is the NAT gateway specification (tier).
	// "1" = small, "2" = medium, "3" = large, "4" = extra-large.
	// +kubebuilder:validation:Enum="1";"2";"3";"4"
	Size string `json:"size"`

	// VPCID is the ID of the VPC this NAT gateway belongs to.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.VPC
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	VPCID *string `json:"vpcId,omitempty"`

	// VPCIDRef is a namespaced reference to a VPC to populate vpcId.
	// +optional
	VPCIDRef *xpv1.NamespacedReference `json:"vpcIdRef,omitempty"`

	// VPCIDSelector selects a namespaced reference to a VPC to populate vpcId.
	// +optional
	VPCIDSelector *xpv1.NamespacedSelector `json:"vpcIdSelector,omitempty"`

	// SubnetID is the ID of the subnet for the NAT gateway.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.Subnet
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	SubnetID *string `json:"subnetId,omitempty"`

	// SubnetIDRef is a namespaced reference to a Subnet to populate subnetId.
	// +optional
	SubnetIDRef *xpv1.NamespacedReference `json:"subnetIdRef,omitempty"`

	// SubnetIDSelector selects a namespaced reference to a Subnet to populate subnetId.
	// +optional
	SubnetIDSelector *xpv1.NamespacedSelector `json:"subnetIdSelector,omitempty"`

	// Tags are NAT gateway tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// NATGatewayObservation are the observable fields of a NATGateway.
type NATGatewayObservation struct {
	// ID is the OTC NAT gateway ID.
	ID string `json:"id,omitempty"`

	// Name is the observed NAT gateway name.
	Name string `json:"name,omitempty"`

	// Description is the observed NAT gateway description.
	Description string `json:"description,omitempty"`

	// Size is the observed NAT gateway specification (tier).
	Size string `json:"size,omitempty"`

	// VPCID is the observed VPC (router) ID.
	VPCID string `json:"vpcId,omitempty"`

	// SubnetID is the observed subnet (internal network) ID.
	SubnetID string `json:"subnetId,omitempty"`

	// Status is the observed NAT gateway lifecycle status.
	Status string `json:"status,omitempty"`

	// AdminStateUp is the observed administrative state.
	AdminStateUp bool `json:"adminStateUp,omitempty"`

	// Tags are observed NAT gateway tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A NATGatewaySpec defines the desired state of a NATGateway.
type NATGatewaySpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              NATGatewayParameters `json:"forProvider"`
}

// A NATGatewayStatus represents the observed state of a NATGateway.
type NATGatewayStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          NATGatewayObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A NATGateway is a managed resource that represents an OpenTelekomCloud NAT Gateway.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="SIZE",type="string",JSONPath=".spec.forProvider.size"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type NATGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NATGatewaySpec   `json:"spec"`
	Status NATGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NATGatewayList contains a list of NATGateway
type NATGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NATGateway `json:"items"`
}

// NATGateway type metadata.
var (
	NATGatewayKind             = reflect.TypeOf(NATGateway{}).Name()
	NATGatewayGroupKind        = schema.GroupKind{Group: Group, Kind: NATGatewayKind}.String()
	NATGatewayKindAPIVersion   = NATGatewayKind + "." + SchemeGroupVersion.String()
	NATGatewayGroupVersionKind = SchemeGroupVersion.WithKind(NATGatewayKind)
)

func init() {
	SchemeBuilder.Register(&NATGateway{}, &NATGatewayList{})
}
