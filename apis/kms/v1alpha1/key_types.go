package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// KeyParameters are the configurable fields of a KMS Customer Master Key (CMK).
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.realm) || (has(self.realm) && self.realm == oldSelf.realm)",message="realm is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.keyUsage) || (has(self.keyUsage) && self.keyUsage == oldSelf.keyUsage)",message="keyUsage is immutable after creation"
// +kubebuilder:validation:XValidation:rule="!has(self.rotationInterval) || (has(self.rotationEnabled) && self.rotationEnabled == true)",message="rotationInterval can only be set when rotationEnabled is true"
type KeyParameters struct {
	// KeyAlias is the human-readable alias of the CMK.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	KeyAlias string `json:"keyAlias"`

	// Description is the CMK description.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Description *string `json:"description,omitempty"`

	// Realm is the OTC region/realm in which the CMK material lives.
	// Optional and immutable after creation; the API may default and report the value back.
	// +optional
	Realm *string `json:"realm,omitempty"`

	// KeyUsage is the cryptographic purpose of the CMK. Defaults to "EncryptAndDecrypt".
	// This is an API create-time option and is intentionally immutable after creation.
	// +optional
	// +kubebuilder:validation:Enum=EncryptAndDecrypt;SignAndVerify
	// +kubebuilder:default=EncryptAndDecrypt
	KeyUsage *string `json:"keyUsage,omitempty"`

	// Enabled is the desired enabled state of the CMK. Defaults to true.
	// Toggling this field issues an EnableKey or DisableKey call on the next reconcile.
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// PendingDays is the number of days the CMK stays in PendingDeletion before
	// being permanently destroyed. Used only at delete time. Range 7..1096; default "7".
	// +optional
	// +kubebuilder:default="7"
	// +kubebuilder:validation:Pattern=`^([7-9]|[1-9][0-9]|[1-9][0-9]{2}|10[0-8][0-9]|109[0-6])$`
	PendingDays *string `json:"pendingDays,omitempty"`

	// RotationEnabled specifies whether key rotation is enabled for normal KMS-origin CMKs.
	// Imported/external key material does not support rotation.
	// +optional
	RotationEnabled *bool `json:"rotationEnabled,omitempty"`

	// RotationInterval is the interval (in days) at which the key is rotated.
	// It is valid only when RotationEnabled is true.
	// +optional
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:validation:Maximum=365
	RotationInterval *int `json:"rotationInterval,omitempty"`

	// Tags are CMK tags managed via the OTC tag service.
	// +optional
	// +kubebuilder:validation:MaxProperties=20
	Tags map[string]string `json:"tags,omitempty"`
}

// KeyObservation are the observable fields of a KMS CMK.
//
// Any key created through this controller is a user-managed KMS-origin CMK. A
// remotely PendingDeletion key is treated as deleted.
type KeyObservation struct {
	// KeyID is the OTC-assigned CMK identifier.
	KeyID string `json:"keyId,omitempty"`

	// KeyAlias is the observed CMK alias.
	KeyAlias string `json:"keyAlias,omitempty"`

	// Description is the observed CMK description.
	Description string `json:"description,omitempty"`

	// Realm is the observed realm.
	Realm string `json:"realm,omitempty"`

	// KeyUsage is the observed cryptographic purpose.
	// Disabled: gophertelekomcloud's keys.Get response does not expose key_usage today.
	// TODO(): Re-enable once the SDK surfaces it on Key responses.
	// KeyUsage string `json:"keyUsage,omitempty"`

	// KeyState reflects the OTC CMK lifecycle state.
	// One of:
	// - "WaitingForEnable"
	// - "Enabled"
	// - "Disabled"
	// - "PendingDeletion"
	// - "WaitingForImport"
	// Unknown values returned by future OTC API revisions pass through verbatim.
	KeyState string `json:"keyState,omitempty"`

	// DomainID is the user domain that owns this CMK.
	DomainID string `json:"domainId,omitempty"`

	// ScheduledDeletionDate is populated once a key is scheduled for deletion.
	ScheduledDeletionDate string `json:"scheduledDeletionDate,omitempty"`

	// ExpirationTime is set for keys with imported material that have an expiration.
	ExpirationTime string `json:"expirationTime,omitempty"`

	// RotationEnabled is the observed key rotation setting.
	RotationEnabled *bool `json:"rotationEnabled,omitempty"`

	// RotationInterval is the observed key rotation interval in days.
	RotationInterval *int `json:"rotationInterval,omitempty"`

	// Rotations is the observed number of times the key has been rotated.
	Rotations int `json:"rotations,omitempty"`

	// Tags are observed CMK tags.
	Tags map[string]string `json:"tags,omitempty"`
}

// A KeySpec defines the desired state of a Key.
type KeySpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              KeyParameters `json:"forProvider"`
}

// A KeyStatus represents the observed state of a Key.
type KeyStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          KeyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Key is a managed resource that represents an OpenTelekomCloud KMS Customer Master Key.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="ALIAS",type="string",JSONPath=".spec.forProvider.keyAlias"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.keyState"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,opentelekomcloud}
type Key struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KeySpec   `json:"spec"`
	Status KeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KeyList contains a list of Key.
type KeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Key `json:"items"`
}

// Key type metadata.
var (
	KeyKind             = reflect.TypeOf(Key{}).Name()
	KeyGroupKind        = schema.GroupKind{Group: Group, Kind: KeyKind}.String()
	KeyKindAPIVersion   = KeyKind + "." + SchemeGroupVersion.String()
	KeyGroupVersionKind = SchemeGroupVersion.WithKind(KeyKind)
)

func init() {
	SchemeBuilder.Register(&Key{}, &KeyList{})
}
