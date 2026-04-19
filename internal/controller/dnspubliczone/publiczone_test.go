package dnspubliczone

import (
	"maps"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/dns/v2/zones"

	dnsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestValidatePublicZoneParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  dnsv1alpha1.PublicZoneParameters
		wantErr bool
	}{
		{
			name: "valid minimal",
			params: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
			},
			wantErr: false,
		},
		{
			name: "valid with all fields",
			params: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(300),
				Description: pointer.To("my zone"),
				Tags:        map[string]string{"env": "dev"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			params: dnsv1alpha1.PublicZoneParameters{
				Email: pointer.To("admin@example.com"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublicZoneParameters(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePublicZoneParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPublicZoneUpToDate(t *testing.T) {
	tests := []struct {
		name         string
		spec         dnsv1alpha1.PublicZoneParameters
		observed     *zones.Zone
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(300),
				Description: pointer.To("my zone"),
				Tags:        map[string]string{"env": "dev"},
			},
			observed: &zones.Zone{
				Name:        "example.com.",
				Email:       "admin@example.com",
				TTL:         300,
				Description: "my zone",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "nil optional fields are up to date",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
			},
			observed: &zones.Zone{
				Name:        "example.com.",
				Email:       "admin@example.com",
				TTL:         300,
				Description: "some desc",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "email mismatch",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:  "example.com.",
				Email: pointer.To("new@example.com"),
			},
			observed: &zones.Zone{
				Name:  "example.com.",
				Email: "old@example.com",
			},
			want: false,
		},
		{
			name: "ttl mismatch",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
				TTL:  pointer.To(600),
			},
			observed: &zones.Zone{
				Name: "example.com.",
				TTL:  300,
			},
			want: false,
		},
		{
			name: "description mismatch",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Description: pointer.To("new desc"),
			},
			observed: &zones.Zone{
				Name:        "example.com.",
				Description: "old desc",
			},
			want: false,
		},
		{
			name: "tags mismatch",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
				Tags: map[string]string{"env": "prod"},
			},
			observed: &zones.Zone{
				Name: "example.com.",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPublicZoneUpToDate(tt.spec, tt.observed, tt.observedTags)
			if got != tt.want {
				t.Errorf("isPublicZoneUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPublicZoneCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec dnsv1alpha1.PublicZoneParameters
		want zones.CreateOpts
	}{
		{
			name: "minimal",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
			},
			want: zones.CreateOpts{
				Name:     "example.com.",
				ZoneType: "public",
			},
		},
		{
			name: "with all fields",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(600),
				Description: pointer.To("my zone"),
			},
			want: zones.CreateOpts{
				Name:        "example.com.",
				ZoneType:    "public",
				Email:       "admin@example.com",
				TTL:         600,
				Description: "my zone",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPublicZoneCreateOpts(tt.spec)
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.ZoneType != tt.want.ZoneType {
				t.Errorf("ZoneType = %v, want %v", got.ZoneType, tt.want.ZoneType)
			}
			if got.Email != tt.want.Email {
				t.Errorf("Email = %v, want %v", got.Email, tt.want.Email)
			}
			if got.TTL != tt.want.TTL {
				t.Errorf("TTL = %v, want %v", got.TTL, tt.want.TTL)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %v, want %v", got.Description, tt.want.Description)
			}
			if got.Router != nil {
				t.Errorf("Router = %v, want nil", got.Router)
			}
		})
	}
}

func TestBuildPublicZoneUpdateOpts(t *testing.T) {
	tests := []struct {
		name            string
		spec            dnsv1alpha1.PublicZoneParameters
		observed        zones.Zone
		wantNeedsUpdate bool
		wantEmail       string
		wantTTL         int
		wantDesc        string
	}{
		{
			name: "no changes",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(300),
				Description: pointer.To("my zone"),
			},
			observed: zones.Zone{
				Email:       "admin@example.com",
				TTL:         300,
				Description: "my zone",
			},
			wantNeedsUpdate: false,
		},
		{
			name: "email changed",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:  "example.com.",
				Email: pointer.To("new@example.com"),
			},
			observed: zones.Zone{
				Email: "old@example.com",
			},
			wantNeedsUpdate: true,
			wantEmail:       "new@example.com",
		},
		{
			name: "ttl changed",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
				TTL:  pointer.To(600),
			},
			observed: zones.Zone{
				TTL: 300,
			},
			wantNeedsUpdate: true,
			wantTTL:         600,
		},
		{
			name: "description changed",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Description: pointer.To("new desc"),
			},
			observed: zones.Zone{
				Description: "old desc",
			},
			wantNeedsUpdate: true,
			wantDesc:        "new desc",
		},
		{
			name: "nil optional is no-op",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
			},
			observed: zones.Zone{
				Email:       "admin@example.com",
				TTL:         300,
				Description: "existing",
			},
			wantNeedsUpdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, needsUpdate := buildPublicZoneUpdateOpts(tt.spec, tt.observed)
			if needsUpdate != tt.wantNeedsUpdate {
				t.Errorf("needsUpdate = %v, want %v", needsUpdate, tt.wantNeedsUpdate)
			}
			if needsUpdate {
				if tt.wantEmail != "" && opts.Email != tt.wantEmail {
					t.Errorf("Email = %v, want %v", opts.Email, tt.wantEmail)
				}
				if tt.wantTTL != 0 && opts.TTL != tt.wantTTL {
					t.Errorf("TTL = %v, want %v", opts.TTL, tt.wantTTL)
				}
				if tt.wantDesc != "" && opts.Description != tt.wantDesc {
					t.Errorf("Description = %v, want %v", opts.Description, tt.wantDesc)
				}
			}
		})
	}
}

