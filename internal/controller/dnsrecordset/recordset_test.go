package dnsrecordset

import (
	"slices"
	"testing"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/dns/v2/recordsets"

	dnsv1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/dns/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestGetZoneInfo(t *testing.T) {
	tests := []struct {
		name           string
		spec           dnsv1alpha1.RecordSetParameters
		wantZoneID     string
		wantTagService string
		wantErr        bool
	}{
		{
			name:           "private zone",
			spec:           dnsv1alpha1.RecordSetParameters{PrivateZoneID: pointer.To("zone-123")},
			wantZoneID:     "zone-123",
			wantTagService: "DNS-private_recordset",
		},
		{
			name:           "public zone",
			spec:           dnsv1alpha1.RecordSetParameters{PublicZoneID: pointer.To("zone-456")},
			wantZoneID:     "zone-456",
			wantTagService: "DNS-public_recordset",
		},
		{
			name:    "neither set",
			spec:    dnsv1alpha1.RecordSetParameters{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getZoneInfo(tc.spec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("getZoneInfo() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got.zoneID != tc.wantZoneID {
				t.Errorf("zoneID = %q, want %q", got.zoneID, tc.wantZoneID)
			}
			if got.tagServiceType != tc.wantTagService {
				t.Errorf("tagServiceType = %q, want %q", got.tagServiceType, tc.wantTagService)
			}
		})
	}
}

func TestQuoteTXTRecords(t *testing.T) {
	tests := []struct {
		name       string
		recordType string
		records    []string
		want       []string
	}{
		{
			name:       "TXT records get quoted",
			recordType: "TXT",
			records:    []string{"v=spf1 include:example.com ~all"},
			want:       []string{`"v=spf1 include:example.com ~all"`},
		},
		{
			name:       "A records unchanged",
			recordType: "A",
			records:    []string{"192.168.1.1"},
			want:       []string{"192.168.1.1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteTXTRecords(tc.recordType, tc.records)
			if !slices.Equal(got, tc.want) {
				t.Errorf("quoteTXTRecords() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTrimTXTQuotes(t *testing.T) {
	tests := []struct {
		name       string
		recordType string
		records    []string
		want       []string
	}{
		{
			name:       "TXT records with quotes trimmed",
			recordType: "TXT",
			records:    []string{`"v=spf1 include:example.com ~all"`},
			want:       []string{"v=spf1 include:example.com ~all"},
		},
		{
			name:       "TXT records without quotes unchanged",
			recordType: "TXT",
			records:    []string{"v=spf1"},
			want:       []string{"v=spf1"},
		},
		{
			name:       "A records unchanged",
			recordType: "A",
			records:    []string{"192.168.1.1"},
			want:       []string{"192.168.1.1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := trimTXTQuotes(tc.recordType, tc.records)
			if !slices.Equal(got, tc.want) {
				t.Errorf("trimTXTQuotes() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateRecordSetParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  dnsv1alpha1.RecordSetParameters
		wantErr bool
	}{
		{
			name: "valid",
			params: dnsv1alpha1.RecordSetParameters{
				Name:    "www.example.com.",
				Type:    "A",
				Records: []string{"192.168.1.1"},
			},
		},
		{
			name: "missing name",
			params: dnsv1alpha1.RecordSetParameters{
				Type:    "A",
				Records: []string{"192.168.1.1"},
			},
			wantErr: true,
		},
		{
			name: "missing type",
			params: dnsv1alpha1.RecordSetParameters{
				Name:    "www.example.com.",
				Records: []string{"192.168.1.1"},
			},
			wantErr: true,
		},
		{
			name: "empty records",
			params: dnsv1alpha1.RecordSetParameters{
				Name:    "www.example.com.",
				Type:    "A",
				Records: []string{},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRecordSetParameters(tc.params)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateRecordSetParameters() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIsRecordSetUpToDate(t *testing.T) {
	tests := []struct {
		name         string
		spec         dnsv1alpha1.RecordSetParameters
		observed     *recordsets.RecordSet
		observedTags map[string]string
		want         bool
	}{
		{
			name: "up to date",
			spec: dnsv1alpha1.RecordSetParameters{
				Records:     []string{"192.168.1.1", "192.168.1.2"},
				Description: pointer.To("test"),
				TTL:         pointer.To(300),
				Tags:        map[string]string{"env": "dev"},
			},
			observed: &recordsets.RecordSet{
				Records:     []string{"192.168.1.2", "192.168.1.1"},
				Description: "test",
				TTL:         300,
				Type:        "A",
			},
			observedTags: map[string]string{"env": "dev"},
			want:         true,
		},
		{
			name: "records mismatch",
			spec: dnsv1alpha1.RecordSetParameters{
				Records: []string{"192.168.1.1"},
			},
			observed: &recordsets.RecordSet{
				Records: []string{"192.168.1.2"},
				Type:    "A",
			},
			observedTags: map[string]string{},
			want:         false,
		},
		{
			name: "TTL mismatch",
			spec: dnsv1alpha1.RecordSetParameters{
				Records: []string{"192.168.1.1"},
				TTL:     pointer.To(600),
			},
			observed: &recordsets.RecordSet{
				Records: []string{"192.168.1.1"},
				TTL:     300,
				Type:    "A",
			},
			observedTags: map[string]string{},
			want:         false,
		},
		{
			name: "description mismatch",
			spec: dnsv1alpha1.RecordSetParameters{
				Records:     []string{"192.168.1.1"},
				Description: pointer.To("new"),
			},
			observed: &recordsets.RecordSet{
				Records:     []string{"192.168.1.1"},
				Description: "old",
				Type:        "A",
			},
			observedTags: map[string]string{},
			want:         false,
		},
		{
			name: "TXT records up to date with quotes",
			spec: dnsv1alpha1.RecordSetParameters{
				Records: []string{"v=spf1 ~all"},
			},
			observed: &recordsets.RecordSet{
				Records: []string{`"v=spf1 ~all"`},
				Type:    "TXT",
			},
			observedTags: map[string]string{},
			want:         true,
		},
		{
			name: "nil optional fields up to date",
			spec: dnsv1alpha1.RecordSetParameters{
				Records: []string{"192.168.1.1"},
			},
			observed: &recordsets.RecordSet{
				Records: []string{"192.168.1.1"},
				Type:    "A",
			},
			observedTags: map[string]string{},
			want:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRecordSetUpToDate(tc.spec, tc.observed, tc.observedTags)
			if got != tc.want {
				t.Errorf("isRecordSetUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildRecordSetCreateOpts(t *testing.T) {
	tests := []struct {
		name string
		spec dnsv1alpha1.RecordSetParameters
		want recordsets.CreateOpts
	}{
		{
			name: "minimal",
			spec: dnsv1alpha1.RecordSetParameters{
				Name:    "www.example.com.",
				Type:    "A",
				Records: []string{"192.168.1.1"},
			},
			want: recordsets.CreateOpts{
				Name:    "www.example.com.",
				Type:    "A",
				Records: []string{"192.168.1.1"},
			},
		},
		{
			name: "all fields",
			spec: dnsv1alpha1.RecordSetParameters{
				Name:        "www.example.com.",
				Type:        "A",
				Records:     []string{"192.168.1.1"},
				Description: pointer.To("test record"),
				TTL:         pointer.To(600),
			},
			want: recordsets.CreateOpts{
				Name:        "www.example.com.",
				Type:        "A",
				Records:     []string{"192.168.1.1"},
				Description: "test record",
				TTL:         600,
			},
		},
		{
			name: "TXT record quoted",
			spec: dnsv1alpha1.RecordSetParameters{
				Name:    "example.com.",
				Type:    "TXT",
				Records: []string{"v=spf1 ~all"},
			},
			want: recordsets.CreateOpts{
				Name:    "example.com.",
				Type:    "TXT",
				Records: []string{`"v=spf1 ~all"`},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRecordSetCreateOpts(tc.spec)
			if got.Name != tc.want.Name ||
				got.Type != tc.want.Type ||
				got.Description != tc.want.Description ||
				got.TTL != tc.want.TTL ||
				!slices.Equal(got.Records, tc.want.Records) {
				t.Errorf("buildRecordSetCreateOpts() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
