package securitygroup

import (
	"maps"
	"testing"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/common/tags"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/vpc/v3/security/group"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
	"go.wilaris.de/provider-opentelekomcloud/internal/util"
)

func TestValidateSecurityGroupParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  networkv1alpha1.SecurityGroupParameters
		wantErr bool
	}{
		{
			name:    "valid minimal",
			params:  networkv1alpha1.SecurityGroupParameters{Name: "my-sg"},
			wantErr: false,
		},
		{
			name: "valid with all fields",
			params: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				Description:         pointer.To("test description"),
				EnterpriseProjectID: pointer.To("0"),
				Tags:                map[string]string{"env": "dev"},
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			params:  networkv1alpha1.SecurityGroupParameters{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecurityGroupParameters(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateSecurityGroupParameters() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestIsSecurityGroupUpToDate(t *testing.T) {
	tests := []struct {
		name         string
		spec         networkv1alpha1.SecurityGroupParameters
		observed     *group.SecurityGroup
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				Description:         pointer.To("desc"),
				EnterpriseProjectID: pointer.To("0"),
				Tags:                map[string]string{"env": "dev"},
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				Description:         "desc",
				EnterpriseProjectID: "0",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "nil optional fields are up to date",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				Description:         "some desc",
				EnterpriseProjectID: "0",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "name mismatch",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "new-name",
			},
			observed: &group.SecurityGroup{
				Name: "old-name",
			},
			observedTags: nil,
			want:         false,
		},
		{
			name: "description mismatch",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:        "my-sg",
				Description: pointer.To("new desc"),
			},
			observed: &group.SecurityGroup{
				Name:        "my-sg",
				Description: "old desc",
			},
			observedTags: nil,
			want:         false,
		},
		{
			name: "enterprise project ID mismatch",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				EnterpriseProjectID: pointer.To("proj-1"),
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				EnterpriseProjectID: "proj-2",
			},
			observedTags: nil,
			want:         false,
		},
		{
			name: "tags mismatch",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
				Tags: map[string]string{"env": "prod"},
			},
			observed: &group.SecurityGroup{
				Name: "my-sg",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecurityGroupUpToDate(tt.spec, tt.observed, tt.observedTags)
			if got != tt.want {
				t.Errorf("isSecurityGroupUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSecurityGroupCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec networkv1alpha1.SecurityGroupParameters
		want group.CreateOpts
	}{
		{
			name: "minimal",
			spec: networkv1alpha1.SecurityGroupParameters{Name: "my-sg"},
			want: group.CreateOpts{
				SecurityGroup: group.SecurityGroupOptions{
					Name: "my-sg",
				},
			},
		},
		{
			name: "with all fields",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				Description:         pointer.To("test desc"),
				EnterpriseProjectID: pointer.To("proj-1"),
				Tags:                map[string]string{"env": "dev", "team": "infra"},
			},
			want: group.CreateOpts{
				SecurityGroup: group.SecurityGroupOptions{
					Name:                "my-sg",
					Description:         "test desc",
					EnterpriseProjectId: "proj-1",
					Tags: util.MapToResourceTags(
						map[string]string{"env": "dev", "team": "infra"},
					),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSecurityGroupCreateOpts(tt.spec)
			if got.SecurityGroup.Name != tt.want.SecurityGroup.Name {
				t.Errorf("Name = %v, want %v", got.SecurityGroup.Name, tt.want.SecurityGroup.Name)
			}
			if got.SecurityGroup.Description != tt.want.SecurityGroup.Description {
				t.Errorf(
					"Description = %v, want %v",
					got.SecurityGroup.Description,
					tt.want.SecurityGroup.Description,
				)
			}
			if got.SecurityGroup.EnterpriseProjectId != tt.want.SecurityGroup.EnterpriseProjectId {
				t.Errorf(
					"EnterpriseProjectId = %v, want %v",
					got.SecurityGroup.EnterpriseProjectId,
					tt.want.SecurityGroup.EnterpriseProjectId,
				)
			}
			if len(got.SecurityGroup.Tags) != len(tt.want.SecurityGroup.Tags) {
				t.Errorf(
					"Tags length = %v, want %v",
					len(got.SecurityGroup.Tags),
					len(tt.want.SecurityGroup.Tags),
				)
			}
		})
	}
}

func TestBuildSecurityGroupUpdateOpts(t *testing.T) {
	tests := []struct {
		name            string
		spec            networkv1alpha1.SecurityGroupParameters
		observed        *group.SecurityGroup
		wantNeedsUpdate bool
		wantName        string
		wantDesc        string
	}{
		{
			name: "no changes",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:        "my-sg",
				Description: pointer.To("desc"),
			},
			observed: &group.SecurityGroup{
				Name:        "my-sg",
				Description: "desc",
			},
			wantNeedsUpdate: false,
		},
		{
			name: "name changed",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "new-name",
			},
			observed: &group.SecurityGroup{
				Name: "old-name",
			},
			wantNeedsUpdate: true,
			wantName:        "new-name",
		},
		{
			name: "description changed",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:        "my-sg",
				Description: pointer.To("new desc"),
			},
			observed: &group.SecurityGroup{
				Name:        "my-sg",
				Description: "old desc",
			},
			wantNeedsUpdate: true,
			wantDesc:        "new desc",
		},
		{
			name: "both changed",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:        "new-name",
				Description: pointer.To("new desc"),
			},
			observed: &group.SecurityGroup{
				Name:        "old-name",
				Description: "old desc",
			},
			wantNeedsUpdate: true,
			wantName:        "new-name",
			wantDesc:        "new desc",
		},
		{
			name: "nil description is no-op",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
			},
			observed: &group.SecurityGroup{
				Name:        "my-sg",
				Description: "existing desc",
			},
			wantNeedsUpdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, needsUpdate := buildSecurityGroupUpdateOpts(tt.spec, tt.observed)
			if needsUpdate != tt.wantNeedsUpdate {
				t.Errorf("needsUpdate = %v, want %v", needsUpdate, tt.wantNeedsUpdate)
			}
			if needsUpdate {
				if tt.wantName != "" && opts.SecurityGroup.Name != tt.wantName {
					t.Errorf("Name = %v, want %v", opts.SecurityGroup.Name, tt.wantName)
				}
				if tt.wantDesc != "" && opts.SecurityGroup.Description != tt.wantDesc {
					t.Errorf(
						"Description = %v, want %v",
						opts.SecurityGroup.Description,
						tt.wantDesc,
					)
				}
			}
		})
	}
}

