package subnet

import (
	"context"
	"testing"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/subnets"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestCreateSkipsWhenExternalNameSet(t *testing.T) {
	cr := &networkv1alpha1.Subnet{}
	meta.SetExternalName(cr, "existing-subnet")

	if _, err := (&external{}).Create(context.Background(), cr); err != nil {
		t.Fatalf("Create() returned error for existing external-name: %v", err)
	}
}

func TestValidateSubnetParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  networkv1alpha1.SubnetParameters
		wantErr bool
	}{
		{
			name: "valid minimal parameters",
			params: networkv1alpha1.SubnetParameters{
				Name:      "example",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
				VPCID:     pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
			},
			wantErr: false,
		},
		{
			name: "missing name",
			params: networkv1alpha1.SubnetParameters{
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
				VPCID:     pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
			},
			wantErr: true,
		},
		{
			name: "missing vpc id",
			params: networkv1alpha1.SubnetParameters{
				Name:      "example",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
			},
			wantErr: true,
		},
		{
			name: "invalid cidr",
			params: networkv1alpha1.SubnetParameters{
				Name:      "example",
				CIDR:      "invalid",
				GatewayIP: "10.0.0.1",
				VPCID:     pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
			},
			wantErr: true,
		},
		{
			name: "invalid gateway ip",
			params: networkv1alpha1.SubnetParameters{
				Name:      "example",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "invalid",
				VPCID:     pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
			},
			wantErr: true,
		},
		{
			name: "invalid primary dns",
			params: networkv1alpha1.SubnetParameters{
				Name:       "example",
				CIDR:       "10.0.0.0/24",
				GatewayIP:  "10.0.0.1",
				VPCID:      pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
				PrimaryDNS: pointer.To("not-an-ip"),
			},
			wantErr: true,
		},
		{
			name: "invalid dns list entry",
			params: networkv1alpha1.SubnetParameters{
				Name:      "example",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
				VPCID:     pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
				DNSList:   []string{"8.8.8.8", "invalid"},
			},
			wantErr: true,
		},
		{
			name: "valid with optionals",
			params: networkv1alpha1.SubnetParameters{
				Name:             "example",
				Description:      pointer.To("desc"),
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				DHCPEnable:       pointer.To(true),
				IPv6Enable:       pointer.To(false),
				PrimaryDNS:       pointer.To("100.125.4.25"),
				SecondaryDNS:     pointer.To("100.125.129.199"),
				AvailabilityZone: pointer.To("eu-de-01"),
				VPCID:            pointer.To("9d4ea4e3-bf95-4739-8bea-8f4f4f8e4f95"),
				DNSList:          []string{"100.125.4.25", "100.125.129.199"},
				NTPAddresses:     pointer.To("ntp.server.local"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubnetParameters(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSubnetParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDesiredDNSValues(t *testing.T) {
	tests := []struct {
		name          string
		spec          networkv1alpha1.SubnetParameters
		wantPrimary   string
		wantSecondary string
		wantDNSList   []string
	}{
		{
			name:          "empty when none specified",
			spec:          networkv1alpha1.SubnetParameters{},
			wantPrimary:   "",
			wantSecondary: "",
			wantDNSList:   nil,
		},
		{
			name: "explicit values",
			spec: networkv1alpha1.SubnetParameters{
				PrimaryDNS:   pointer.To("1.1.1.1"),
				SecondaryDNS: pointer.To("8.8.8.8"),
				DNSList:      []string{"9.9.9.9"},
			},
			wantPrimary:   "1.1.1.1",
			wantSecondary: "8.8.8.8",
			wantDNSList:   []string{"9.9.9.9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrimary, gotSecondary, gotDNSList := desiredDNSValues(tt.spec)
			if gotPrimary != tt.wantPrimary {
				t.Errorf("desiredDNSValues() primary = %q, want %q", gotPrimary, tt.wantPrimary)
			}
			if gotSecondary != tt.wantSecondary {
				t.Errorf(
					"desiredDNSValues() secondary = %q, want %q",
					gotSecondary,
					tt.wantSecondary,
				)
			}
			if len(gotDNSList) != len(tt.wantDNSList) {
				t.Errorf("desiredDNSValues() dnsList = %v, want %v", gotDNSList, tt.wantDNSList)
				return
			}
			for i := range gotDNSList {
				if gotDNSList[i] != tt.wantDNSList[i] {
					t.Errorf(
						"desiredDNSValues() dnsList[%d] = %q, want %q",
						i,
						gotDNSList[i],
						tt.wantDNSList[i],
					)
				}
			}
		})
	}
}

func TestIsSubnetUpToDate(t *testing.T) {
	baseObserved := &subnets.Subnet{
		Name:             "subnet-1",
		Description:      "desc",
		CIDR:             "10.0.0.0/24",
		GatewayIP:        "10.0.0.1",
		EnableDHCP:       true,
		EnableIpv6:       false,
		PrimaryDNS:       "100.125.4.25",
		SecondaryDNS:     "100.125.129.199",
		AvailabilityZone: "eu-de-01",
		VpcID:            "vpc-1",
		DNSList:          []string{"100.125.4.25", "100.125.129.199"},
	}
	baseNTP := "ntp.server.local"
	baseTags := map[string]string{"env": "dev"}

	tests := []struct {
		name         string
		spec         networkv1alpha1.SubnetParameters
		observed     *subnets.Subnet
		observedNTP  string
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date",
			spec: networkv1alpha1.SubnetParameters{
				Name:             "subnet-1",
				Description:      pointer.To("desc"),
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				DHCPEnable:       pointer.To(true),
				IPv6Enable:       pointer.To(false),
				PrimaryDNS:       pointer.To("100.125.4.25"),
				SecondaryDNS:     pointer.To("100.125.129.199"),
				AvailabilityZone: pointer.To("eu-de-01"),
				VPCID:            pointer.To("vpc-1"),
				DNSList:          []string{"100.125.4.25", "100.125.129.199"},
				NTPAddresses:     pointer.To("ntp.server.local"),
				Tags:             map[string]string{"env": "dev"},
			},
			observed:     baseObserved,
			observedNTP:  baseNTP,
			observedTags: baseTags,
			want:         true,
		},
		{
			name: "cidr mismatch",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.1.0/24",
				GatewayIP: "10.0.0.1",
			},
			observed:     baseObserved,
			observedNTP:  baseNTP,
			observedTags: baseTags,
			want:         false,
		},
		{
			name: "tags mismatch",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
				Tags:      map[string]string{"env": "prod"},
			},
			observed:     baseObserved,
			observedNTP:  baseNTP,
			observedTags: baseTags,
			want:         false,
		},
		{
			name: "nil optional fields are up to date",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
			},
			observed:     baseObserved,
			observedNTP:  baseNTP,
			observedTags: baseTags,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSubnetUpToDate(tt.spec, tt.observed, tt.observedNTP, tt.observedTags)
			if got != tt.want {
				t.Errorf("isSubnetUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractNTPAddress(t *testing.T) {
	tests := []struct {
		name string
		opts []subnets.ExtraDHCP
		want string
	}{
		{
			name: "no ntp option",
			opts: []subnets.ExtraDHCP{{OptName: "router", OptValue: "10.0.0.1"}},
			want: "",
		},
		{
			name: "ntp option present",
			opts: []subnets.ExtraDHCP{{OptName: "ntp", OptValue: "ntp.server.local"}},
			want: "ntp.server.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNTPAddress(tt.opts)
			if got != tt.want {
				t.Errorf("extractNTPAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLateInitializeSubnet(t *testing.T) {
	cr := &networkv1alpha1.Subnet{
		Spec: networkv1alpha1.SubnetSpec{
			ForProvider: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
			},
		},
	}

	observed := &subnets.Subnet{
		Description:      "desc",
		DNSList:          []string{"100.125.4.25", "100.125.129.199"},
		EnableDHCP:       true,
		EnableIpv6:       false,
		PrimaryDNS:       "100.125.4.25",
		SecondaryDNS:     "100.125.129.199",
		AvailabilityZone: "eu-de-01",
		VpcID:            "vpc-1",
	}
	observedTags := map[string]string{"env": "dev"}

	li := resource.NewLateInitializer()
	lateInitializeSubnet(cr, observed, "ntp.server.local", observedTags, li)

	if !li.IsChanged() {
		t.Fatal("lateInitializeSubnet() did not mark resource as changed")
	}
	if cr.Spec.ForProvider.Description == nil || *cr.Spec.ForProvider.Description != "desc" {
		t.Fatalf("description not late initialized: %#v", cr.Spec.ForProvider.Description)
	}
	if cr.Spec.ForProvider.DHCPEnable == nil || !*cr.Spec.ForProvider.DHCPEnable {
		t.Fatalf("dhcpEnable not late initialized: %#v", cr.Spec.ForProvider.DHCPEnable)
	}
	if cr.Spec.ForProvider.IPv6Enable == nil || *cr.Spec.ForProvider.IPv6Enable {
		t.Fatalf("ipv6Enable not late initialized: %#v", cr.Spec.ForProvider.IPv6Enable)
	}
	if cr.Spec.ForProvider.VPCID == nil || *cr.Spec.ForProvider.VPCID != "vpc-1" {
		t.Fatalf("vpcId not late initialized: %#v", cr.Spec.ForProvider.VPCID)
	}
	if cr.Spec.ForProvider.NTPAddresses == nil ||
		*cr.Spec.ForProvider.NTPAddresses != "ntp.server.local" {
		t.Fatalf("ntpAddresses not late initialized: %#v", cr.Spec.ForProvider.NTPAddresses)
	}
	if len(cr.Spec.ForProvider.Tags) != 1 || cr.Spec.ForProvider.Tags["env"] != "dev" {
		t.Fatalf("tags not late initialized: %#v", cr.Spec.ForProvider.Tags)
	}
}

func TestBuildSubnetUpdateOpts(t *testing.T) {
	tests := []struct {
		name        string
		spec        networkv1alpha1.SubnetParameters
		observed    *subnets.Subnet
		observedNTP string
		wantUpdate  bool
		wantName    string
	}{
		{
			name: "no changes needed",
			spec: networkv1alpha1.SubnetParameters{
				Name:        "subnet-1",
				CIDR:        "10.0.0.0/24",
				GatewayIP:   "10.0.0.1",
				Description: pointer.To("desc"),
				DHCPEnable:  pointer.To(true),
			},
			observed:    &subnets.Subnet{Name: "subnet-1", Description: "desc", EnableDHCP: true},
			observedNTP: "",
			wantUpdate:  false,
			wantName:    "subnet-1",
		},
		{
			name: "name changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-2",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
			},
			observed:   &subnets.Subnet{Name: "subnet-1"},
			wantUpdate: true,
			wantName:   "subnet-2",
		},
		{
			name: "description changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:        "subnet-1",
				CIDR:        "10.0.0.0/24",
				GatewayIP:   "10.0.0.1",
				Description: pointer.To("new desc"),
			},
			observed:   &subnets.Subnet{Name: "subnet-1", Description: "old desc"},
			wantUpdate: true,
			wantName:   "subnet-1",
		},
		{
			name: "DHCP changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:       "subnet-1",
				CIDR:       "10.0.0.0/24",
				GatewayIP:  "10.0.0.1",
				DHCPEnable: pointer.To(false),
			},
			observed:   &subnets.Subnet{Name: "subnet-1", EnableDHCP: true},
			wantUpdate: true,
			wantName:   "subnet-1",
		},
		{
			name: "NTP changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:         "subnet-1",
				CIDR:         "10.0.0.0/24",
				GatewayIP:    "10.0.0.1",
				NTPAddresses: pointer.To("ntp.new.local"),
			},
			observed:    &subnets.Subnet{Name: "subnet-1"},
			observedNTP: "ntp.old.local",
			wantUpdate:  true,
			wantName:    "subnet-1",
		},
		{
			name: "nil optional fields are no-op",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
			},
			observed: &subnets.Subnet{
				Name:        "subnet-1",
				Description: "desc",
				PrimaryDNS:  "1.1.1.1",
				EnableDHCP:  true,
			},
			observedNTP: "ntp.server.local",
			wantUpdate:  false,
			wantName:    "subnet-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, needsUpdate := buildSubnetUpdateOpts(tt.spec, tt.observed, tt.observedNTP)
			if needsUpdate != tt.wantUpdate {
				t.Errorf(
					"buildSubnetUpdateOpts() needsUpdate = %v, want %v",
					needsUpdate,
					tt.wantUpdate,
				)
			}
			if opts.Name != tt.wantName {
				t.Errorf("buildSubnetUpdateOpts() Name = %q, want %q", opts.Name, tt.wantName)
			}
		})
	}
}

func TestValidateImmutableSubnetFields(t *testing.T) {
	tests := []struct {
		name     string
		spec     networkv1alpha1.SubnetParameters
		observed *subnets.Subnet
		wantErr  bool
	}{
		{
			name: "all immutable fields unchanged",
			spec: networkv1alpha1.SubnetParameters{
				Name:             "subnet-1",
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				IPv6Enable:       pointer.To(false),
				AvailabilityZone: pointer.To("eu-de-01"),
				VPCID:            pointer.To("vpc-1"),
			},
			observed: &subnets.Subnet{
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				EnableIpv6:       false,
				AvailabilityZone: "eu-de-01",
				VpcID:            "vpc-1",
			},
			wantErr: false,
		},
		{
			name: "CIDR changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.1.0/24",
				GatewayIP: "10.0.0.1",
			},
			observed: &subnets.Subnet{CIDR: "10.0.0.0/24", GatewayIP: "10.0.0.1"},
			wantErr:  true,
		},
		{
			name: "gateway changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.2",
			},
			observed: &subnets.Subnet{CIDR: "10.0.0.0/24", GatewayIP: "10.0.0.1"},
			wantErr:  true,
		},
		{
			name: "IPv6 changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:       "subnet-1",
				CIDR:       "10.0.0.0/24",
				GatewayIP:  "10.0.0.1",
				IPv6Enable: pointer.To(true),
			},
			observed: &subnets.Subnet{
				CIDR:       "10.0.0.0/24",
				GatewayIP:  "10.0.0.1",
				EnableIpv6: false,
			},
			wantErr: true,
		},
		{
			name: "availability zone changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:             "subnet-1",
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				AvailabilityZone: pointer.To("eu-de-02"),
			},
			observed: &subnets.Subnet{
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				AvailabilityZone: "eu-de-01",
			},
			wantErr: true,
		},
		{
			name: "VPC ID changed",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
				VPCID:     pointer.To("vpc-2"),
			},
			observed: &subnets.Subnet{CIDR: "10.0.0.0/24", GatewayIP: "10.0.0.1", VpcID: "vpc-1"},
			wantErr:  true,
		},
		{
			name: "nil optional immutable fields are ok",
			spec: networkv1alpha1.SubnetParameters{
				Name:      "subnet-1",
				CIDR:      "10.0.0.0/24",
				GatewayIP: "10.0.0.1",
			},
			observed: &subnets.Subnet{
				CIDR:             "10.0.0.0/24",
				GatewayIP:        "10.0.0.1",
				EnableIpv6:       true,
				AvailabilityZone: "eu-de-01",
				VpcID:            "vpc-1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImmutableSubnetFields(tt.spec, tt.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImmutableSubnetFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
