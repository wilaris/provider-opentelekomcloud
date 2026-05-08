package kmskey

import (
	"context"
	"maps"
	"net/http"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/kms/v1/keys"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kmsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/kms/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestCreateSkipsWhenExternalNameSet(t *testing.T) {
	cr := &kmsv1alpha1.Key{}
	meta.SetExternalName(cr, "existing-key-id")

	got, err := (&external{}).Create(context.Background(), cr)
	if err != nil {
		t.Fatalf("Create() returned error for existing external-name: %v", err)
	}
	if len(got.ConnectionDetails) != 0 {
		t.Errorf("expected no connection details, got %v", got.ConnectionDetails)
	}
}

func TestMapKeyStateFromAPI(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "enabled", raw: "2", want: keyStateEnabled},
		{name: "pending deletion", raw: "4", want: keyStatePendingDeletion},
		{name: "unknown passes through", raw: "custom", want: "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapKeyStateFromAPI(tt.raw); got != tt.want {
				t.Errorf("mapKeyStateFromAPI(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestIsRotationManaged(t *testing.T) {
	tests := []struct {
		name string
		spec kmsv1alpha1.KeyParameters
		want bool
	}{
		{
			name: "unmanaged rotation",
			spec: kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			want: false,
		},
		{
			name: "rotation disabled is managed",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:        "alias",
				RotationEnabled: pointer.To(false),
			},
			want: true,
		},
		{
			name: "rotation enabled is managed",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:        "alias",
				RotationEnabled: pointer.To(true),
			},
			want: true,
		},
		{
			name: "rotation interval is managed",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:         "alias",
				RotationEnabled:  pointer.To(true),
				RotationInterval: pointer.To(90),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRotationManaged(tt.spec); got != tt.want {
				t.Errorf("isRotationManaged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildKeyCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec kmsv1alpha1.KeyParameters
		want keys.CreateOpts
	}{
		{
			name: "minimal alias-only",
			spec: kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			want: keys.CreateOpts{KeyAlias: "alias"},
		},
		{
			name: "all optionals set translates KeyUsage to API form",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:    "alias",
				Description: pointer.To("my desc"),
				Realm:       pointer.To("eu-de"),
				KeyUsage:    pointer.To("EncryptAndDecrypt"),
			},
			want: keys.CreateOpts{
				KeyAlias:       "alias",
				KeyDescription: "my desc",
				Realm:          "eu-de",
				KeyUsage:       "Encrypt_Decrypt",
			},
		},
		{
			name: "SignAndVerify translates to Sign_Verify",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				KeyUsage: pointer.To("SignAndVerify"),
			},
			want: keys.CreateOpts{
				KeyAlias: "alias",
				KeyUsage: "Sign_Verify",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKeyCreateOpts(tt.spec)
			if got != tt.want {
				t.Errorf("buildKeyCreateOpts() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateImmutableKeyFields(t *testing.T) {
	tests := []struct {
		name     string
		spec     kmsv1alpha1.KeyParameters
		observed *keys.Key
		wantErr  bool
	}{
		{
			name:     "all unset is ok",
			spec:     kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			observed: &keys.Key{Realm: "eu-de"},
			wantErr:  false,
		},
		{
			name: "matching realm is ok",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				Realm:    pointer.To("eu-de"),
			},
			observed: &keys.Key{Realm: "eu-de"},
			wantErr:  false,
		},
		{
			name: "realm changed",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				Realm:    pointer.To("eu-nl"),
			},
			observed: &keys.Key{Realm: "eu-de"},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImmutableKeyFields(tt.spec, tt.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImmutableKeyFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsKeyUpToDate(t *testing.T) {
	tests := []struct {
		name         string
		spec         kmsv1alpha1.KeyParameters
		observed     *keys.Key
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date with all fields",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:    "alias",
				Description: pointer.To("desc"),
				Enabled:     pointer.To(true),
				Tags:        map[string]string{"env": "dev"},
			},
			observed: &keys.Key{
				KeyAlias:       "alias",
				KeyDescription: "desc",
				KeyState:       keyStateEnabled,
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "nil optional fields do not trigger drift",
			spec: kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			observed: &keys.Key{
				KeyAlias:       "alias",
				KeyDescription: "auto",
				KeyState:       keyStateEnabled,
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name:     "alias mismatch",
			spec:     kmsv1alpha1.KeyParameters{KeyAlias: "new"},
			observed: &keys.Key{KeyAlias: "old", KeyState: keyStateEnabled},
			want:     false,
		},
		{
			name: "description mismatch",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:    "alias",
				Description: pointer.To("new"),
			},
			observed: &keys.Key{
				KeyAlias:       "alias",
				KeyDescription: "old",
				KeyState:       keyStateEnabled,
			},
			want: false,
		},
		{
			name: "enabled drift: spec wants enabled, observed disabled",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				Enabled:  pointer.To(true),
			},
			observed: &keys.Key{KeyAlias: "alias", KeyState: keyStateDisabled},
			want:     false,
		},
		{
			name: "enabled drift: spec wants disabled, observed enabled",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				Enabled:  pointer.To(false),
			},
			observed: &keys.Key{KeyAlias: "alias", KeyState: keyStateEnabled},
			want:     false,
		},
		{
			name: "enabled stable during transition is up-to-date",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				Enabled:  pointer.To(true),
			},
			observed: &keys.Key{KeyAlias: "alias", KeyState: keyStateWaitingForEnable},
			want:     true,
		},
		{
			name: "tags mismatch",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias: "alias",
				Tags:     map[string]string{"env": "prod"},
			},
			observed:     &keys.Key{KeyAlias: "alias", KeyState: keyStateEnabled},
			observedTags: map[string]string{"env": "dev"},
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isKeyUpToDate(tt.spec, tt.observed, tt.observedTags, nil, tt.observed.KeyState)
			if got != tt.want {
				t.Errorf("isKeyUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEnabledUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		desired  *bool
		keyState string
		want     bool
	}{
		{
			name:     "nil desired is always up to date",
			desired:  nil,
			keyState: keyStateDisabled,
			want:     true,
		},
		{
			name:     "want enabled, is enabled",
			desired:  pointer.To(true),
			keyState: keyStateEnabled,
			want:     true,
		},
		{
			name:     "want enabled, is disabled",
			desired:  pointer.To(true),
			keyState: keyStateDisabled,
			want:     false,
		},
		{
			name:     "want disabled, is enabled",
			desired:  pointer.To(false),
			keyState: keyStateEnabled,
			want:     false,
		},
		{
			name:     "want disabled, is disabled",
			desired:  pointer.To(false),
			keyState: keyStateDisabled,
			want:     true,
		},
		{
			name:     "transitional waiting-for-enable is up-to-date",
			desired:  pointer.To(true),
			keyState: keyStateWaitingForEnable,
			want:     true,
		},
		{
			name:     "transitional waiting-for-import is up-to-date",
			desired:  pointer.To(false),
			keyState: keyStateWaitingForImport,
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEnabledUpToDate(tt.desired, tt.keyState); got != tt.want {
				t.Errorf(
					"isEnabledUpToDate(%v,%q) = %v, want %v",
					tt.desired,
					tt.keyState,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestIsRotationUpToDate(t *testing.T) {
	spec := kmsv1alpha1.KeyParameters{
		KeyAlias:         "alias",
		RotationEnabled:  pointer.To(true),
		RotationInterval: pointer.To(90),
	}
	rotation := &keys.KeyRotationResult{Enabled: false, Interval: 30}

	if isRotationUpToDate(spec, rotation, keyStateEnabled) {
		t.Fatal("enabled rotation mismatch reported up-to-date")
	}
	if !isRotationUpToDate(spec, rotation, keyStateWaitingForEnable) {
		t.Fatal("transitional key state should ignore rotation drift")
	}
}

func TestValidateKeyParameters(t *testing.T) {
	tests := []struct {
		name    string
		spec    kmsv1alpha1.KeyParameters
		wantErr bool
	}{
		{
			name:    "missing alias",
			spec:    kmsv1alpha1.KeyParameters{},
			wantErr: true,
		},
		{
			name: "rotation interval requires rotation enabled",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:         "alias",
				RotationInterval: pointer.To(90),
			},
			wantErr: true,
		},
		{
			name: "rotation interval rejects disabled rotation",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:         "alias",
				RotationEnabled:  pointer.To(false),
				RotationInterval: pointer.To(90),
			},
			wantErr: true,
		},
		{
			name: "rotation interval allows enabled rotation",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:         "alias",
				RotationEnabled:  pointer.To(true),
				RotationInterval: pointer.To(90),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKeyParameters(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateKeyParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLateInitializeKey(t *testing.T) {
	tests := []struct {
		name             string
		spec             kmsv1alpha1.KeyParameters
		observed         *keys.Key
		tags             map[string]string
		rotationStatus   *keys.KeyRotationResult
		wantChanged      bool
		wantDesc         *string
		wantRealm        *string
		wantTags         map[string]string
		wantRotation     *bool
		wantRotationDays *int
	}{
		{
			name:        "unset fields get late-initialized",
			spec:        kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			observed:    &keys.Key{KeyDescription: "auto desc", Realm: "eu-de"},
			tags:        map[string]string{"env": "dev"},
			wantChanged: true,
			wantDesc:    pointer.To("auto desc"),
			wantRealm:   pointer.To("eu-de"),
			wantTags:    map[string]string{"env": "dev"},
		},
		{
			name: "already set fields are not overwritten",
			spec: kmsv1alpha1.KeyParameters{
				KeyAlias:    "alias",
				Description: pointer.To("user desc"),
				Realm:       pointer.To("eu-nl"),
				Tags:        map[string]string{"env": "prod"},
			},
			observed:    &keys.Key{KeyDescription: "auto", Realm: "eu-de"},
			tags:        map[string]string{"env": "dev"},
			wantChanged: false,
			wantDesc:    pointer.To("user desc"),
			wantRealm:   pointer.To("eu-nl"),
			wantTags:    map[string]string{"env": "prod"},
		},
		{
			name:        "empty observed values do not late-initialize",
			spec:        kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			observed:    &keys.Key{},
			tags:        nil,
			wantChanged: false,
		},
		{
			name:           "disabled rotation does not late-initialize interval",
			spec:           kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			observed:       &keys.Key{},
			rotationStatus: &keys.KeyRotationResult{Enabled: false, Interval: 90},
			wantChanged:    true,
			wantRotation:   pointer.To(false),
		},
		{
			name:             "enabled rotation late-initializes interval",
			spec:             kmsv1alpha1.KeyParameters{KeyAlias: "alias"},
			observed:         &keys.Key{},
			rotationStatus:   &keys.KeyRotationResult{Enabled: true, Interval: 90},
			wantChanged:      true,
			wantRotation:     pointer.To(true),
			wantRotationDays: pointer.To(90),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &kmsv1alpha1.Key{
				Spec: kmsv1alpha1.KeySpec{ForProvider: tt.spec},
			}
			li := resource.NewLateInitializer()
			lateInitializeKey(cr, tt.observed, tt.tags, tt.rotationStatus, li)

			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
			p := cr.Spec.ForProvider
			assertPtrEqual(t, "Description", p.Description, tt.wantDesc)
			assertPtrEqual(t, "Realm", p.Realm, tt.wantRealm)
			assertPtrEqual(t, "RotationEnabled", p.RotationEnabled, tt.wantRotation)
			assertPtrEqual(t, "RotationInterval", p.RotationInterval, tt.wantRotationDays)
			if !maps.Equal(p.Tags, tt.wantTags) {
				t.Errorf("Tags = %v, want %v", p.Tags, tt.wantTags)
			}
		})
	}
}

func TestSetKeyConditions(t *testing.T) {
	tests := []struct {
		name      string
		keyState  string
		wantReady xpv1.ConditionReason
	}{
		{name: "enabled -> Available", keyState: keyStateEnabled, wantReady: xpv1.ReasonAvailable},
		{
			name:      "waiting-for-enable -> Creating",
			keyState:  keyStateWaitingForEnable,
			wantReady: xpv1.ReasonCreating,
		},
		{
			name:      "pending-deletion -> Deleting",
			keyState:  keyStatePendingDeletion,
			wantReady: xpv1.ReasonDeleting,
		},
		{
			name:      "disabled -> Unavailable",
			keyState:  keyStateDisabled,
			wantReady: xpv1.ReasonUnavailable,
		},
		{
			name:      "waiting-for-import -> Unavailable",
			keyState:  keyStateWaitingForImport,
			wantReady: xpv1.ReasonUnavailable,
		},
		{name: "unknown -> Unavailable", keyState: "99", wantReady: xpv1.ReasonUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &kmsv1alpha1.Key{}
			setKeyConditions(cr, tt.keyState)
			cond := cr.GetCondition(xpv1.TypeReady)
			if cond.Reason != tt.wantReady {
				t.Errorf("Ready reason = %q, want %q", cond.Reason, tt.wantReady)
			}
		})
	}
}

func TestIsUnsupportedRotation(t *testing.T) {
	err400 := golangsdk.ErrUnexpectedResponseCode{Actual: http.StatusBadRequest}
	if !isUnsupportedRotation(err400, &keys.Key{Origin: keyOriginExternal}) {
		t.Fatal("external-origin 400 should be treated as unsupported rotation")
	}
	if isUnsupportedRotation(err400, &keys.Key{Origin: "kms"}) {
		t.Fatal("kms-origin 400 should not be treated as unsupported rotation")
	}
}

func TestKeyPollInterval(t *testing.T) {
	const fallback = 5 * time.Minute

	tests := []struct {
		name     string
		state    string
		wantFast bool
		deletion bool
	}{
		{name: "empty state polls fast", state: "", wantFast: true},
		{name: "waiting-for-enable polls fast", state: keyStateWaitingForEnable, wantFast: true},
		{name: "pending-deletion polls fast", state: keyStatePendingDeletion, wantFast: true},
		{name: "enabled uses fallback", state: keyStateEnabled, wantFast: false},
		{name: "disabled uses fallback", state: keyStateDisabled, wantFast: false},
		{
			name:     "waiting-for-import uses fallback",
			state:    keyStateWaitingForImport,
			wantFast: false,
		},
		{
			name:     "deletion timestamp forces fast",
			state:    keyStateEnabled,
			wantFast: true,
			deletion: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &kmsv1alpha1.Key{}
			cr.Status.AtProvider.KeyState = tt.state
			if tt.deletion {
				now := metav1.Now()
				cr.SetDeletionTimestamp(&now)
			}
			got := keyPollInterval(cr, fallback)
			gotFast := got != fallback
			if gotFast != tt.wantFast {
				t.Errorf("keyPollInterval state=%q deletion=%v fast=%v want=%v",
					tt.state, tt.deletion, gotFast, tt.wantFast)
			}
		})
	}
}

func assertPtrEqual[T comparable](t *testing.T, field string, got, want *T) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s: got nil=%v, want nil=%v", field, got == nil, want == nil)
		return
	}
	if got != nil && *got != *want {
		t.Errorf("%s = %v, want %v", field, *got, *want)
	}
}
