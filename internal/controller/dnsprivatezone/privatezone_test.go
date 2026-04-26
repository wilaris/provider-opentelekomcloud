package dnsprivatezone

import (
	"maps"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/dns/v2/zones"

	dnsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestValidatePrivateZoneParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  dnsv1alpha1.PrivateZoneParameters
		wantErr bool
	}{
		{
			name: "valid minimal",
			params: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with all fields",
			params: dnsv1alpha1.PrivateZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(300),
				Description: pointer.To("my zone"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
					{VPCID: pointer.To("vpc-456")},
				},
				Tags: map[string]string{"env": "dev"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			params: dnsv1alpha1.PrivateZoneParameters{
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			wantErr: true,
		},
		{
			name: "empty VPCs",
			params: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{},
			},
			wantErr: true,
		},
		{
			name: "VPC with empty vpcId",
			params: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("")},
				},
			},
			wantErr: true,
		},
		{
			name: "VPC with nil vpcId",
			params: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrivateZoneParameters(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePrivateZoneParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPrivateZoneUpToDate(t *testing.T) {
	tests := []struct {
		name         string
		spec         dnsv1alpha1.PrivateZoneParameters
		observed     *zones.Zone
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(300),
				Description: pointer.To("my zone"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
				Tags: map[string]string{"env": "dev"},
			},
			observed: &zones.Zone{
				Email:       "admin@example.com",
				TTL:         300,
				Description: "my zone",
				Routers: []zones.RouterResult{
					{RouterID: "vpc-123"},
				},
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "nil optional fields are up to date",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			observed: &zones.Zone{
				Email:       "admin@example.com",
				TTL:         300,
				Description: "some desc",
				Routers: []zones.RouterResult{
					{RouterID: "vpc-123"},
				},
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "description mismatch",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name:        "example.com.",
				Description: pointer.To("new desc"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			observed: &zones.Zone{
				Description: "old desc",
				Routers: []zones.RouterResult{
					{RouterID: "vpc-123"},
				},
			},
			want: false,
		},
		{
			name: "VPC count mismatch",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
					{VPCID: pointer.To("vpc-456")},
				},
			},
			observed: &zones.Zone{
				Routers: []zones.RouterResult{
					{RouterID: "vpc-123"},
				},
			},
			want: false,
		},
		{
			name: "VPC ID mismatch",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-new")},
				},
			},
			observed: &zones.Zone{
				Routers: []zones.RouterResult{
					{RouterID: "vpc-old"},
				},
			},
			want: false,
		},
		{
			name: "tags mismatch",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				Tags: map[string]string{"env": "prod"},
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			observed: &zones.Zone{
				Routers: []zones.RouterResult{
					{RouterID: "vpc-123"},
				},
			},
			observedTags: map[string]string{"env": "dev"},
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPrivateZoneUpToDate(tt.spec, tt.observed, tt.observedTags)
			if got != tt.want {
				t.Errorf("isPrivateZoneUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPrivateZoneCreateOpts(t *testing.T) {
	tests := []struct {
		name             string
		spec             dnsv1alpha1.PrivateZoneParameters
		region           string
		wantName         string
		wantZoneType     string
		wantEmail        string
		wantTTL          int
		wantDesc         string
		wantRouterID     string
		wantRouterRegion string
	}{
		{
			name: "minimal",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			region:           "eu-de",
			wantName:         "example.com.",
			wantZoneType:     "private",
			wantRouterID:     "vpc-123",
			wantRouterRegion: "eu-de",
		},
		{
			name: "with all fields",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(600),
				Description: pointer.To("my zone"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-first")},
					{VPCID: pointer.To("vpc-second")},
				},
			},
			region:           "eu-nl",
			wantName:         "example.com.",
			wantZoneType:     "private",
			wantEmail:        "admin@example.com",
			wantTTL:          600,
			wantDesc:         "my zone",
			wantRouterID:     "vpc-first",
			wantRouterRegion: "eu-nl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPrivateZoneCreateOpts(tt.spec, tt.region)
			if got.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", got.Name, tt.wantName)
			}
			if got.ZoneType != tt.wantZoneType {
				t.Errorf("ZoneType = %v, want %v", got.ZoneType, tt.wantZoneType)
			}
			if got.Email != tt.wantEmail {
				t.Errorf("Email = %v, want %v", got.Email, tt.wantEmail)
			}
			if got.TTL != tt.wantTTL {
				t.Errorf("TTL = %v, want %v", got.TTL, tt.wantTTL)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", got.Description, tt.wantDesc)
			}
			if got.Router == nil {
				t.Fatal("Router is nil, want non-nil")
			}
			if got.Router.RouterID != tt.wantRouterID {
				t.Errorf("Router.RouterID = %v, want %v", got.Router.RouterID, tt.wantRouterID)
			}
			if got.Router.RouterRegion != tt.wantRouterRegion {
				t.Errorf("Router.RouterRegion = %v, want %v", got.Router.RouterRegion, tt.wantRouterRegion)
			}
		})
	}
}

func TestBuildPrivateZoneUpdateOpts(t *testing.T) {
	tests := []struct {
		name            string
		spec            dnsv1alpha1.PrivateZoneParameters
		observed        zones.Zone
		wantNeedsUpdate bool
		wantEmail       string
		wantTTL         int
		wantDesc        string
	}{
		{
			name: "no changes",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("admin@example.com"),
				TTL:         pointer.To(300),
				Description: pointer.To("my zone"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
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
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name:  "example.com.",
				Email: pointer.To("new@example.com"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			observed: zones.Zone{
				Email: "old@example.com",
			},
			wantNeedsUpdate: true,
			wantEmail:       "new@example.com",
		},
		{
			name: "ttl changed",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				TTL:  pointer.To(600),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
			},
			observed: zones.Zone{
				TTL: 300,
			},
			wantNeedsUpdate: true,
			wantTTL:         600,
		},
		{
			name: "nil optional is no-op",
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
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
			opts, needsUpdate := buildPrivateZoneUpdateOpts(tt.spec, tt.observed)
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

func TestIsVPCsUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		desired  []dnsv1alpha1.VPC
		observed []zones.RouterResult
		want     bool
	}{
		{
			name: "same single VPC",
			desired: []dnsv1alpha1.VPC{
				{VPCID: pointer.To("vpc-123")},
			},
			observed: []zones.RouterResult{
				{RouterID: "vpc-123"},
			},
			want: true,
		},
		{
			name: "same multiple VPCs",
			desired: []dnsv1alpha1.VPC{
				{VPCID: pointer.To("vpc-aaa")},
				{VPCID: pointer.To("vpc-bbb")},
			},
			observed: []zones.RouterResult{
				{RouterID: "vpc-aaa"},
				{RouterID: "vpc-bbb"},
			},
			want: true,
		},
		{
			name: "reordered VPCs are still up to date",
			desired: []dnsv1alpha1.VPC{
				{VPCID: pointer.To("vpc-bbb")},
				{VPCID: pointer.To("vpc-aaa")},
			},
			observed: []zones.RouterResult{
				{RouterID: "vpc-aaa"},
				{RouterID: "vpc-bbb"},
			},
			want: true,
		},
		{
			name: "different count",
			desired: []dnsv1alpha1.VPC{
				{VPCID: pointer.To("vpc-123")},
				{VPCID: pointer.To("vpc-456")},
			},
			observed: []zones.RouterResult{
				{RouterID: "vpc-123"},
			},
			want: false,
		},
		{
			name: "different IDs",
			desired: []dnsv1alpha1.VPC{
				{VPCID: pointer.To("vpc-new")},
			},
			observed: []zones.RouterResult{
				{RouterID: "vpc-old"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVPCsUpToDate(tt.desired, tt.observed)
			if got != tt.want {
				t.Errorf("isVPCsUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLateInitializePrivateZone(t *testing.T) {
	tests := []struct {
		name        string
		spec        dnsv1alpha1.PrivateZoneParameters
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
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
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
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name:        "example.com.",
				Email:       pointer.To("my@example.com"),
				TTL:         pointer.To(600),
				Description: pointer.To("my desc"),
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
				Tags: map[string]string{"env": "prod"},
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
			spec: dnsv1alpha1.PrivateZoneParameters{
				Name: "example.com.",
				VPCs: []dnsv1alpha1.VPC{
					{VPCID: pointer.To("vpc-123")},
				},
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
			cr := &dnsv1alpha1.PrivateZone{
				Spec: dnsv1alpha1.PrivateZoneSpec{
					ForProvider: tt.spec,
				},
			}
			li := resource.NewLateInitializer()
			lateInitializePrivateZone(cr, tt.observed, tt.tags, li)

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
