package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SNATRuleParameters are the configurable fields of a SNATRule.
// All fields are immutable after creation — the OTC API does not support updates.
// +kubebuilder:validation:XValidation:rule="has(self.natGatewayId) || has(self.natGatewayIdRef) || has(self.natGatewayIdSelector)",message="one of natGatewayId, natGatewayIdRef or natGatewayIdSelector is required"
// +kubebuilder:validation:XValidation:rule="has(self.elasticIpId) || has(self.elasticIpIdRef) || has(self.elasticIpIdSelector)",message="one of elasticIpId, elasticIpIdRef or elasticIpIdSelector is required"
// +kubebuilder:validation:XValidation:rule="oldSelf.natGatewayId == null || self.natGatewayId == oldSelf.natGatewayId",message="natGatewayId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.elasticIpId == null || self.elasticIpId == oldSelf.elasticIpId",message="elasticIpId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.subnetId == null || self.subnetId == oldSelf.subnetId",message="subnetId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.cidr == null || self.cidr == oldSelf.cidr",message="cidr is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.sourceType == null || self.sourceType == oldSelf.sourceType",message="sourceType is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.description == null || self.description == oldSelf.description",message="description is immutable after creation"
type SNATRuleParameters struct {
	// NatGatewayID is the ID of the NAT gateway this SNAT rule belongs to.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.NATGateway
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	NatGatewayID *string `json:"natGatewayId,omitempty"`

	// NatGatewayIDRef is a namespaced reference to a NATGateway to populate natGatewayId.
	// +optional
	NatGatewayIDRef *xpv1.NamespacedReference `json:"natGatewayIdRef,omitempty"`

	// NatGatewayIDSelector selects a namespaced reference to a NATGateway to populate natGatewayId.
	// +optional
	NatGatewayIDSelector *xpv1.NamespacedSelector `json:"natGatewayIdSelector,omitempty"`

	// ElasticIPID is the ID of the Elastic IP for this SNAT rule.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.ElasticIP
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	ElasticIPID *string `json:"elasticIpId,omitempty"`

	// ElasticIPIDRef is a namespaced reference to an ElasticIP to populate elasticIpId.
	// +optional
	ElasticIPIDRef *xpv1.NamespacedReference `json:"elasticIpIdRef,omitempty"`

	// ElasticIPIDSelector selects a namespaced reference to an ElasticIP to populate elasticIpId.
	// +optional
	ElasticIPIDSelector *xpv1.NamespacedSelector `json:"elasticIpIdSelector,omitempty"`

	// SubnetID is the ID of the subnet for this SNAT rule.
	// Mutually exclusive with cidr.
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

	// CIDR is the CIDR block for this SNAT rule.
	// If sourceType is VPC, CIDR must be a subset of the VPC subnet CIDR.
	// Mutually exclusive with subnetId.
	// +optional
	CIDR *string `json:"cidr,omitempty"`

	// SourceType is the SNAT rule source type.
	// VPC means either subnetId or cidr can be specified.
	// +optional
	// +kubebuilder:validation:Enum="VPC"
	SourceType *string `json:"sourceType,omitempty"`

	// Description provides supplementary information about the SNAT rule.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`
}

// SNATRuleObservation are the observable fields of a SNATRule.
type SNATRuleObservation struct {
	// ID is the OTC SNAT rule ID.
	ID string `json:"id,omitempty"`

	// NatGatewayID is the observed NAT gateway ID.
	NatGatewayID string `json:"natGatewayId,omitempty"`

	// SubnetID is the observed subnet ID.
	SubnetID string `json:"subnetId,omitempty"`

	// ElasticIPID is the observed Elastic IP ID.
	ElasticIPID string `json:"elasticIpId,omitempty"`

	// ElasticIPAddress is the observed Elastic IP address.
	ElasticIPAddress string `json:"elasticIpAddress,omitempty"`

	// CIDR is the observed CIDR block.
	CIDR string `json:"cidr,omitempty"`

	// SourceType is the observed source type (0 = VPC, 1 = Direct Connect).
	SourceType int `json:"sourceType,omitempty"`

	// Status is the observed SNAT rule status.
	Status string `json:"status,omitempty"`

	// AdminStateUp is the observed administrative state.
	AdminStateUp bool `json:"adminStateUp,omitempty"`

	// TenantID is the project to which the rule belongs.
	TenantID string `json:"tenantId,omitempty"`

	// Description is the observed description.
	Description string `json:"description,omitempty"`
}

// A SNATRuleSpec defines the desired state of a SNATRule.
type SNATRuleSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SNATRuleParameters `json:"forProvider"`
}

// A SNATRuleStatus represents the observed state of a SNATRule.
type SNATRuleStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SNATRuleObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A SNATRule is a managed resource that represents an OpenTelekomCloud NAT SNAT Rule.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type SNATRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SNATRuleSpec   `json:"spec"`
	Status SNATRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SNATRuleList contains a list of SNATRule
type SNATRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SNATRule `json:"items"`
}

// SNATRule type metadata.
var (
	SNATRuleKind             = reflect.TypeOf(SNATRule{}).Name()
	SNATRuleGroupKind        = schema.GroupKind{Group: Group, Kind: SNATRuleKind}.String()
	SNATRuleKindAPIVersion   = SNATRuleKind + "." + SchemeGroupVersion.String()
	SNATRuleGroupVersionKind = SchemeGroupVersion.WithKind(SNATRuleKind)
)

func init() {
	SchemeBuilder.Register(&SNATRule{}, &SNATRuleList{})
}
