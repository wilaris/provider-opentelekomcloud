package securitygrouprule

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/vpc/v3/security/rules"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestCreateSkipsWhenExternalNameSet(t *testing.T) {
	cr := &networkv1alpha1.SecurityGroupRule{}
	meta.SetExternalName(cr, "existing-security-group-rule")

	if _, err := (&external{}).Create(context.Background(), cr); err != nil {
		t.Fatalf("Create() returned error for existing external-name: %v", err)
	}
}

func TestIsSecurityGroupRuleUpToDate(t *testing.T) {
	tests := []struct {
		name     string
		spec     networkv1alpha1.SecurityGroupRuleParameters
		observed *rules.SecurityGroupRule
		want     bool
	}{
		{
			name: "fully up to date",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Description:     pointer.To("allow http"),
				EtherType:       pointer.To("IPv4"),
				Protocol:        pointer.To("tcp"),
				MultiPort:       pointer.To("80"),
				RemoteIPPrefix:  pointer.To("0.0.0.0/0"),
				Action:          pointer.To("allow"),
				Priority:        pointer.To(1),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Description:     "allow http",
				Ethertype:       "IPv4",
				Protocol:        "tcp",
				Multiport:       "80",
				RemoteIPPrefix:  "0.0.0.0/0",
				Action:          "allow",
				Priority:        1,
			},
			want: true,
		},
		{
			name: "nil optional fields are up to date",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Ethertype:       "IPv4",
				Action:          "deny",
				Priority:        50,
			},
			want: true,
		},
		{
			name: "direction mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "egress",
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
			},
			want: false,
		},
		{
			name: "security group ID mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-new"),
				Direction:       "ingress",
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-old",
				Direction:       "ingress",
			},
			want: false,
		},
		{
			name: "description mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Description:     pointer.To("new desc"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Description:     "old desc",
			},
			want: false,
		},
		{
			name: "protocol mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Protocol:        pointer.To("udp"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Protocol:        "tcp",
			},
			want: false,
		},
		{
			name: "ether type mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				EtherType:       pointer.To("IPv6"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Ethertype:       "IPv4",
			},
			want: false,
		},
		{
			name: "multi port mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				MultiPort:       pointer.To("443"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Multiport:       "80",
			},
			want: false,
		},
		{
			name: "remote group ID mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				RemoteGroupID:   pointer.To("sg-other-new"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				RemoteGroupID:   "sg-other-old",
			},
			want: false,
		},
		{
			name: "remote IP prefix mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				RemoteIPPrefix:  pointer.To("10.0.0.0/8"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				RemoteIPPrefix:  "0.0.0.0/0",
			},
			want: false,
		},
		{
			name: "action mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Action:          pointer.To("allow"),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Action:          "deny",
			},
			want: false,
		},
		{
			name: "priority mismatch",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Priority:        pointer.To(1),
			},
			observed: &rules.SecurityGroupRule{
				SecurityGroupID: "sg-123",
				Direction:       "ingress",
				Priority:        50,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecurityGroupRuleUpToDate(tt.spec, tt.observed)
			if got != tt.want {
				t.Errorf("isSecurityGroupRuleUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec networkv1alpha1.SecurityGroupRuleParameters
		want rules.CreateOpts
	}{
		{
			name: "minimal required fields",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
			},
			want: rules.CreateOpts{
				SecurityGroupRule: rules.SecurityGroupRuleOptions{
					SecurityGroupID: "sg-123",
					Direction:       "ingress",
				},
			},
		},
		{
			name: "all fields set",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "egress",
				Description:     pointer.To("allow https"),
				EtherType:       pointer.To("IPv4"),
				Protocol:        pointer.To("tcp"),
				MultiPort:       pointer.To("443"),
				RemoteIPPrefix:  pointer.To("10.0.0.0/8"),
				Action:          pointer.To("allow"),
				Priority:        pointer.To(1),
			},
			want: rules.CreateOpts{
				SecurityGroupRule: rules.SecurityGroupRuleOptions{
					SecurityGroupID: "sg-123",
					Direction:       "egress",
					Description:     "allow https",
					Ethertype:       "IPv4",
					Protocol:        "tcp",
					Multiport:       "443",
					RemoteIPPrefix:  "10.0.0.0/8",
					Action:          "allow",
					Priority:        1,
				},
			},
		},
		{
			name: "with remote group ID",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Protocol:        pointer.To("tcp"),
				MultiPort:       pointer.To("22"),
				RemoteGroupID:   pointer.To("sg-bastion"),
			},
			want: rules.CreateOpts{
				SecurityGroupRule: rules.SecurityGroupRuleOptions{
					SecurityGroupID: "sg-123",
					Direction:       "ingress",
					Protocol:        "tcp",
					Multiport:       "22",
					RemoteGroupID:   "sg-bastion",
				},
			},
		},
		{
			name: "nil security group ID defaults to empty",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				Direction: "ingress",
			},
			want: rules.CreateOpts{
				SecurityGroupRule: rules.SecurityGroupRuleOptions{
					SecurityGroupID: "",
					Direction:       "ingress",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCreateOpts(tt.spec)
			r := got.SecurityGroupRule
			w := tt.want.SecurityGroupRule
			if r.SecurityGroupID != w.SecurityGroupID {
				t.Errorf("SecurityGroupID = %v, want %v", r.SecurityGroupID, w.SecurityGroupID)
			}
			if r.Direction != w.Direction {
				t.Errorf("Direction = %v, want %v", r.Direction, w.Direction)
			}
			if r.Description != w.Description {
				t.Errorf("Description = %v, want %v", r.Description, w.Description)
			}
			if r.Ethertype != w.Ethertype {
				t.Errorf("Ethertype = %v, want %v", r.Ethertype, w.Ethertype)
			}
			if r.Protocol != w.Protocol {
				t.Errorf("Protocol = %v, want %v", r.Protocol, w.Protocol)
			}
			if r.Multiport != w.Multiport {
				t.Errorf("Multiport = %v, want %v", r.Multiport, w.Multiport)
			}
			if r.RemoteIPPrefix != w.RemoteIPPrefix {
				t.Errorf("RemoteIPPrefix = %v, want %v", r.RemoteIPPrefix, w.RemoteIPPrefix)
			}
			if r.RemoteGroupID != w.RemoteGroupID {
				t.Errorf("RemoteGroupID = %v, want %v", r.RemoteGroupID, w.RemoteGroupID)
			}
			if r.Action != w.Action {
				t.Errorf("Action = %v, want %v", r.Action, w.Action)
			}
			if r.Priority != w.Priority {
				t.Errorf("Priority = %v, want %v", r.Priority, w.Priority)
			}
		})
	}
}

