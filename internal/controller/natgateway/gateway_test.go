package natgateway

import (
	"maps"
	"testing"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/natgateways"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	natv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/nat/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestValidateGatewayParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  natv1alpha1.GatewayParameters
		wantErr bool
	}{
		{
			name: "valid minimal",
			params: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			wantErr: false,
		},
		{
			name: "valid with all fields",
			params: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("test description"),
				Size:        "4",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
				Tags:        map[string]string{"env": "dev"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			params: natv1alpha1.GatewayParameters{
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			wantErr: true,
		},
		{
			name: "missing size",
			params: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			wantErr: true,
		},
		{
			name: "missing vpcId",
			params: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				SubnetID: pointer.To("subnet-456"),
			},
			wantErr: true,
		},
		{
			name: "missing subnetId",
			params: natv1alpha1.GatewayParameters{
				Name:  "my-nat",
				Size:  "1",
				VPCID: pointer.To("vpc-123"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGatewayParameters(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateGatewayParameters() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestIsGatewayUpToDate(t *testing.T) {
	tests := []struct {
		name         string
		spec         natv1alpha1.GatewayParameters
		observed     natgateways.NatGateway
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date",
			spec: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("desc"),
				Size:        "2",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
				Tags:        map[string]string{"env": "dev"},
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Description:       "desc",
				Spec:              "2",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "nil optional fields are up to date",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Description:       "some desc",
				Spec:              "1",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "name mismatch",
			spec: natv1alpha1.GatewayParameters{
				Name:     "new-name",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:              "old-name",
				Spec:              "1",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			want: false,
		},
		{
			name: "size mismatch",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "3",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Spec:              "1",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			want: false,
		},
		{
			name: "description mismatch",
			spec: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("new desc"),
				Size:        "1",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Description:       "old desc",
				Spec:              "1",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			want: false,
		},
		{
			name: "tags mismatch",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
				Tags:     map[string]string{"env": "prod"},
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Spec:              "1",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGatewayUpToDate(tt.spec, tt.observed, tt.observedTags)
			if got != tt.want {
				t.Errorf("isGatewayUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildGatewayCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec natv1alpha1.GatewayParameters
		want natgateways.CreateOpts
	}{
		{
			name: "minimal",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			want: natgateways.CreateOpts{
				Name:              "my-nat",
				Spec:              "1",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
		},
		{
			name: "with all fields",
			spec: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("test desc"),
				Size:        "4",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
				Tags:        map[string]string{"env": "dev"},
			},
			want: natgateways.CreateOpts{
				Name:              "my-nat",
				Description:       "test desc",
				Spec:              "4",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGatewayCreateOpts(tt.spec)
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %v, want %v", got.Description, tt.want.Description)
			}
			if got.Spec != tt.want.Spec {
				t.Errorf("Spec = %v, want %v", got.Spec, tt.want.Spec)
			}
			if got.RouterID != tt.want.RouterID {
				t.Errorf("RouterID = %v, want %v", got.RouterID, tt.want.RouterID)
			}
			if got.InternalNetworkID != tt.want.InternalNetworkID {
				t.Errorf(
					"InternalNetworkID = %v, want %v",
					got.InternalNetworkID,
					tt.want.InternalNetworkID,
				)
			}
		})
	}
}

func TestBuildGatewayUpdateOpts(t *testing.T) {
	tests := []struct {
		name            string
		spec            natv1alpha1.GatewayParameters
		observed        natgateways.NatGateway
		wantNeedsUpdate bool
		wantName        string
		wantSpec        string
		wantDesc        string
	}{
		{
			name: "no changes",
			spec: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("desc"),
				Size:        "2",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:        "my-nat",
				Description: "desc",
				Spec:        "2",
			},
			wantNeedsUpdate: false,
		},
		{
			name: "name changed",
			spec: natv1alpha1.GatewayParameters{
				Name:     "new-name",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name: "old-name",
				Spec: "1",
			},
			wantNeedsUpdate: true,
			wantName:        "new-name",
		},
		{
			name: "size changed",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "3",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name: "my-nat",
				Spec: "1",
			},
			wantNeedsUpdate: true,
			wantSpec:        "3",
		},
		{
			name: "description changed",
			spec: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("new desc"),
				Size:        "1",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:        "my-nat",
				Description: "old desc",
				Spec:        "1",
			},
			wantNeedsUpdate: true,
			wantDesc:        "new desc",
		},
		{
			name: "nil description is no-op",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				Name:        "my-nat",
				Description: "existing desc",
				Spec:        "1",
			},
			wantNeedsUpdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, needsUpdate := buildGatewayUpdateOpts(tt.spec, tt.observed)
			if needsUpdate != tt.wantNeedsUpdate {
				t.Errorf("needsUpdate = %v, want %v", needsUpdate, tt.wantNeedsUpdate)
			}
			if needsUpdate {
				if tt.wantName != "" && opts.Name != tt.wantName {
					t.Errorf("Name = %v, want %v", opts.Name, tt.wantName)
				}
				if tt.wantSpec != "" && opts.Spec != tt.wantSpec {
					t.Errorf("Spec = %v, want %v", opts.Spec, tt.wantSpec)
				}
				if tt.wantDesc != "" && opts.Description != tt.wantDesc {
					t.Errorf("Description = %v, want %v", opts.Description, tt.wantDesc)
				}
			}
		})
	}
}

func TestValidateImmutableGatewayFields(t *testing.T) {
	tests := []struct {
		name     string
		spec     natv1alpha1.GatewayParameters
		observed natgateways.NatGateway
		wantErr  bool
	}{
		{
			name: "all unchanged",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			wantErr: false,
		},
		{
			name: "nil optional immutables are ok",
			spec: natv1alpha1.GatewayParameters{
				Name: "my-nat",
				Size: "1",
			},
			observed: natgateways.NatGateway{
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			wantErr: false,
		},
		{
			name: "vpcId changed",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-new"),
				SubnetID: pointer.To("subnet-456"),
			},
			observed: natgateways.NatGateway{
				RouterID:          "vpc-old",
				InternalNetworkID: "subnet-456",
			},
			wantErr: true,
		},
		{
			name: "subnetId changed",
			spec: natv1alpha1.GatewayParameters{
				Name:     "my-nat",
				Size:     "1",
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-new"),
			},
			observed: natgateways.NatGateway{
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-old",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImmutableGatewayFields(tt.spec, tt.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateImmutableGatewayFields() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestLateInitializeGateway(t *testing.T) {
	tests := []struct {
		name        string
		spec        natv1alpha1.GatewayParameters
		observed    natgateways.NatGateway
		tags        map[string]string
		wantChanged bool
		wantDesc    *string
		wantVPCID   *string
		wantSubnet  *string
		wantTags    map[string]string
	}{
		{
			name: "unset fields get late-initialized",
			spec: natv1alpha1.GatewayParameters{
				Name: "my-nat",
				Size: "1",
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Description:       "auto desc",
				RouterID:          "vpc-123",
				InternalNetworkID: "subnet-456",
			},
			tags:        map[string]string{"env": "dev"},
			wantChanged: true,
			wantDesc:    pointer.To("auto desc"),
			wantVPCID:   pointer.To("vpc-123"),
			wantSubnet:  pointer.To("subnet-456"),
			wantTags:    map[string]string{"env": "dev"},
		},
		{
			name: "already set fields are not overwritten",
			spec: natv1alpha1.GatewayParameters{
				Name:        "my-nat",
				Description: pointer.To("my desc"),
				Size:        "1",
				VPCID:       pointer.To("vpc-123"),
				SubnetID:    pointer.To("subnet-456"),
				Tags:        map[string]string{"env": "prod"},
			},
			observed: natgateways.NatGateway{
				Name:              "my-nat",
				Description:       "other desc",
				RouterID:          "vpc-other",
				InternalNetworkID: "subnet-other",
			},
			tags:        map[string]string{"env": "dev"},
			wantChanged: false,
			wantDesc:    pointer.To("my desc"),
			wantVPCID:   pointer.To("vpc-123"),
			wantSubnet:  pointer.To("subnet-456"),
			wantTags:    map[string]string{"env": "prod"},
		},
		{
			name: "empty observed values are not late-initialized",
			spec: natv1alpha1.GatewayParameters{
				Name: "my-nat",
				Size: "1",
			},
			observed: natgateways.NatGateway{
				Name: "my-nat",
			},
			tags:        nil,
			wantChanged: false,
			wantDesc:    nil,
			wantVPCID:   nil,
			wantSubnet:  nil,
			wantTags:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &natv1alpha1.Gateway{
				Spec: natv1alpha1.GatewaySpec{
					ForProvider: tt.spec,
				},
			}
			li := resource.NewLateInitializer()
			lateInitializeGateway(cr, tt.observed, tt.tags, li)

			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
			p := cr.Spec.ForProvider
			if (p.Description == nil) != (tt.wantDesc == nil) {
				t.Errorf("Description = %v, want %v", p.Description, tt.wantDesc)
			} else if p.Description != nil && *p.Description != *tt.wantDesc {
				t.Errorf("Description = %v, want %v", *p.Description, *tt.wantDesc)
			}
			if (p.VPCID == nil) != (tt.wantVPCID == nil) {
				t.Errorf("VPCID = %v, want %v", p.VPCID, tt.wantVPCID)
			} else if p.VPCID != nil && *p.VPCID != *tt.wantVPCID {
				t.Errorf("VPCID = %v, want %v", *p.VPCID, *tt.wantVPCID)
			}
			if (p.SubnetID == nil) != (tt.wantSubnet == nil) {
				t.Errorf("SubnetID = %v, want %v", p.SubnetID, tt.wantSubnet)
			} else if p.SubnetID != nil && *p.SubnetID != *tt.wantSubnet {
				t.Errorf("SubnetID = %v, want %v", *p.SubnetID, *tt.wantSubnet)
			}
			if !maps.Equal(p.Tags, tt.wantTags) {
				t.Errorf("Tags = %v, want %v", p.Tags, tt.wantTags)
			}
		})
	}
}
