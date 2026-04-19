package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// RecordSetParameters are the configurable fields of a RecordSet.
// +kubebuilder:validation:XValidation:rule="(has(self.privateZoneId) || has(self.privateZoneIdRef) || has(self.privateZoneIdSelector)) != (has(self.publicZoneId) || has(self.publicZoneIdRef) || has(self.publicZoneIdSelector))",message="exactly one of privateZone or publicZone must be specified"
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.type == oldSelf.type",message="type is immutable after creation"
type RecordSetParameters struct {
	// PrivateZoneID is the ID of the private DNS zone this record set belongs to.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1.PrivateZone
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	PrivateZoneID *string `json:"privateZoneId,omitempty"`

	// PrivateZoneIDRef is a namespaced reference to a PrivateZone to populate privateZoneId.
	// +optional
	PrivateZoneIDRef *xpv1.NamespacedReference `json:"privateZoneIdRef,omitempty"`

	// PrivateZoneIDSelector selects a namespaced reference to a PrivateZone to populate privateZoneId.
	// +optional
	PrivateZoneIDSelector *xpv1.NamespacedSelector `json:"privateZoneIdSelector,omitempty"`

	// PublicZoneID is the ID of the public DNS zone this record set belongs to.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1.PublicZone
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	PublicZoneID *string `json:"publicZoneId,omitempty"`

	// PublicZoneIDRef is a namespaced reference to a PublicZone to populate publicZoneId.
	// +optional
	PublicZoneIDRef *xpv1.NamespacedReference `json:"publicZoneIdRef,omitempty"`

	// PublicZoneIDSelector selects a namespaced reference to a PublicZone to populate publicZoneId.
	// +optional
	PublicZoneIDSelector *xpv1.NamespacedSelector `json:"publicZoneIdSelector,omitempty"`

	// Name is the DNS record name (FQDN, e.g. "www.example.com."). Immutable after creation.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Type is the DNS record type. Immutable after creation.
	// +kubebuilder:validation:Enum=A;AAAA;MX;CNAME;TXT;NS;SRV;PTR;CAA
	Type string `json:"type"`

	// Records is the list of DNS record values.
	// +kubebuilder:validation:MinItems=1
	Records []string `json:"records"`

	// Description of the record set.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// TTL is the time to live in seconds. Defaults to 300 server-side.
	// +optional
	TTL *int `json:"ttl,omitempty"`

	// Tags are resource tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// RecordSetObservation are the observable fields of a RecordSet.
type RecordSetObservation struct {
	// ID is the record set UUID.
	ID string `json:"id,omitempty"`

	// Name is the observed DNS record name.
	Name string `json:"name,omitempty"`

	// Type is the observed DNS record type.
	Type string `json:"type,omitempty"`

	// Records is the observed list of DNS record values.
	Records []string `json:"records,omitempty"`

	// Description is the observed description.
	Description string `json:"description,omitempty"`

	// TTL is the observed time to live in seconds.
	TTL int `json:"ttl,omitempty"`

	// Status is the record set lifecycle status (ACTIVE, PENDING, ERROR).
	Status string `json:"status,omitempty"`

	// Tags are observed resource tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A RecordSetSpec defines the desired state of a RecordSet.
type RecordSetSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              RecordSetParameters `json:"forProvider"`
}

// A RecordSetStatus represents the observed state of a RecordSet.
type RecordSetStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          RecordSetObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A RecordSet is a managed resource that represents an OpenTelekomCloud DNS record set.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.forProvider.type"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type RecordSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RecordSetSpec   `json:"spec"`
	Status RecordSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RecordSetList contains a list of RecordSet
type RecordSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RecordSet `json:"items"`
}

// RecordSet type metadata.
var (
	RecordSetKind             = reflect.TypeOf(RecordSet{}).Name()
	RecordSetGroupKind        = schema.GroupKind{Group: Group, Kind: RecordSetKind}.String()
	RecordSetKindAPIVersion   = RecordSetKind + "." + SchemeGroupVersion.String()
	RecordSetGroupVersionKind = SchemeGroupVersion.WithKind(RecordSetKind)
)

func init() {
	SchemeBuilder.Register(&RecordSet{}, &RecordSetList{})
}
