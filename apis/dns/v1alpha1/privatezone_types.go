package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// VPC defines a VPC association for a private DNS zone.
// +kubebuilder:validation:XValidation:rule="has(self.vpcId) || has(self.vpcIdRef) || has(self.vpcIdSelector)",message="one of vpcId, vpcIdRef or vpcIdSelector is required"
type VPC struct {
	// VPCID is the VPC ID to associate with the private zone.
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
}

// VPCObservation is the observed state of a VPC association.
type VPCObservation struct {
	// VPCID is the VPC ID.
	VPCID string `json:"vpcId,omitempty"`

	// Status is the association status (ACTIVE, PENDING, ERROR).
	Status string `json:"status,omitempty"`
}

// PrivateZoneParameters are the configurable fields of a PrivateZone.
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable after creation"
type PrivateZoneParameters struct {
	// Name is the DNS zone FQDN (e.g. "example.com.").
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Email is the SOA contact email for the zone.
	// +optional
	Email *string `json:"email,omitempty"`

	// TTL is the time to live in seconds for the zone SOA record.
	// +optional
	TTL *int `json:"ttl,omitempty"`

	// Description is the zone description.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// VPCs are VPC associations for this private zone.
	// At least one VPC is required.
	// +kubebuilder:validation:MinItems=1
	VPCs []VPC `json:"vpcs"`

	// Tags are resource tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// PrivateZoneObservation are the observable fields of a PrivateZone.
type PrivateZoneObservation struct {
	// ID is the zone UUID.
	ID string `json:"id,omitempty"`

	// Name is the observed zone FQDN.
	Name string `json:"name,omitempty"`

	// Email is the observed SOA contact email.
	Email string `json:"email,omitempty"`

	// TTL is the observed time to live in seconds.
	TTL int `json:"ttl,omitempty"`

	// Description is the observed zone description.
	Description string `json:"description,omitempty"`

	// Status is the zone lifecycle status (ACTIVE, PENDING, ERROR).
	Status string `json:"status,omitempty"`

	// Masters are the DNS master name servers.
	Masters []string `json:"masters,omitempty"`

	// VPCs are the observed VPC associations.
	VPCs []VPCObservation `json:"vpcs,omitempty"`

	// Tags are observed resource tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A PrivateZoneSpec defines the desired state of a PrivateZone.
type PrivateZoneSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              PrivateZoneParameters `json:"forProvider"`
}

// A PrivateZoneStatus represents the observed state of a PrivateZone.
type PrivateZoneStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          PrivateZoneObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A PrivateZone is a managed resource that represents an OpenTelekomCloud DNS private zone.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type PrivateZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PrivateZoneSpec   `json:"spec"`
	Status PrivateZoneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PrivateZoneList contains a list of PrivateZone
type PrivateZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PrivateZone `json:"items"`
}

// PrivateZone type metadata.
var (
	PrivateZoneKind             = reflect.TypeOf(PrivateZone{}).Name()
	PrivateZoneGroupKind        = schema.GroupKind{Group: Group, Kind: PrivateZoneKind}.String()
	PrivateZoneKindAPIVersion   = PrivateZoneKind + "." + SchemeGroupVersion.String()
	PrivateZoneGroupVersionKind = SchemeGroupVersion.WithKind(PrivateZoneKind)
)

func init() {
	SchemeBuilder.Register(&PrivateZone{}, &PrivateZoneList{})
}
