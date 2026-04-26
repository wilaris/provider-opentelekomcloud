package vpc

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/vpcs"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestCreateSkipsWhenExternalNameSet(t *testing.T) {
	cr := &networkv1alpha1.VPC{}
	meta.SetExternalName(cr, "existing-vpc")

	if _, err := (&external{}).Create(context.Background(), cr); err != nil {
		t.Fatalf("Create() returned error for existing external-name: %v", err)
	}
}

func TestValidateVPCParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  networkv1alpha1.VPCParameters
		wantErr bool
	}{
		{
			name: "valid minimal parameters",
			params: networkv1alpha1.VPCParameters{
				Name: "example",
				CIDR: "10.0.0.0/16",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			params: networkv1alpha1.VPCParameters{
				CIDR: "10.0.0.0/16",
			},
			wantErr: true,
		},
		{
			name: "missing CIDR",
			params: networkv1alpha1.VPCParameters{
				Name: "example",
			},
			wantErr: true,
		},
		{
			name: "invalid CIDR",
			params: networkv1alpha1.VPCParameters{
				Name: "example",
				CIDR: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid secondary CIDR",
			params: networkv1alpha1.VPCParameters{
				Name:          "example",
				CIDR:          "10.0.0.0/16",
				SecondaryCIDR: pointer.To("also-invalid"),
			},
			wantErr: true,
		},
		{
			name: "valid with secondary CIDR",
			params: networkv1alpha1.VPCParameters{
				Name:          "example",
				CIDR:          "10.0.0.0/16",
				SecondaryCIDR: pointer.To("10.10.0.0/16"),
			},
			wantErr: false,
		},
		{
			name: "empty secondary CIDR is valid",
			params: networkv1alpha1.VPCParameters{
				Name:          "example",
				CIDR:          "10.0.0.0/16",
				SecondaryCIDR: pointer.To(""),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVPCParameters(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVPCParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsVPCUpToDate(t *testing.T) {
	baseObserved := &vpcs.Vpc{
		Name:        "vpc-1",
		CIDR:        "10.0.0.0/16",
		Description: "desc",
	}
	baseSecondary := "10.10.0.0/16"
	baseTags := map[string]string{"env": "dev"}

	tests := []struct {
		name              string
		spec              networkv1alpha1.VPCParameters
		observed          *vpcs.Vpc
		observedSecondary string
		observedTags      map[string]string
		want              bool
	}{
		{
			name: "fully up to date",
			spec: networkv1alpha1.VPCParameters{
				Name:          "vpc-1",
				CIDR:          "10.0.0.0/16",
				Description:   pointer.To("desc"),
				SecondaryCIDR: pointer.To("10.10.0.0/16"),
				Tags:          map[string]string{"env": "dev"},
			},
			observed:          baseObserved,
			observedSecondary: baseSecondary,
			observedTags:      baseTags,
			want:              true,
		},
		{
			name: "tags mismatch",
			spec: networkv1alpha1.VPCParameters{
				Name:          "vpc-1",
				CIDR:          "10.0.0.0/16",
				Description:   pointer.To("desc"),
				SecondaryCIDR: pointer.To("10.10.0.0/16"),
				Tags:          map[string]string{"env": "prod"},
			},
			observed:          baseObserved,
			observedSecondary: baseSecondary,
			observedTags:      baseTags,
			want:              false,
		},
		{
			name: "name mismatch",
			spec: networkv1alpha1.VPCParameters{
				Name:          "vpc-2",
				CIDR:          "10.0.0.0/16",
				Description:   pointer.To("desc"),
				SecondaryCIDR: pointer.To("10.10.0.0/16"),
				Tags:          map[string]string{"env": "dev"},
			},
			observed:          baseObserved,
			observedSecondary: baseSecondary,
			observedTags:      baseTags,
			want:              false,
		},
		{
			name: "nil optional fields are up to date",
			spec: networkv1alpha1.VPCParameters{
				Name: "vpc-1",
				CIDR: "10.0.0.0/16",
			},
			observed:          baseObserved,
			observedSecondary: baseSecondary,
			observedTags:      baseTags,
			want:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVPCUpToDate(tt.spec, tt.observed, tt.observedSecondary, tt.observedTags)
			if got != tt.want {
				t.Errorf("isVPCUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildVPCUpdateOpts(t *testing.T) {
	tests := []struct {
		name           string
		spec           networkv1alpha1.VPCParameters
		observed       *vpcs.Vpc
		wantUpdate     bool
		wantName       string
		wantDescNotNil bool
	}{
		{
			name: "no changes needed",
			spec: networkv1alpha1.VPCParameters{
				Name:        "vpc-1",
				CIDR:        "10.0.0.0/16",
				Description: pointer.To("desc"),
			},
			observed:   &vpcs.Vpc{Name: "vpc-1", Description: "desc"},
			wantUpdate: false,
			wantName:   "vpc-1",
		},
		{
			name: "name changed",
			spec: networkv1alpha1.VPCParameters{
				Name: "vpc-2",
				CIDR: "10.0.0.0/16",
			},
			observed:   &vpcs.Vpc{Name: "vpc-1", Description: ""},
			wantUpdate: true,
			wantName:   "vpc-2",
		},
		{
			name: "description changed",
			spec: networkv1alpha1.VPCParameters{
				Name:        "vpc-1",
				CIDR:        "10.0.0.0/16",
				Description: pointer.To("new desc"),
			},
			observed:       &vpcs.Vpc{Name: "vpc-1", Description: "old desc"},
			wantUpdate:     true,
			wantName:       "vpc-1",
			wantDescNotNil: true,
		},
		{
			name: "nil description is no-op",
			spec: networkv1alpha1.VPCParameters{
				Name: "vpc-1",
				CIDR: "10.0.0.0/16",
			},
			observed:   &vpcs.Vpc{Name: "vpc-1", Description: "desc"},
			wantUpdate: false,
			wantName:   "vpc-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, needsUpdate := buildVPCUpdateOpts(tt.spec, tt.observed)
			if needsUpdate != tt.wantUpdate {
				t.Errorf(
					"buildVPCUpdateOpts() needsUpdate = %v, want %v",
					needsUpdate,
					tt.wantUpdate,
				)
			}
			if opts.Name != tt.wantName {
				t.Errorf("buildVPCUpdateOpts() Name = %q, want %q", opts.Name, tt.wantName)
			}
			if tt.wantDescNotNil && opts.Description == nil {
				t.Error("buildVPCUpdateOpts() Description is nil, want non-nil")
			}
		})
	}
}

func TestValidateImmutableVPCFields(t *testing.T) {
	tests := []struct {
		name     string
		spec     networkv1alpha1.VPCParameters
		observed *vpcs.Vpc
		wantErr  bool
	}{
		{
			name:     "CIDR unchanged",
			spec:     networkv1alpha1.VPCParameters{Name: "vpc-1", CIDR: "10.0.0.0/16"},
			observed: &vpcs.Vpc{CIDR: "10.0.0.0/16"},
			wantErr:  false,
		},
		{
			name:     "CIDR changed",
			spec:     networkv1alpha1.VPCParameters{Name: "vpc-1", CIDR: "10.1.0.0/16"},
			observed: &vpcs.Vpc{CIDR: "10.0.0.0/16"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImmutableVPCFields(tt.spec, tt.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImmutableVPCFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLateInitializeVPC(t *testing.T) {
	cr := &networkv1alpha1.VPC{
		Spec: networkv1alpha1.VPCSpec{
			ForProvider: networkv1alpha1.VPCParameters{
				Name: "vpc-1",
				CIDR: "10.0.0.0/16",
			},
		},
	}

	observed := &vpcs.Vpc{
		Description: "desc from provider",
	}
	observedTags := map[string]string{"env": "dev"}

	li := resource.NewLateInitializer()
	lateInitializeVPC(cr, observed, "10.10.0.0/16", observedTags, li)

	if !li.IsChanged() {
		t.Fatal("lateInitializeVPC() did not mark resource as changed")
	}
	if cr.Spec.ForProvider.Description == nil ||
		*cr.Spec.ForProvider.Description != "desc from provider" {
		t.Fatalf("description not late initialized: %#v", cr.Spec.ForProvider.Description)
	}
	if cr.Spec.ForProvider.SecondaryCIDR == nil ||
		*cr.Spec.ForProvider.SecondaryCIDR != "10.10.0.0/16" {
		t.Fatalf("secondaryCIDR not late initialized: %#v", cr.Spec.ForProvider.SecondaryCIDR)
	}
	if len(cr.Spec.ForProvider.Tags) != 1 || cr.Spec.ForProvider.Tags["env"] != "dev" {
		t.Fatalf("tags not late initialized: %#v", cr.Spec.ForProvider.Tags)
	}
}

func TestLateInitializeVPCNoChange(t *testing.T) {
	cr := &networkv1alpha1.VPC{
		Spec: networkv1alpha1.VPCSpec{
			ForProvider: networkv1alpha1.VPCParameters{
				Name:        "vpc-1",
				CIDR:        "10.0.0.0/16",
				Description: pointer.To("already set"),
			},
		},
	}

	observed := &vpcs.Vpc{Description: "provider value"}

	li := resource.NewLateInitializer()
	lateInitializeVPC(cr, observed, "", nil, li)

	if li.IsChanged() {
		t.Fatal("lateInitializeVPC() should not mark as changed when all fields are already set")
	}
	if *cr.Spec.ForProvider.Description != "already set" {
		t.Fatalf("description was overwritten: %q", *cr.Spec.ForProvider.Description)
	}
}
