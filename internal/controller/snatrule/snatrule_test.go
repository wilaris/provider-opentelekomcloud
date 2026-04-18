package snatrule

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/snatrules"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestBuildCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec networkv1alpha1.SNATRuleParameters
		want snatrules.CreateOpts
	}{
		{
			name: "with subnetId",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
				SourceType:   pointer.To("VPC"),
			},
			want: snatrules.CreateOpts{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
				SourceType:   0,
			},
		},
		{
			name: "with cidr",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				CIDR:         pointer.To("192.168.1.0/24"),
				SourceType:   pointer.To("VPC"),
				Description:  pointer.To("test rule"),
			},
			want: snatrules.CreateOpts{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				Cidr:         "192.168.1.0/24",
				SourceType:   0,
				Description:  "test rule",
			},
		},
		{
			name: "minimal with subnetId and no source_type",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
			},
			want: snatrules.CreateOpts{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
				SourceType:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCreateOpts(tt.spec)
			if got != tt.want {
				t.Errorf("buildCreateOpts() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsSNATRuleUpToDate(t *testing.T) {
	tests := []struct {
		name       string
		spec       networkv1alpha1.SNATRuleParameters
		observed   *snatrules.SnatRule
		sourceType int
		want       bool
	}{
		{
			name: "all fields match",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
				SourceType:   pointer.To("VPC"),
				Description:  pointer.To("test"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
				Description:  "test",
			},
			sourceType: 0,
			want:       true,
		},
		{
			name: "nil optional fields are up-to-date",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
				Description:  "some desc",
			},
			sourceType: 0,
			want:       true,
		},
		{
			name: "natGatewayId mismatch",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-different"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
			},
			sourceType: 0,
			want:       false,
		},
		{
			name: "elasticIpId mismatch",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-different"),
				SubnetID:     pointer.To("net-789"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
			},
			sourceType: 0,
			want:       false,
		},
		{
			name: "description mismatch",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
				Description:  pointer.To("changed"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				NetworkID:    "net-789",
				Description:  "original",
			},
			sourceType: 0,
			want:       false,
		},
		{
			name: "cidr match",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				CIDR:         pointer.To("10.0.0.0/8"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				Cidr:         "10.0.0.0/8",
			},
			sourceType: 0,
			want:       true,
		},
		{
			name: "cidr mismatch",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				CIDR:         pointer.To("10.0.0.0/16"),
			},
			observed: &snatrules.SnatRule{
				NatGatewayID: "gw-123",
				FloatingIPID: "eip-456",
				Cidr:         "10.0.0.0/8",
			},
			sourceType: 0,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSNATRuleUpToDate(tt.spec, tt.observed, tt.sourceType)
			if got != tt.want {
				t.Errorf("isSNATRuleUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertSourceType(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int
	}{
		{name: "float64 zero", input: float64(0), want: 0},
		{name: "float64 one", input: float64(1), want: 1},
		{name: "string zero", input: "0", want: 0},
		{name: "string one", input: "1", want: 1},
		{name: "int zero", input: int(0), want: 0},
		{name: "int one", input: int(1), want: 1},
		{name: "nil defaults to zero", input: nil, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSourceType(tt.input)
			if got != tt.want {
				t.Errorf("convertSourceType(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLateInitializeSNATRule(t *testing.T) {
	tests := []struct {
		name           string
		spec           networkv1alpha1.SNATRuleParameters
		observed       *snatrules.SnatRule
		sourceType     int
		wantChanged    bool
		wantSubnetID   *string
		wantCIDR       *string
		wantSourceType *string
		wantDesc       *string
	}{
		{
			name: "late-init unset fields",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
			},
			observed: &snatrules.SnatRule{
				NetworkID:   "net-789",
				Description: "initialized",
			},
			sourceType:     0,
			wantChanged:    true,
			wantSubnetID:   pointer.To("net-789"),
			wantDesc:       pointer.To("initialized"),
			wantSourceType: pointer.To("VPC"),
		},
		{
			name: "already-set fields not overwritten",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-original"),
				Description:  pointer.To("original"),
				SourceType:   pointer.To("VPC"),
			},
			observed: &snatrules.SnatRule{
				NetworkID:   "net-different",
				Description: "different",
			},
			sourceType:     0,
			wantChanged:    false,
			wantSubnetID:   pointer.To("net-original"),
			wantDesc:       pointer.To("original"),
			wantSourceType: pointer.To("VPC"),
		},
		{
			name: "zero observed values are not late-initialized",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
			},
			observed:       &snatrules.SnatRule{},
			sourceType:     0,
			wantChanged:    true,
			wantSubnetID:   nil,
			wantCIDR:       nil,
			wantDesc:       nil,
			wantSourceType: pointer.To("VPC"), // 0 maps to "VPC"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &networkv1alpha1.SNATRule{
				Spec: networkv1alpha1.SNATRuleSpec{
					ForProvider: tt.spec,
				},
			}
			li := resource.NewLateInitializer()
			lateInitializeSNATRule(cr, tt.observed, tt.sourceType, li)

			if li.IsChanged() != tt.wantChanged {
				t.Errorf("changed = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
			if !pointer.Equal(cr.Spec.ForProvider.SubnetID, tt.wantSubnetID) {
				t.Errorf(
					"SubnetID = %v, want %v",
					pointer.Deref(cr.Spec.ForProvider.SubnetID, "<nil>"),
					pointer.Deref(tt.wantSubnetID, "<nil>"),
				)
			}
			if !pointer.Equal(cr.Spec.ForProvider.CIDR, tt.wantCIDR) {
				t.Errorf(
					"CIDR = %v, want %v",
					pointer.Deref(cr.Spec.ForProvider.CIDR, "<nil>"),
					pointer.Deref(tt.wantCIDR, "<nil>"),
				)
			}
			if !pointer.Equal(cr.Spec.ForProvider.SourceType, tt.wantSourceType) {
				t.Errorf(
					"SourceType = %v, want %v",
					pointer.Deref(cr.Spec.ForProvider.SourceType, "<nil>"),
					pointer.Deref(tt.wantSourceType, "<nil>"),
				)
			}
			if !pointer.Equal(cr.Spec.ForProvider.Description, tt.wantDesc) {
				t.Errorf(
					"Description = %v, want %v",
					pointer.Deref(cr.Spec.ForProvider.Description, "<nil>"),
					pointer.Deref(tt.wantDesc, "<nil>"),
				)
			}
		})
	}
}

func TestValidateSNATRuleParameters(t *testing.T) {
	tests := []struct {
		name    string
		spec    networkv1alpha1.SNATRuleParameters
		wantErr string
	}{
		{
			name: "valid with subnetId",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
			},
		},
		{
			name: "valid with cidr",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				CIDR:         pointer.To("10.0.0.0/8"),
			},
		},
		{
			name: "empty natGatewayId",
			spec: networkv1alpha1.SNATRuleParameters{
				ElasticIPID: pointer.To("eip-456"),
				SubnetID:    pointer.To("net-789"),
			},
			wantErr: errEmptyNatGateway,
		},
		{
			name: "empty elasticIpId",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				SubnetID:     pointer.To("net-789"),
			},
			wantErr: errEmptyElasticIP,
		},
		{
			name: "both subnetId and cidr",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
				SubnetID:     pointer.To("net-789"),
				CIDR:         pointer.To("10.0.0.0/8"),
			},
			wantErr: errMutuallyExclusive,
		},
		{
			name: "neither subnetId nor cidr",
			spec: networkv1alpha1.SNATRuleParameters{
				NatGatewayID: pointer.To("gw-123"),
				ElasticIPID:  pointer.To("eip-456"),
			},
			wantErr: errNeitherSubnetNor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSNATRuleParameters(tt.spec)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateSNATRuleParameters() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("validateSNATRuleParameters() expected error %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf(
					"validateSNATRuleParameters() error = %q, want %q",
					err.Error(),
					tt.wantErr,
				)
			}
		})
	}
}
