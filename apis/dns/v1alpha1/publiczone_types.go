package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// PublicZoneParameters are the configurable fields of a PublicZone.
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable after creation"
type PublicZoneParameters struct {
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

	// Tags are resource tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// PublicZoneObservation are the observable fields of a PublicZone.
type PublicZoneObservation struct {
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

	// Tags are observed resource tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A PublicZoneSpec defines the desired state of a PublicZone.
type PublicZoneSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              PublicZoneParameters `json:"forProvider"`
}

// A PublicZoneStatus represents the observed state of a PublicZone.
type PublicZoneStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          PublicZoneObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A PublicZone is a managed resource that represents an OpenTelekomCloud DNS public zone.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type PublicZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PublicZoneSpec   `json:"spec"`
	Status PublicZoneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PublicZoneList contains a list of PublicZone
type PublicZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PublicZone `json:"items"`
}

// PublicZone type metadata.
var (
	PublicZoneKind             = reflect.TypeOf(PublicZone{}).Name()
	PublicZoneGroupKind        = schema.GroupKind{Group: Group, Kind: PublicZoneKind}.String()
	PublicZoneKindAPIVersion   = PublicZoneKind + "." + SchemeGroupVersion.String()
	PublicZoneGroupVersionKind = SchemeGroupVersion.WithKind(PublicZoneKind)
)

func init() {
	SchemeBuilder.Register(&PublicZone{}, &PublicZoneList{})
}
