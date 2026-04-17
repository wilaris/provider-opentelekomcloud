package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// ElasticIPParameters are the configurable fields of an ElasticIP.
// +kubebuilder:validation:XValidation:rule="self.publicIp.type == oldSelf.publicIp.type",message="publicIp.type is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.publicIp.ipAddress == null || self.publicIp.ipAddress == oldSelf.publicIp.ipAddress",message="publicIp.ipAddress is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.publicIp.name == null || self.publicIp.name == oldSelf.publicIp.name",message="publicIp.name is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.bandwidth.shareType == oldSelf.bandwidth.shareType",message="bandwidth.shareType is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf.bandwidth.chargeMode == null || self.bandwidth.chargeMode == oldSelf.bandwidth.chargeMode",message="bandwidth.chargeMode is immutable after creation"
type ElasticIPParameters struct {
	// PublicIP defines the public IP configuration.
	PublicIP ElasticIPPublicIPParameters `json:"publicIp"`

	// Bandwidth defines the bandwidth configuration.
	Bandwidth ElasticIPBandwidthParameters `json:"bandwidth"`

	// Tags are ElasticIP tags.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// ElasticIPPublicIPParameters defines the public IP configuration of an ElasticIP.
type ElasticIPPublicIPParameters struct {
	// Type is the public IP type.
	// BGP corresponds to the OTC "5_bgp" dynamic BGP type.
	// +kubebuilder:validation:Enum="BGP"
	Type string `json:"type"`

	// IPAddress is the specific IP address to request.
	// +optional
	IPAddress *string `json:"ipAddress,omitempty"`

	// Name is the alias of the public IP.
	// +optional
	// +kubebuilder:validation:MaxLength=64
	Name *string `json:"name,omitempty"`

	// PortID is the port to bind the EIP to.
	// +optional
	PortID *string `json:"portId,omitempty"`
}

// ElasticIPBandwidthParameters defines the bandwidth configuration of an ElasticIP.
type ElasticIPBandwidthParameters struct {
	// Name is the bandwidth name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	Name string `json:"name"`

	// Size is the bandwidth size in Mbps.
	// +kubebuilder:validation:Minimum=1
	Size int `json:"size"`

	// ShareType is the bandwidth sharing type.
	// Dedicated allocates bandwidth exclusively for this EIP.
	// +kubebuilder:validation:Enum="Dedicated"
	ShareType string `json:"shareType"`

	// ChargeMode is the bandwidth charge mode.
	// +optional
	ChargeMode *string `json:"chargeMode,omitempty"`
}

// ElasticIPObservation are the observable fields of an ElasticIP.
type ElasticIPObservation struct {
	// ID is the OTC EIP ID.
	ID string `json:"id,omitempty"`

	// Status is the observed EIP status.
	Status string `json:"status,omitempty"`

	// PublicIPAddress is the allocated public IP address.
	PublicIPAddress string `json:"publicIpAddress,omitempty"`

	// PrivateIPAddress is the private IP address if bound to a port.
	PrivateIPAddress string `json:"privateIpAddress,omitempty"`

	// PortID is the currently bound port ID.
	PortID string `json:"portId,omitempty"`

	// BandwidthID is the associated bandwidth ID.
	BandwidthID string `json:"bandwidthId,omitempty"`

	// BandwidthName is the observed bandwidth name.
	BandwidthName string `json:"bandwidthName,omitempty"`

	// BandwidthSize is the observed bandwidth size in Mbps.
	BandwidthSize int `json:"bandwidthSize,omitempty"`

	// BandwidthShareType is the observed bandwidth sharing type.
	BandwidthShareType string `json:"bandwidthShareType,omitempty"`

	// BandwidthChargeMode is the observed bandwidth charge mode.
	BandwidthChargeMode string `json:"bandwidthChargeMode,omitempty"`

	// IPVersion is the observed IP version (4 or 6).
	IPVersion int `json:"ipVersion,omitempty"`

	// Tags are observed EIP tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// An ElasticIPSpec defines the desired state of an ElasticIP.
type ElasticIPSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ElasticIPParameters `json:"forProvider"`
}

// An ElasticIPStatus represents the observed state of an ElasticIP.
type ElasticIPStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ElasticIPObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An ElasticIP is a managed resource that represents an OpenTelekomCloud VPC Elastic IP.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="IP",type="string",JSONPath=".status.atProvider.publicIpAddress"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type ElasticIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ElasticIPSpec   `json:"spec"`
	Status ElasticIPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ElasticIPList contains a list of ElasticIP
type ElasticIPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElasticIP `json:"items"`
}

// ElasticIP type metadata.
var (
	ElasticIPKind             = reflect.TypeOf(ElasticIP{}).Name()
	ElasticIPGroupKind        = schema.GroupKind{Group: Group, Kind: ElasticIPKind}.String()
	ElasticIPKindAPIVersion   = ElasticIPKind + "." + SchemeGroupVersion.String()
	ElasticIPGroupVersionKind = SchemeGroupVersion.WithKind(ElasticIPKind)
)

func init() {
	SchemeBuilder.Register(&ElasticIP{}, &ElasticIPList{})
}
