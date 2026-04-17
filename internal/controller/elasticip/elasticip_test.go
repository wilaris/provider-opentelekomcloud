package elasticip

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/bandwidths"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/eips"

	networkv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/network/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestBuildCreateOpts(t *testing.T) {
	tests := []struct {
		name          string
		spec          networkv1alpha1.ElasticIPParameters
		wantIPType    string
		wantShareType string
		wantBWName    string
		wantBWSize    int
		wantAddress   string
		wantAlias     string
	}{
		{
			name: "minimal parameters with enum mapping",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type: "BGP",
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-1",
					Size:      100,
					ShareType: "Dedicated",
				},
			},
			wantIPType:    "5_bgp",
			wantShareType: "PER",
			wantBWName:    "bw-1",
			wantBWSize:    100,
		},
		{
			name: "with optional public IP fields",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:      "BGP",
					IPAddress: pointer.To("80.158.1.100"),
					Name:      pointer.To("my-eip"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:       "bw-1",
					Size:       10,
					ShareType:  "Dedicated",
					ChargeMode: pointer.To("traffic"),
				},
			},
			wantIPType:    "5_bgp",
			wantShareType: "PER",
			wantBWName:    "bw-1",
			wantBWSize:    10,
			wantAddress:   "80.158.1.100",
			wantAlias:     "my-eip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildCreateOpts(tt.spec)
			if opts.IP.Type != tt.wantIPType {
				t.Errorf("buildCreateOpts() IP.Type = %q, want %q", opts.IP.Type, tt.wantIPType)
			}
			if opts.Bandwidth.ShareType != tt.wantShareType {
				t.Errorf(
					"buildCreateOpts() Bandwidth.ShareType = %q, want %q",
					opts.Bandwidth.ShareType,
					tt.wantShareType,
				)
			}
			if opts.Bandwidth.Name != tt.wantBWName {
				t.Errorf(
					"buildCreateOpts() Bandwidth.Name = %q, want %q",
					opts.Bandwidth.Name,
					tt.wantBWName,
				)
			}
			if opts.Bandwidth.Size != tt.wantBWSize {
				t.Errorf(
					"buildCreateOpts() Bandwidth.Size = %d, want %d",
					opts.Bandwidth.Size,
					tt.wantBWSize,
				)
			}
			if opts.IP.Address != tt.wantAddress {
				t.Errorf(
					"buildCreateOpts() IP.Address = %q, want %q",
					opts.IP.Address,
					tt.wantAddress,
				)
			}
			if opts.IP.Name != tt.wantAlias {
				t.Errorf("buildCreateOpts() IP.Name = %q, want %q", opts.IP.Name, tt.wantAlias)
			}
		})
	}
}

func TestBuildBandwidthUpdateOpts(t *testing.T) {
	tests := []struct {
		name       string
		spec       networkv1alpha1.ElasticIPBandwidthParameters
		observed   *bandwidths.BandWidth
		wantUpdate bool
		wantName   string
		wantSize   int
	}{
		{
			name: "no changes needed",
			spec: networkv1alpha1.ElasticIPBandwidthParameters{
				Name:      "bw-1",
				Size:      100,
				ShareType: "Dedicated",
			},
			observed:   &bandwidths.BandWidth{Name: "bw-1", Size: 100},
			wantUpdate: false,
		},
		{
			name: "name changed",
			spec: networkv1alpha1.ElasticIPBandwidthParameters{
				Name:      "bw-2",
				Size:      100,
				ShareType: "Dedicated",
			},
			observed:   &bandwidths.BandWidth{Name: "bw-1", Size: 100},
			wantUpdate: true,
			wantName:   "bw-2",
			wantSize:   0,
		},
		{
			name: "size changed",
			spec: networkv1alpha1.ElasticIPBandwidthParameters{
				Name:      "bw-1",
				Size:      200,
				ShareType: "Dedicated",
			},
			observed:   &bandwidths.BandWidth{Name: "bw-1", Size: 100},
			wantUpdate: true,
			wantName:   "",
			wantSize:   200,
		},
		{
			name: "both changed",
			spec: networkv1alpha1.ElasticIPBandwidthParameters{
				Name:      "bw-2",
				Size:      200,
				ShareType: "Dedicated",
			},
			observed:   &bandwidths.BandWidth{Name: "bw-1", Size: 100},
			wantUpdate: true,
			wantName:   "bw-2",
			wantSize:   200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, needsUpdate := buildBandwidthUpdateOpts(tt.spec, tt.observed)
			if needsUpdate != tt.wantUpdate {
				t.Errorf(
					"buildBandwidthUpdateOpts() needsUpdate = %v, want %v",
					needsUpdate,
					tt.wantUpdate,
				)
			}
			if opts.Name != tt.wantName {
				t.Errorf("buildBandwidthUpdateOpts() Name = %q, want %q", opts.Name, tt.wantName)
			}
			if opts.Size != tt.wantSize {
				t.Errorf("buildBandwidthUpdateOpts() Size = %d, want %d", opts.Size, tt.wantSize)
			}
		})
	}
}

