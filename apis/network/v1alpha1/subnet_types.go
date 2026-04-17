package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SubnetParameters are the configurable fields of a Subnet.
// +kubebuilder:validation:XValidation:rule="has(self.vpcId) || has(self.vpcIdRef) || has(self.vpcIdSelector)",message="one of vpcId, vpcIdRef or vpcIdSelector is required"
// +kubebuilder:validation:XValidation:rule="self.cidr == oldSelf.cidr",message="cidr is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.gatewayIp == oldSelf.gatewayIp",message="gatewayIp is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.ipv6Enable == null || self.ipv6Enable == oldSelf.ipv6Enable",message="ipv6Enable is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.availabilityZone == null || self.availabilityZone == oldSelf.availabilityZone",message="availabilityZone is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.vpcId == null || self.vpcId == oldSelf.vpcId",message="vpcId is immutable after creation"
type SubnetParameters struct {
	// Name is the subnet name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`

	// Description is the subnet description.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// CIDR is the subnet CIDR block.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=18
	CIDR string `json:"cidr"`

	// DNSList is the subnet DNS server list.
	// +optional
	DNSList []string `json:"dnsList,omitempty"`

	// GatewayIP is the subnet gateway IPv4 address.
	// +kubebuilder:validation:MinLength=7
	// +kubebuilder:validation:MaxLength=15
	GatewayIP string `json:"gatewayIp"`

	// DHCPEnable indicates whether DHCP is enabled for the subnet.
	// Defaults to true when omitted.
	// +optional
	DHCPEnable *bool `json:"dhcpEnable,omitempty"`

	// IPv6Enable indicates whether IPv6 is enabled for the subnet.
	// +optional
	IPv6Enable *bool `json:"ipv6Enable,omitempty"`

	// PrimaryDNS is the primary DNS server IPv4 address.
	// +optional
	PrimaryDNS *string `json:"primaryDns,omitempty"`

	// SecondaryDNS is the secondary DNS server IPv4 address.
	// +optional
	SecondaryDNS *string `json:"secondaryDns,omitempty"`

	// AvailabilityZone is the availability zone to which the subnet belongs.
	// +optional
	AvailabilityZone *string `json:"availabilityZone,omitempty"`

	// VPCID is the VPC ID to which the subnet belongs.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.VPC
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	VPCID *string `json:"vpcId,omitempty"`

	// VPCIDRef is a namespaced reference to a VPC.
	// +optional
	VPCIDRef *xpv1.NamespacedReference `json:"vpcIdRef,omitempty"`

	// VPCIDSelector selects a namespaced reference to a VPC.
	// +optional
	VPCIDSelector *xpv1.NamespacedSelector `json:"vpcIdSelector,omitempty"`

	// Tags are subnet tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`

	// NTPAddresses is the subnet NTP server address configuration.
	// +optional
	NTPAddresses *string `json:"ntpAddresses,omitempty"`
}

// SubnetObservation are the observable fields of a Subnet.
type SubnetObservation struct {
	// ID is the OTC subnet ID.
	ID string `json:"id,omitempty"`

	// Name is the observed subnet name.
	Name string `json:"name,omitempty"`

	// Description is the observed subnet description.
	Description string `json:"description,omitempty"`

	// CIDR is the observed subnet CIDR.
	CIDR string `json:"cidr,omitempty"`

	// DNSList is the observed subnet DNS list.
	DNSList []string `json:"dnsList,omitempty"`

	// GatewayIP is the observed subnet gateway.
	GatewayIP string `json:"gatewayIp,omitempty"`

	// DHCPEnable is the observed subnet DHCP state.
	DHCPEnable bool `json:"dhcpEnable,omitempty"`

	// IPv6Enable is the observed subnet IPv6 state.
	IPv6Enable bool `json:"ipv6Enable,omitempty"`

	// CIDRIPv6 is the observed IPv6 CIDR.
	CIDRIPv6 string `json:"cidrIpv6,omitempty"`

	// GatewayIPv6 is the observed IPv6 gateway.
	GatewayIPv6 string `json:"gatewayIpv6,omitempty"`

	// PrimaryDNS is the observed primary DNS server.
	PrimaryDNS string `json:"primaryDns,omitempty"`

	// SecondaryDNS is the observed secondary DNS server.
	SecondaryDNS string `json:"secondaryDns,omitempty"`

	// AvailabilityZone is the observed availability zone.
	AvailabilityZone string `json:"availabilityZone,omitempty"`

	// VPCID is the observed VPC ID.
	VPCID string `json:"vpcId,omitempty"`

	// SubnetID is the observed OpenStack subnet ID.
	SubnetID string `json:"subnetId,omitempty"`

	// NetworkID is the observed OpenStack network ID.
	NetworkID string `json:"networkId,omitempty"`

	// NTPAddresses is the observed NTP server address configuration.
	NTPAddresses string `json:"ntpAddresses,omitempty"`

	// Status is the observed subnet lifecycle status.
	Status string `json:"status,omitempty"`

	// Tags are observed subnet tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A SubnetSpec defines the desired state of a Subnet.
type SubnetSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SubnetParameters `json:"forProvider"`
}

// A SubnetStatus represents the observed state of a Subnet.
type SubnetStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SubnetObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Subnet is a managed resource that represents an OpenTelekomCloud VPC subnet.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="CIDR",type="string",JSONPath=".spec.forProvider.cidr"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type Subnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec"`
	Status SubnetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubnetList contains a list of Subnet
type SubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnet `json:"items"`
}

// Subnet type metadata.
var (
	SubnetKind             = reflect.TypeOf(Subnet{}).Name()
	SubnetGroupKind        = schema.GroupKind{Group: Group, Kind: SubnetKind}.String()
	SubnetKindAPIVersion   = SubnetKind + "." + SchemeGroupVersion.String()
	SubnetGroupVersionKind = SchemeGroupVersion.WithKind(SubnetKind)
)

func init() {
	SchemeBuilder.Register(&Subnet{}, &SubnetList{})
}