func TestLateInitializeSecurityGroupRule(t *testing.T) {
	tests := []struct {
		name             string
		spec             networkv1alpha1.SecurityGroupRuleParameters
		observed         *rules.SecurityGroupRule
		wantChanged      bool
		wantDescription  *string
		wantEtherType    *string
		wantProtocol     *string
		wantMultiPort    *string
		wantRemoteGroup  *string
		wantRemotePrefix *string
		wantAction       *string
		wantPriority     *int
	}{
		{
			name: "unset fields get late-initialized",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
			},
			observed: &rules.SecurityGroupRule{
				Description:    "auto desc",
				Ethertype:      "IPv4",
				Protocol:       "tcp",
				Multiport:      "80",
				RemoteIPPrefix: "0.0.0.0/0",
				Action:         "allow",
				Priority:       1,
			},
			wantChanged:      true,
			wantDescription:  pointer.To("auto desc"),
			wantEtherType:    pointer.To("IPv4"),
			wantProtocol:     pointer.To("tcp"),
			wantMultiPort:    pointer.To("80"),
			wantRemoteGroup:  nil,
			wantRemotePrefix: pointer.To("0.0.0.0/0"),
			wantAction:       pointer.To("allow"),
			wantPriority:     pointer.To(1),
		},
		{
			name: "already set fields are not overwritten",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
				Description:     pointer.To("my desc"),
				EtherType:       pointer.To("IPv6"),
				Protocol:        pointer.To("udp"),
				MultiPort:       pointer.To("443"),
				RemoteIPPrefix:  pointer.To("10.0.0.0/8"),
				Action:          pointer.To("deny"),
				Priority:        pointer.To(50),
			},
			observed: &rules.SecurityGroupRule{
				Description:    "other desc",
				Ethertype:      "IPv4",
				Protocol:       "tcp",
				Multiport:      "80",
				RemoteIPPrefix: "0.0.0.0/0",
				Action:         "allow",
				Priority:       1,
			},
			wantChanged:      false,
			wantDescription:  pointer.To("my desc"),
			wantEtherType:    pointer.To("IPv6"),
			wantProtocol:     pointer.To("udp"),
			wantMultiPort:    pointer.To("443"),
			wantRemoteGroup:  nil,
			wantRemotePrefix: pointer.To("10.0.0.0/8"),
			wantAction:       pointer.To("deny"),
			wantPriority:     pointer.To(50),
		},
		{
			name: "zero observed values are not late-initialized",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
			},
			observed: &rules.SecurityGroupRule{
				// All string fields empty, priority 0
			},
			wantChanged:      false,
			wantDescription:  nil,
			wantEtherType:    nil,
			wantProtocol:     nil,
			wantMultiPort:    nil,
			wantRemoteGroup:  nil,
			wantRemotePrefix: nil,
			wantAction:       nil,
			wantPriority:     nil,
		},
		{
			name: "remote group ID gets late-initialized",
			spec: networkv1alpha1.SecurityGroupRuleParameters{
				SecurityGroupID: pointer.To("sg-123"),
				Direction:       "ingress",
			},
			observed: &rules.SecurityGroupRule{
				RemoteGroupID: "sg-bastion",
				Ethertype:     "IPv4",
			},
			wantChanged:      true,
			wantDescription:  nil,
			wantEtherType:    pointer.To("IPv4"),
			wantProtocol:     nil,
			wantMultiPort:    nil,
			wantRemoteGroup:  pointer.To("sg-bastion"),
			wantRemotePrefix: nil,
			wantAction:       nil,
			wantPriority:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &networkv1alpha1.SecurityGroupRule{
				Spec: networkv1alpha1.SecurityGroupRuleSpec{
					ForProvider: tt.spec,
				},
			}
			li := resource.NewLateInitializer()
			lateInitializeSecurityGroupRule(cr, tt.observed, li)

			if li.IsChanged() != tt.wantChanged {
				t.Errorf("IsChanged() = %v, want %v", li.IsChanged(), tt.wantChanged)
			}

			p := cr.Spec.ForProvider
			assertPtrEqual(t, "Description", p.Description, tt.wantDescription)
			assertPtrEqual(t, "EtherType", p.EtherType, tt.wantEtherType)
			assertPtrEqual(t, "Protocol", p.Protocol, tt.wantProtocol)
			assertPtrEqual(t, "MultiPort", p.MultiPort, tt.wantMultiPort)
			assertPtrEqual(t, "RemoteGroupID", p.RemoteGroupID, tt.wantRemoteGroup)
			assertPtrEqual(t, "RemoteIPPrefix", p.RemoteIPPrefix, tt.wantRemotePrefix)
			assertPtrEqual(t, "Action", p.Action, tt.wantAction)
			assertPtrEqual(t, "Priority", p.Priority, tt.wantPriority)
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