func TestLateInitializePublicZone(t *testing.T) {
	tests := []struct {
		name        string
		spec        dnsv1alpha1.PublicZoneParameters
		observed    *zones.Zone
		tags        map[string]string
		wantChanged bool
		wantEmail   *string
		wantTTL     *int
		wantDesc    *string
		wantTags    map[string]string
	}{
		{
			name: "unset fields get late-initialized",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
			},
			observed: &zones.Zone{
				Email:       "admin@example.com",
				TTL:         300,
				Description: "auto desc",
			},
			tags:        map[string]string{"env": "dev"},
			wantChanged: true,
			wantEmail:   pointer.To("admin@example.com"),
			wantTTL:     pointer.To(300),
			wantDesc:    pointer.To("auto desc"),
			wantTags:    map[string]string{"env": "dev"},
		},
		{
			name: "already set fields are not overwritten",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("my@example.com"),
				TTL:         pointer.To(600),
				Description: pointer.To("my desc"),
				Tags:        map[string]string{"env": "prod"},
			},
			observed: &zones.Zone{
				Email:       "other@example.com",
				TTL:         300,
				Description: "other desc",
			},
			tags:        map[string]string{"env": "dev"},
			wantChanged: false,
			wantEmail:   pointer.To("my@example.com"),
			wantTTL:     pointer.To(600),
			wantDesc:    pointer.To("my desc"),
			wantTags:    map[string]string{"env": "prod"},
		},
		{
			name: "empty observed values are not late-initialized",
			spec: dnsv1alpha1.PublicZoneParameters{
				Name: "example.com.",
			},
			observed:    &zones.Zone{},
			tags:        nil,
			wantChanged: false,
			wantEmail:   nil,
			wantTTL:     nil,
			wantDesc:    nil,
			wantTags:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &dnsv1alpha1.PublicZone{
				Spec: dnsv1alpha1.PublicZoneSpec{
					ForProvider: tt.spec,
				},
			}
			li := resource.NewLateInitializer()
			lateInitializePublicZone(cr, tt.observed, tt.tags, li)

			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
			p := cr.Spec.ForProvider
			if !pointer.Equal(p.Email, tt.wantEmail) {
				t.Errorf("Email = %v, want %v", p.Email, tt.wantEmail)
			}
			if !pointer.Equal(p.TTL, tt.wantTTL) {
				t.Errorf("TTL = %v, want %v", p.TTL, tt.wantTTL)
			}
			if !pointer.Equal(p.Description, tt.wantDesc) {
				t.Errorf("Description = %v, want %v", p.Description, tt.wantDesc)
			}
			if !maps.Equal(p.Tags, tt.wantTags) {
				t.Errorf("Tags = %v, want %v", p.Tags, tt.wantTags)
			}
		})
	}
}
