package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SecurityGroupRuleParameters are the configurable fields of a SecurityGroupRule.
// All fields are immutable after creation — the OTC API does not support updates.
// +kubebuilder:validation:XValidation:rule="has(self.securityGroupId) || has(self.securityGroupIdRef) || has(self.securityGroupIdSelector)",message="one of securityGroupId, securityGroupIdRef or securityGroupIdSelector is required"
// +kubebuilder:validation:XValidation:rule="self.direction == oldSelf.direction",message="direction is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.securityGroupId == null || self.securityGroupId == oldSelf.securityGroupId",message="securityGroupId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.description == null || self.description == oldSelf.description",message="description is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.etherType == null || self.etherType == oldSelf.etherType",message="etherType is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.protocol == null || self.protocol == oldSelf.protocol",message="protocol is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.multiPort == null || self.multiPort == oldSelf.multiPort",message="multiPort is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.remoteGroupId == null || self.remoteGroupId == oldSelf.remoteGroupId",message="remoteGroupId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.remoteIpPrefix == null || self.remoteIpPrefix == oldSelf.remoteIpPrefix",message="remoteIpPrefix is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.action == null || self.action == oldSelf.action",message="action is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.priority == null || self.priority == oldSelf.priority",message="priority is immutable after creation"
type SecurityGroupRuleParameters struct {
	// SecurityGroupID is the ID of the security group to which this rule belongs.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.SecurityGroup
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	SecurityGroupID *string `json:"securityGroupId,omitempty"`

	// SecurityGroupIDRef is a namespaced reference to a SecurityGroup.
	// +optional
	SecurityGroupIDRef *xpv1.NamespacedReference `json:"securityGroupIdRef,omitempty"`

	// SecurityGroupIDSelector selects a namespaced reference to a SecurityGroup.
	// +optional
	SecurityGroupIDSelector *xpv1.NamespacedSelector `json:"securityGroupIdSelector,omitempty"`

	// Direction is the traffic direction of the security group rule.
	// +kubebuilder:validation:Enum="ingress";"egress"
	Direction string `json:"direction"`

	// Description provides supplementary information about the security group rule.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// EtherType is the IP version. Defaults to IPv4 when omitted.
	// +optional
	// +kubebuilder:validation:Enum="IPv4";"IPv6"
	EtherType *string `json:"etherType,omitempty"`

	// Protocol is the protocol type.
	// The value can be icmp, tcp, udp, icmpv6, or an IP protocol number.
	// +optional
	Protocol *string `json:"protocol,omitempty"`

	// MultiPort is the port or port range.
	// Can be a single port (80), a port range (1-30), or comma-separated ports (22,3389,80).
	// +optional
	MultiPort *string `json:"multiPort,omitempty"`

	// RemoteGroupID is the ID of the remote security group.
	// Mutually exclusive with remoteIpPrefix.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.SecurityGroup
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	RemoteGroupID *string `json:"remoteGroupId,omitempty"`

	// RemoteGroupIDRef is a namespaced reference to a SecurityGroup for remoteGroupId.
	// +optional
	RemoteGroupIDRef *xpv1.NamespacedReference `json:"remoteGroupIdRef,omitempty"`

	// RemoteGroupIDSelector selects a namespaced reference to a SecurityGroup for remoteGroupId.
	// +optional
	RemoteGroupIDSelector *xpv1.NamespacedSelector `json:"remoteGroupIdSelector,omitempty"`

	// RemoteIPPrefix is the remote IP address or CIDR block.
	// Mutually exclusive with remoteGroupId.
	// +optional
	RemoteIPPrefix *string `json:"remoteIpPrefix,omitempty"`

	// Action is the security group rule action.
	// +optional
	// +kubebuilder:validation:Enum="allow";"deny"
	Action *string `json:"action,omitempty"`

	// Priority is the rule priority. The value is from 1 to 100, where 1 is the highest priority.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Priority *int `json:"priority,omitempty"`
}

// SecurityGroupRuleObservation are the observable fields of a SecurityGroupRule.
type SecurityGroupRuleObservation struct {
	// ID is the OTC security group rule ID.
	ID string `json:"id,omitempty"`

	// SecurityGroupID is the observed security group ID.
	SecurityGroupID string `json:"securityGroupId,omitempty"`

	// Direction is the observed traffic direction.
	Direction string `json:"direction,omitempty"`

	// Description is the observed description.
	Description string `json:"description,omitempty"`

	// EtherType is the observed IP version.
	EtherType string `json:"etherType,omitempty"`

	// Protocol is the observed protocol type.
	Protocol string `json:"protocol,omitempty"`

	// MultiPort is the observed port or port range.
	MultiPort string `json:"multiPort,omitempty"`

	// RemoteGroupID is the observed remote security group ID.
	RemoteGroupID string `json:"remoteGroupId,omitempty"`

	// RemoteIPPrefix is the observed remote IP address or CIDR block.
	RemoteIPPrefix string `json:"remoteIpPrefix,omitempty"`

	// RemoteAddressGroupID is the observed remote address group ID.
	RemoteAddressGroupID string `json:"remoteAddressGroupId,omitempty"`

	// Action is the observed rule action.
	Action string `json:"action,omitempty"`

	// Priority is the observed rule priority.
	Priority int `json:"priority,omitempty"`

	// ProjectID is the project to which the rule belongs.
	ProjectID string `json:"projectId,omitempty"`
}

// A SecurityGroupRuleSpec defines the desired state of a SecurityGroupRule.
type SecurityGroupRuleSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SecurityGroupRuleParameters `json:"forProvider"`
}

// A SecurityGroupRuleStatus represents the observed state of a SecurityGroupRule.
type SecurityGroupRuleStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SecurityGroupRuleObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A SecurityGroupRule is a managed resource that represents an OpenTelekomCloud VPC Security Group Rule.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="DIRECTION",type="string",JSONPath=".spec.forProvider.direction"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type SecurityGroupRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupRuleSpec   `json:"spec"`
	Status SecurityGroupRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityGroupRuleList contains a list of SecurityGroupRule
type SecurityGroupRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroupRule `json:"items"`
}

// SecurityGroupRule type metadata.
var (
	SecurityGroupRuleKind             = reflect.TypeOf(SecurityGroupRule{}).Name()
	SecurityGroupRuleGroupKind        = schema.GroupKind{Group: Group, Kind: SecurityGroupRuleKind}.String()
	SecurityGroupRuleKindAPIVersion   = SecurityGroupRuleKind + "." + SchemeGroupVersion.String()
	SecurityGroupRuleGroupVersionKind = SchemeGroupVersion.WithKind(SecurityGroupRuleKind)
)

func init() {
	SchemeBuilder.Register(&SecurityGroupRule{}, &SecurityGroupRuleList{})
}