func TestIsElasticIPUpToDate(t *testing.T) {
	baseEIP := &eips.PublicIp{
		Type:   "5_bgp",
		PortID: "port-1",
		Name:   "my-eip",
	}
	baseBW := &bandwidths.BandWidth{
		Name:       "bw-1",
		Size:       100,
		ShareType:  "PER",
		ChargeMode: "traffic",
	}
	baseTags := map[string]string{"env": "dev"}

	tests := []struct {
		name         string
		spec         networkv1alpha1.ElasticIPParameters
		observedEIP  *eips.PublicIp
		observedBW   *bandwidths.BandWidth
		observedTags map[string]string
		want         bool
	}{
		{
			name: "fully up to date",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:   "BGP",
					PortID: pointer.To("port-1"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:       "bw-1",
					Size:       100,
					ShareType:  "Dedicated",
					ChargeMode: pointer.To("traffic"),
				},
				Tags: map[string]string{"env": "dev"},
			},
			observedEIP:  baseEIP,
			observedBW:   baseBW,
			observedTags: baseTags,
			want:         true,
		},
		{
			name: "bandwidth name mismatch",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:   "BGP",
					PortID: pointer.To("port-1"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-new",
					Size:      100,
					ShareType: "Dedicated",
				},
				Tags: map[string]string{"env": "dev"},
			},
			observedEIP:  baseEIP,
			observedBW:   baseBW,
			observedTags: baseTags,
			want:         false,
		},
		{
			name: "bandwidth size mismatch",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:   "BGP",
					PortID: pointer.To("port-1"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-1",
					Size:      200,
					ShareType: "Dedicated",
				},
				Tags: map[string]string{"env": "dev"},
			},
			observedEIP:  baseEIP,
			observedBW:   baseBW,
			observedTags: baseTags,
			want:         false,
		},
		{
			name: "port ID mismatch",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:   "BGP",
					PortID: pointer.To("port-2"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-1",
					Size:      100,
					ShareType: "Dedicated",
				},
				Tags: map[string]string{"env": "dev"},
			},
			observedEIP:  baseEIP,
			observedBW:   baseBW,
			observedTags: baseTags,
			want:         false,
		},
		{
			name: "tags mismatch",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:   "BGP",
					PortID: pointer.To("port-1"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-1",
					Size:      100,
					ShareType: "Dedicated",
				},
				Tags: map[string]string{"env": "prod"},
			},
			observedEIP:  baseEIP,
			observedBW:   baseBW,
			observedTags: baseTags,
			want:         false,
		},
		{
			name: "nil optional fields are up to date",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type: "BGP",
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-1",
					Size:      100,
					ShareType: "Dedicated",
				},
			},
			observedEIP:  baseEIP,
			observedBW:   baseBW,
			observedTags: baseTags,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isElasticIPUpToDate(tt.spec, tt.observedEIP, tt.observedBW, tt.observedTags)
			if got != tt.want {
				t.Errorf("isElasticIPUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateImmutableFields(t *testing.T) {
	tests := []struct {
		name       string
		spec       networkv1alpha1.ElasticIPParameters
		observedIP *eips.PublicIp
		observedBW *bandwidths.BandWidth
		wantErr    bool
	}{
		{
			name: "all immutable fields unchanged",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:      "BGP",
					IPAddress: pointer.To("80.158.1.100"),
					Name:      pointer.To("my-eip"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:       "bw-1",
					Size:       100,
					ShareType:  "Dedicated",
					ChargeMode: pointer.To("traffic"),
				},
			},
			observedIP: &eips.PublicIp{
				Type:          "5_bgp",
				PublicAddress: "80.158.1.100",
				Name:          "my-eip",
			},
			observedBW: &bandwidths.BandWidth{ShareType: "PER", ChargeMode: "traffic"},
			wantErr:    false,
		},
		{
			name: "type changed",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{Type: "BGP"},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name: "bw-1", Size: 100, ShareType: "Dedicated",
				},
			},
			observedIP: &eips.PublicIp{Type: "5_sbgp"},
			observedBW: &bandwidths.BandWidth{ShareType: "PER"},
			wantErr:    true,
		},
		{
			name: "ip address changed",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:      "BGP",
					IPAddress: pointer.To("80.158.1.200"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name: "bw-1", Size: 100, ShareType: "Dedicated",
				},
			},
			observedIP: &eips.PublicIp{Type: "5_bgp", PublicAddress: "80.158.1.100"},
			observedBW: &bandwidths.BandWidth{ShareType: "PER"},
			wantErr:    true,
		},
		{
			name: "name changed",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type: "BGP",
					Name: pointer.To("new-name"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name: "bw-1", Size: 100, ShareType: "Dedicated",
				},
			},
			observedIP: &eips.PublicIp{Type: "5_bgp", Name: "old-name"},
			observedBW: &bandwidths.BandWidth{ShareType: "PER"},
			wantErr:    true,
		},
		{
			name: "charge mode changed",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{Type: "BGP"},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name: "bw-1", Size: 100, ShareType: "Dedicated",
					ChargeMode: pointer.To("bandwidth"),
				},
			},
			observedIP: &eips.PublicIp{Type: "5_bgp"},
			observedBW: &bandwidths.BandWidth{ShareType: "PER", ChargeMode: "traffic"},
			wantErr:    true,
		},
		{
			name: "nil optional immutable fields are ok",
			spec: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{Type: "BGP"},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name: "bw-1", Size: 100, ShareType: "Dedicated",
				},
			},
			observedIP: &eips.PublicIp{
				Type:          "5_bgp",
				PublicAddress: "80.158.1.100",
				Name:          "my-eip",
			},
			observedBW: &bandwidths.BandWidth{ShareType: "PER", ChargeMode: "traffic"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImmutableFields(tt.spec, tt.observedIP, tt.observedBW)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateImmutableFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLateInitializeElasticIP(t *testing.T) {
	cr := &networkv1alpha1.ElasticIP{
		Spec: networkv1alpha1.ElasticIPSpec{
			ForProvider: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type: "BGP",
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:      "bw-1",
					Size:      100,
					ShareType: "Dedicated",
				},
			},
		},
	}

	observed := &eips.PublicIp{
		PublicAddress: "80.158.1.100",
		Name:          "my-eip",
		PortID:        "port-1",
	}
	observedBW := &bandwidths.BandWidth{
		ChargeMode: "traffic",
	}
	observedTags := map[string]string{"env": "dev"}

	li := resource.NewLateInitializer()
	lateInitializeElasticIP(cr, observed, observedBW, observedTags, li)

	if !li.IsChanged() {
		t.Fatal("lateInitializeElasticIP() did not mark resource as changed")
	}
	if cr.Spec.ForProvider.PublicIP.IPAddress == nil ||
		*cr.Spec.ForProvider.PublicIP.IPAddress != "80.158.1.100" {
		t.Fatalf("ipAddress not late initialized: %#v", cr.Spec.ForProvider.PublicIP.IPAddress)
	}
	if cr.Spec.ForProvider.PublicIP.Name == nil ||
		*cr.Spec.ForProvider.PublicIP.Name != "my-eip" {
		t.Fatalf("name not late initialized: %#v", cr.Spec.ForProvider.PublicIP.Name)
	}
	if cr.Spec.ForProvider.PublicIP.PortID == nil ||
		*cr.Spec.ForProvider.PublicIP.PortID != "port-1" {
		t.Fatalf("portId not late initialized: %#v", cr.Spec.ForProvider.PublicIP.PortID)
	}
	if cr.Spec.ForProvider.Bandwidth.ChargeMode == nil ||
		*cr.Spec.ForProvider.Bandwidth.ChargeMode != "traffic" {
		t.Fatalf("chargeMode not late initialized: %#v", cr.Spec.ForProvider.Bandwidth.ChargeMode)
	}
	if len(cr.Spec.ForProvider.Tags) != 1 || cr.Spec.ForProvider.Tags["env"] != "dev" {
		t.Fatalf("tags not late initialized: %#v", cr.Spec.ForProvider.Tags)
	}
}

