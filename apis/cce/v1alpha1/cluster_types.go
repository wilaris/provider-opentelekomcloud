package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// ClusterParameters are the configurable fields of a CCE Cluster.
// +kubebuilder:validation:XValidation:rule="has(self.vpcId) || has(self.vpcIdRef) || has(self.vpcIdSelector)",message="one of vpcId, vpcIdRef or vpcIdSelector is required"
// +kubebuilder:validation:XValidation:rule="has(self.subnetId) || has(self.subnetIdRef) || has(self.subnetIdSelector)",message="one of subnetId, subnetIdRef or subnetIdSelector is required"
// +kubebuilder:validation:XValidation:rule="self.name == oldSelf.name",message="name is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.flavorId == oldSelf.flavorId",message="flavorId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.clusterType == oldSelf.clusterType",message="clusterType is immutable after creation"
// +kubebuilder:validation:XValidation:rule="self.containerNetworkType == oldSelf.containerNetworkType",message="containerNetworkType is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.vpcId) || self.vpcId == oldSelf.vpcId",message="vpcId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.subnetId) || self.subnetId == oldSelf.subnetId",message="subnetId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.securityGroupId) || self.securityGroupId == oldSelf.securityGroupId",message="securityGroupId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.clusterVersion) || self.clusterVersion == oldSelf.clusterVersion",message="clusterVersion is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.containerNetworkCidr) || self.containerNetworkCidr == oldSelf.containerNetworkCidr",message="containerNetworkCidr is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.authenticationMode) || self.authenticationMode == oldSelf.authenticationMode",message="authenticationMode is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.ipv6Enable) || self.ipv6Enable == oldSelf.ipv6Enable",message="ipv6Enable is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.billingMode) || self.billingMode == oldSelf.billingMode",message="billingMode is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.highwaySubnetId) || self.highwaySubnetId == oldSelf.highwaySubnetId",message="highwaySubnetId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.enableVolumeEncryption) || self.enableVolumeEncryption == oldSelf.enableVolumeEncryption",message="enableVolumeEncryption is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.eniSubnetId) || self.eniSubnetId == oldSelf.eniSubnetId",message="eniSubnetId is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.eniSubnetCidr) || self.eniSubnetCidr == oldSelf.eniSubnetCidr",message="eniSubnetCidr is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.kubeProxyMode) || self.kubeProxyMode == oldSelf.kubeProxyMode",message="kubeProxyMode is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.kubernetesSvcIpRange) || self.kubernetesSvcIpRange == oldSelf.kubernetesSvcIpRange",message="kubernetesSvcIpRange is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.multiAz) || self.multiAz == oldSelf.multiAz",message="multiAz is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.enableDeletionProtection) || self.enableDeletionProtection == oldSelf.enableDeletionProtection",message="enableDeletionProtection is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.noAddons) || self.noAddons == oldSelf.noAddons",message="noAddons is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.masters) || self.masters == oldSelf.masters",message="masters is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.authenticatingProxy) || self.authenticatingProxy == oldSelf.authenticatingProxy",message="authenticatingProxy is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.labels) || self.labels == oldSelf.labels",message="labels is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.annotations) || self.annotations == oldSelf.annotations",message="annotations is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.timezone) || self.timezone == oldSelf.timezone",message="timezone is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.extendParam) || self.extendParam == oldSelf.extendParam",message="extendParam is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.apiAccessTrustlist) || self.apiAccessTrustlist == oldSelf.apiAccessTrustlist",message="apiAccessTrustlist is immutable after creation"
// +kubebuilder:validation:XValidation:rule="has(self.eniSubnetId) == has(self.eniSubnetCidr)",message="eniSubnetId and eniSubnetCidr must be set together"
// +kubebuilder:validation:XValidation:rule="!(has(self.authenticationMode) && self.authenticationMode == 'authenticating_proxy') || has(self.authenticatingProxy)",message="authenticatingProxy must be set when authenticationMode is 'authenticating_proxy'"
// +kubebuilder:validation:XValidation:rule="!(has(self.masters) && has(self.multiAz) && self.multiAz)",message="masters and multiAz are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.customSan) || self.customSan == oldSelf.customSan",message="customSan is immutable after creation (the CCE API has no update path)"
// +kubebuilder:validation:XValidation:rule="!has(self.deleteAllStorage) || (!has(self.deleteEfs) && !has(self.deleteEvs) && !has(self.deleteObs) && !has(self.deleteSfs))",message="deleteAllStorage cannot be combined with deleteEfs, deleteEvs, deleteObs or deleteSfs"
// +kubebuilder:validation:XValidation:rule="!has(self.deleteAllNetwork) || (!has(self.deleteEni) && !has(self.deleteNet))",message="deleteAllNetwork cannot be combined with deleteEni or deleteNet"
type ClusterParameters struct {
	// Name is the cluster name.
	// Must be 4-128 characters, start with a lowercase letter, end with an
	// alphanumeric character, and contain only lowercase letters, digits and
	// hyphens.
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]{2,126}[a-z0-9]$`
	Name string `json:"name"`

	// FlavorID is the cluster flavor (e.g. cce.s1.small, cce.s2.medium).
	FlavorID string `json:"flavorId"`

	// ClusterType is the type of the cluster. Valid values per the CCE v3
	// API are "VirtualMachine", "BareMetal" and "Windows".
	// +kubebuilder:validation:Enum=VirtualMachine;BareMetal;Windows
	ClusterType string `json:"clusterType"`

	// ContainerNetworkType is the container network mode.
	// +kubebuilder:validation:Enum=overlay_l2;underlay_ipvlan;vpc-router
	ContainerNetworkType string `json:"containerNetworkType"`

	// VPCID is the ID of the VPC used by the cluster.
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

	// SubnetID is the ID of the subnet used by the cluster.
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

	// SecurityGroupID is the ID of the security group used by the cluster.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.SecurityGroup
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	SecurityGroupID *string `json:"securityGroupId,omitempty"`

	// SecurityGroupIDRef is a namespaced reference to a SecurityGroup to populate securityGroupId.
	// +optional
	SecurityGroupIDRef *xpv1.NamespacedReference `json:"securityGroupIdRef,omitempty"`

	// SecurityGroupIDSelector selects a namespaced reference to a SecurityGroup to populate securityGroupId.
	// +optional
	SecurityGroupIDSelector *xpv1.NamespacedSelector `json:"securityGroupIdSelector,omitempty"`

	// Description is the cluster description.
	// +optional
	Description *string `json:"description,omitempty"`

	// ClusterVersion is the Kubernetes version of the cluster (e.g. "v1.25").
	// +optional
	ClusterVersion *string `json:"clusterVersion,omitempty"`

	// Labels are cluster labels as key/value pairs.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are cluster annotations as key/value pairs.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Timezone is the cluster timezone (e.g. "Asia/Shanghai").
	// +optional
	Timezone *string `json:"timezone,omitempty"`

	// BillingMode is the billing mode. 0 = on-demand (default).
	// +optional
	BillingMode *int `json:"billingMode,omitempty"`

	// IPv6Enable indicates whether IPv6 is enabled for the cluster.
	// +optional
	IPv6Enable *bool `json:"ipv6Enable,omitempty"`

	// HighwaySubnetID is the ID of the high-speed network used for bare metal nodes.
	// +optional
	HighwaySubnetID *string `json:"highwaySubnetId,omitempty"`

	// ExtendParam is a map of extended parameters for the cluster.
	// +optional
	ExtendParam map[string]string `json:"extendParam,omitempty"`

	// EnableVolumeEncryption indicates whether system/data disks of master nodes are encrypted.
	// +optional
	EnableVolumeEncryption *bool `json:"enableVolumeEncryption,omitempty"`

	// ContainerNetworkCIDR is the container network CIDR block.
	// +optional
	ContainerNetworkCIDR *string `json:"containerNetworkCidr,omitempty"`

	// EniSubnetID is the ENI subnet ID (required for Turbo clusters).
	// +optional
	EniSubnetID *string `json:"eniSubnetId,omitempty"`

	// EniSubnetCIDR is the ENI subnet CIDR (required for Turbo clusters).
	// +optional
	EniSubnetCIDR *string `json:"eniSubnetCidr,omitempty"`

	// AuthenticationMode is the authentication mode for the cluster.
	// Defaults to "rbac".
	// +optional
	// +kubebuilder:validation:Enum=rbac;x509;authenticating_proxy
	AuthenticationMode *string `json:"authenticationMode,omitempty"`

	// Masters contains the advanced configuration of master nodes.
	// +optional
	// +kubebuilder:validation:MaxItems=3
	Masters []MasterSpec `json:"masters,omitempty"`

	// AuthenticatingProxy is the authenticating proxy configuration.
	// Required when authenticationMode is "authenticating_proxy".
	// +optional
	AuthenticatingProxy *AuthenticatingProxySpec `json:"authenticatingProxy,omitempty"`

	// APIAccessTrustlist is a list of CIDRs allowed to access the cluster API.
	// +optional
	APIAccessTrustlist []string `json:"apiAccessTrustlist,omitempty"`

	// KubernetesSvcIPRange is the Kubernetes service IP range.
	// +optional
	KubernetesSvcIPRange *string `json:"kubernetesSvcIpRange,omitempty"`

	// KubeProxyMode is the service forwarding mode.
	// +optional
	// +kubebuilder:validation:Enum=iptables;ipvs
	KubeProxyMode *string `json:"kubeProxyMode,omitempty"`

	// MultiAZ enables multi-AZ mode for the cluster.
	// +optional
	MultiAZ *bool `json:"multiAz,omitempty"`

	// EIP is the elastic IP address to bind to the cluster master node.
	// Set to empty string to unbind. Can be populated via eipRef/eipSelector
	// from an ElasticIP managed resource's allocated address.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.ElasticIP
	// +crossplane:generate:reference:extractor=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.ElasticIPAddress()
	// +optional
	EIP *string `json:"eip,omitempty"`

	// EIPRef is a namespaced reference to an ElasticIP to populate eip.
	// +optional
	EIPRef *xpv1.NamespacedReference `json:"eipRef,omitempty"`

	// EIPSelector selects a namespaced reference to an ElasticIP to populate eip.
	// +optional
	EIPSelector *xpv1.NamespacedSelector `json:"eipSelector,omitempty"`

	// EIPID is the FIP UUID used for bind operations. Typically populated via
	// eipIdRef/eipIdSelector referencing the same ElasticIP as EIP.
	// +crossplane:generate:reference:type=go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1.ElasticIP
	// +crossplane:generate:reference:extractor=github.com/crossplane/crossplane-runtime/v2/pkg/reference.ExternalName()
	// +optional
	EIPID *string `json:"eipId,omitempty"`

	// EIPIDRef is a namespaced reference to an ElasticIP to populate eipId.
	// +optional
	EIPIDRef *xpv1.NamespacedReference `json:"eipIdRef,omitempty"`

	// EIPIDSelector selects a namespaced reference to an ElasticIP to populate eipId.
	// +optional
	EIPIDSelector *xpv1.NamespacedSelector `json:"eipIdSelector,omitempty"`

	// ComponentConfigurations overrides component configurations (e.g. kube-apiserver).
	// +optional
	ComponentConfigurations []ComponentConfiguration `json:"componentConfigurations,omitempty"`

	// EnableDeletionProtection enables deletion protection for the cluster.
	// +optional
	EnableDeletionProtection *bool `json:"enableDeletionProtection,omitempty"`

	// CustomSAN is a list of custom Subject Alternative Names for the cluster API server certificate.
	// +optional
	CustomSAN []string `json:"customSan,omitempty"`

	// NoAddons disables installation of default addons when set to true.
	// +optional
	NoAddons *bool `json:"noAddons,omitempty"`

	// DeleteEFS controls deletion of EFS resources on cluster deletion.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteEFS *string `json:"deleteEfs,omitempty"`

	// DeleteENI controls deletion of ENI resources on cluster deletion.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteENI *string `json:"deleteEni,omitempty"`

	// DeleteEVS controls deletion of EVS resources on cluster deletion.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteEVS *string `json:"deleteEvs,omitempty"`

	// DeleteNet controls deletion of network resources on cluster deletion.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteNet *string `json:"deleteNet,omitempty"`

	// DeleteOBS controls deletion of OBS resources on cluster deletion.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteOBS *string `json:"deleteObs,omitempty"`

	// DeleteSFS controls deletion of SFS resources on cluster deletion.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteSFS *string `json:"deleteSfs,omitempty"`

	// DeleteAllStorage deletes all storage resources (EFS/EVS/OBS/SFS) on
	// cluster deletion. "try" best-efforts each delete and ignores failures.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteAllStorage *string `json:"deleteAllStorage,omitempty"`

	// DeleteAllNetwork deletes all network resources (ENI/Net) on cluster
	// deletion. "try" best-efforts each delete and ignores failures.
	// +optional
	// +kubebuilder:validation:Enum="true";"try";"false"
	DeleteAllNetwork *string `json:"deleteAllNetwork,omitempty"`
}

// MasterSpec describes the configuration of a master node.
type MasterSpec struct {
	// AvailabilityZone is the availability zone for the master node.
	AvailabilityZone string `json:"availabilityZone"`
}

// AuthenticatingProxySpec is the authenticating proxy configuration for the cluster.
type AuthenticatingProxySpec struct {
	// CA is the CA certificate for the authenticating proxy. Raw PEM is
	// accepted and will be base64-encoded on the wire.
	CA string `json:"ca"`

	// Cert is the client certificate for the authenticating proxy. Raw PEM is
	// accepted and will be base64-encoded on the wire.
	Cert string `json:"cert"`

	// PrivateKey is the private key for the authenticating proxy. Raw PEM is
	// accepted and will be base64-encoded on the wire.
	PrivateKey string `json:"privateKey"`
}

// ComponentConfiguration describes a component configuration override.
type ComponentConfiguration struct {
	// Name is the component name (e.g. "kube-apiserver").
	Name string `json:"name"`

	// Configurations is the list of configuration items for this component.
	Configurations []ConfigurationItem `json:"configurations"`
}

// ConfigurationItem is a single configuration parameter.
type ConfigurationItem struct {
	// Name is the configuration parameter name.
	Name string `json:"name"`

	// Value is the configuration parameter value.
	Value string `json:"value"`
}

// ClusterObservation are the observable fields of a CCE Cluster.
type ClusterObservation struct {
	// ID is the cluster ID.
	ID string `json:"id,omitempty"`

	// Status is the cluster lifecycle status (e.g. "Available", "Creating").
	Status string `json:"status,omitempty"`

	// InternalEndpoint is the internal API endpoint of the cluster.
	InternalEndpoint string `json:"internalEndpoint,omitempty"`

	// ExternalEndpoint is the external API endpoint of the cluster.
	ExternalEndpoint string `json:"externalEndpoint,omitempty"`

	// ExternalOTCEndpoint is the OTC-specific external API endpoint.
	ExternalOTCEndpoint string `json:"externalOtcEndpoint,omitempty"`

	// SecurityGroupControl is the security group ID of the control plane.
	SecurityGroupControl string `json:"securityGroupControl,omitempty"`

	// SecurityGroupNode is the security group ID of the worker nodes.
	SecurityGroupNode string `json:"securityGroupNode,omitempty"`

	// SupportIstio indicates whether the cluster supports Istio integration.
	SupportIstio *bool `json:"supportIstio,omitempty"`
}

// A ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              ClusterParameters `json:"forProvider"`
}

// A ClusterStatus represents the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Cluster is a managed resource that represents an OpenTelekomCloud CCE Cluster.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="FLAVOR",type="string",JSONPath=".spec.forProvider.flavorId"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.atProvider.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// Cluster type metadata.
var (
	ClusterKind             = reflect.TypeOf(Cluster{}).Name()
	ClusterGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterKind}.String()
	ClusterKindAPIVersion   = ClusterKind + "." + SchemeGroupVersion.String()
	ClusterGroupVersionKind = SchemeGroupVersion.WithKind(ClusterKind)
)

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
