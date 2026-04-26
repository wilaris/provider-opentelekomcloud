package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SecurityGroupParameters are the configurable fields of a SecurityGroup.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.enterpriseProjectId) || self.enterpriseProjectId == oldSelf.enterpriseProjectId",message="enterpriseProjectId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.tags) || self.tags == oldSelf.tags",message="tags are immutable after creation"
type SecurityGroupParameters struct {
	// Name is the security group name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`

	// Description is the security group description.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// EnterpriseProjectID is the enterprise project ID to associate with the security group.
	// This field is immutable after creation.
	// +optional
	// +kubebuilder:validation:MaxLength=36
	EnterpriseProjectID *string `json:"enterpriseProjectId,omitempty"`

	// Tags are security group tags. Tags are set at creation time only and cannot be updated.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// SecurityGroupObservation are the observable fields of a SecurityGroup.
type SecurityGroupObservation struct {
	// ID is the OTC security group ID.
	ID string `json:"id,omitempty"`

	// Name is the observed security group name.
	Name string `json:"name,omitempty"`

	// Description is the observed security group description.
	Description string `json:"description,omitempty"`

	// ProjectID is the project to which the security group belongs.
	ProjectID string `json:"projectId,omitempty"`

	// EnterpriseProjectID is the observed enterprise project ID.
	EnterpriseProjectID string `json:"enterpriseProjectId,omitempty"`

	// Tags are observed security group tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A SecurityGroupSpec defines the desired state of a SecurityGroup.
type SecurityGroupSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SecurityGroupParameters `json:"forProvider"`
}

// A SecurityGroupStatus represents the observed state of a SecurityGroup.
type SecurityGroupStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SecurityGroupObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A SecurityGroup is a managed resource that represents an OpenTelekomCloud VPC Security Group.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type SecurityGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupSpec   `json:"spec"`
	Status SecurityGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityGroupList contains a list of SecurityGroup
type SecurityGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroup `json:"items"`
}

// SecurityGroup type metadata.
var (
	SecurityGroupKind             = reflect.TypeOf(SecurityGroup{}).Name()
	SecurityGroupGroupKind        = schema.GroupKind{Group: Group, Kind: SecurityGroupKind}.String()
	SecurityGroupKindAPIVersion   = SecurityGroupKind + "." + SchemeGroupVersion.String()
	SecurityGroupGroupVersionKind = SchemeGroupVersion.WithKind(SecurityGroupKind)
)

func init() {
	SchemeBuilder.Register(&SecurityGroup{}, &SecurityGroupList{})
}