func TestValidateImmutableSecurityGroupFields(t *testing.T) {
	tests := []struct {
		name     string
		spec     networkv1alpha1.SecurityGroupParameters
		observed *group.SecurityGroup
		wantErr  bool
	}{
		{
			name: "all unchanged",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				EnterpriseProjectID: pointer.To("0"),
				Tags:                map[string]string{"env": "dev"},
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				EnterpriseProjectID: "0",
				Tags:                []tags.ResourceTag{{Key: "env", Value: "dev"}},
			},
			wantErr: false,
		},
		{
			name: "nil optional fields are ok",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				EnterpriseProjectID: "0",
				Tags:                []tags.ResourceTag{{Key: "env", Value: "dev"}},
			},
			wantErr: false,
		},
		{
			name: "enterprise project ID changed",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				EnterpriseProjectID: pointer.To("proj-new"),
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				EnterpriseProjectID: "proj-old",
			},
			wantErr: true,
		},
		{
			name: "tags changed",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
				Tags: map[string]string{"env": "prod"},
			},
			observed: &group.SecurityGroup{
				Name: "my-sg",
				Tags: []tags.ResourceTag{{Key: "env", Value: "dev"}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImmutableSecurityGroupFields(tt.spec, tt.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"validateImmutableSecurityGroupFields() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestLateInitializeSecurityGroup(t *testing.T) {
	tests := []struct {
		name        string
		spec        networkv1alpha1.SecurityGroupParameters
		observed    *group.SecurityGroup
		tags        map[string]string
		wantChanged bool
		wantDesc    *string
		wantEPID    *string
		wantTags    map[string]string
	}{
		{
			name: "unset fields get late-initialized",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				Description:         "auto desc",
				EnterpriseProjectID: "0",
			},
			tags:        map[string]string{"env": "dev"},
			wantChanged: true,
			wantDesc:    pointer.To("auto desc"),
			wantEPID:    pointer.To("0"),
			wantTags:    map[string]string{"env": "dev"},
		},
		{
			name: "already set fields are not overwritten",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name:                "my-sg",
				Description:         pointer.To("my desc"),
				EnterpriseProjectID: pointer.To("proj-1"),
				Tags:                map[string]string{"env": "prod"},
			},
			observed: &group.SecurityGroup{
				Name:                "my-sg",
				Description:         "other desc",
				EnterpriseProjectID: "0",
			},
			tags:        map[string]string{"env": "dev"},
			wantChanged: false,
			wantDesc:    pointer.To("my desc"),
			wantEPID:    pointer.To("proj-1"),
			wantTags:    map[string]string{"env": "prod"},
		},
		{
			name: "empty observed values are not late-initialized",
			spec: networkv1alpha1.SecurityGroupParameters{
				Name: "my-sg",
			},
			observed: &group.SecurityGroup{
				Name: "my-sg",
			},
			tags:        nil,
			wantChanged: false,
			wantDesc:    nil,
			wantEPID:    nil,
			wantTags:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &networkv1alpha1.SecurityGroup{
				Spec: networkv1alpha1.SecurityGroupSpec{
					ForProvider: tt.spec,
				},
			}
			li := resource.NewLateInitializer()
			lateInitializeSecurityGroup(cr, tt.observed, tt.tags, li)

			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}
			p := cr.Spec.ForProvider
			if (p.Description == nil) != (tt.wantDesc == nil) {
				t.Errorf("Description = %v, want %v", p.Description, tt.wantDesc)
			} else if p.Description != nil && *p.Description != *tt.wantDesc {
				t.Errorf("Description = %v, want %v", *p.Description, *tt.wantDesc)
			}
			if (p.EnterpriseProjectID == nil) != (tt.wantEPID == nil) {
				t.Errorf("EnterpriseProjectID = %v, want %v", p.EnterpriseProjectID, tt.wantEPID)
			} else if p.EnterpriseProjectID != nil && *p.EnterpriseProjectID != *tt.wantEPID {
				t.Errorf("EnterpriseProjectID = %v, want %v", *p.EnterpriseProjectID, *tt.wantEPID)
			}
			if !maps.Equal(p.Tags, tt.wantTags) {
				t.Errorf("Tags = %v, want %v", p.Tags, tt.wantTags)
			}
		})
	}
}