func TestLateInitializeElasticIPNoChange(t *testing.T) {
	cr := &networkv1alpha1.ElasticIP{
		Spec: networkv1alpha1.ElasticIPSpec{
			ForProvider: networkv1alpha1.ElasticIPParameters{
				PublicIP: networkv1alpha1.ElasticIPPublicIPParameters{
					Type:      "BGP",
					IPAddress: pointer.To("80.158.1.100"),
					Name:      pointer.To("my-eip"),
					PortID:    pointer.To("port-1"),
				},
				Bandwidth: networkv1alpha1.ElasticIPBandwidthParameters{
					Name:       "bw-1",
					Size:       100,
					ShareType:  "Dedicated",
					ChargeMode: pointer.To("traffic"),
				},
				Tags: map[string]string{"env": "dev"},
			},
		},
	}

	observed := &eips.PublicIp{
		PublicAddress: "80.158.1.200",
		Name:          "other-name",
		PortID:        "port-2",
	}
	observedBW := &bandwidths.BandWidth{
		ChargeMode: "bandwidth",
	}
	observedTags := map[string]string{"env": "prod"}

	li := resource.NewLateInitializer()
	lateInitializeElasticIP(cr, observed, observedBW, observedTags, li)

	if li.IsChanged() {
		t.Fatal(
			"lateInitializeElasticIP() should not mark as changed when all fields are already set",
		)
	}
	if *cr.Spec.ForProvider.PublicIP.IPAddress != "80.158.1.100" {
		t.Fatalf("ipAddress was overwritten: %q", *cr.Spec.ForProvider.PublicIP.IPAddress)
	}
	if *cr.Spec.ForProvider.Bandwidth.ChargeMode != "traffic" {
		t.Fatalf("chargeMode was overwritten: %q", *cr.Spec.ForProvider.Bandwidth.ChargeMode)
	}
}
