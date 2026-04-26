package ccecluster

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/cce/v3/clusters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ccev1alpha1 "go.wilaris.de/provider-opentelekomcloud/apis/cce/v1alpha1"
	"go.wilaris.de/provider-opentelekomcloud/internal/pointer"
)

func TestCreateSkipsWhenExternalNameSet(t *testing.T) {
	cr := &ccev1alpha1.Cluster{}
	meta.SetExternalName(cr, "existing-cluster")

	if _, err := (&external{}).Create(context.Background(), cr); err != nil {
		t.Fatalf("Create() returned error for existing external-name: %v", err)
	}
}

func TestCCEClusterPollInterval(t *testing.T) {
	stableInterval := 5 * time.Minute
	fastInterval := 30 * time.Second

	now := metav1.Now()

	cases := map[string]struct {
		cluster *ccev1alpha1.Cluster
		want    time.Duration
	}{
		"EmptyStatus": {
			cluster: &ccev1alpha1.Cluster{},
			want:    fastInterval,
		},
		"Creating": {
			cluster: &ccev1alpha1.Cluster{
				Status: ccev1alpha1.ClusterStatus{
					AtProvider: ccev1alpha1.ClusterObservation{Status: "Creating"},
				},
			},
			want: fastInterval,
		},
		"Deleting": {
			cluster: &ccev1alpha1.Cluster{
				Status: ccev1alpha1.ClusterStatus{
					AtProvider: ccev1alpha1.ClusterObservation{Status: "Deleting"},
				},
			},
			want: fastInterval,
		},
		"Unavailable": {
			cluster: &ccev1alpha1.Cluster{
				Status: ccev1alpha1.ClusterStatus{
					AtProvider: ccev1alpha1.ClusterObservation{Status: "Unavailable"},
				},
			},
			want: fastInterval,
		},
		"Upgrading": {
			cluster: &ccev1alpha1.Cluster{
				Status: ccev1alpha1.ClusterStatus{
					AtProvider: ccev1alpha1.ClusterObservation{Status: "Upgrading"},
				},
			},
			want: fastInterval,
		},
		"Available": {
			cluster: &ccev1alpha1.Cluster{
				Status: ccev1alpha1.ClusterStatus{
					AtProvider: ccev1alpha1.ClusterObservation{Status: "Available"},
				},
			},
			want: stableInterval,
		},
		"DeletingObjectWithAvailableStatus": {
			cluster: &ccev1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
				Status: ccev1alpha1.ClusterStatus{
					AtProvider: ccev1alpha1.ClusterObservation{Status: "Available"},
				},
			},
			want: fastInterval,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := cceClusterPollInterval(tc.cluster, stableInterval)
			if got != tc.want {
				t.Errorf("cceClusterPollInterval() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestIsClusterUpToDate(t *testing.T) {
	cases := map[string]struct {
		spec               ccev1alpha1.ClusterParameters
		observed           *clusters.Clusters
		lastAppliedCfgHash string
		want               bool
	}{
		"UpToDate": {
			spec: ccev1alpha1.ClusterParameters{
				Description: pointer.To("test"),
			},
			observed: &clusters.Clusters{
				Spec: clusters.Spec{Description: "test"},
			},
			want: true,
		},
		"DescriptionChanged": {
			spec: ccev1alpha1.ClusterParameters{
				Description: pointer.To("new"),
			},
			observed: &clusters.Clusters{
				Spec: clusters.Spec{Description: "old"},
			},
			want: false,
		},
		"DescriptionNilIsUpToDate": {
			spec: ccev1alpha1.ClusterParameters{},
			observed: &clusters.Clusters{
				Spec: clusters.Spec{Description: "server-set"},
			},
			want: true,
		},
		"EIPWantedButMissing": {
			spec: ccev1alpha1.ClusterParameters{EIP: pointer.To("1.2.3.4")},
			observed: &clusters.Clusters{
				Status: clusters.Status{Endpoints: []clusters.Endpoints{{}}},
			},
			want: false,
		},
		"EIPUnwantedButPresent": {
			spec: ccev1alpha1.ClusterParameters{},
			observed: &clusters.Clusters{
				Status: clusters.Status{
					Endpoints: []clusters.Endpoints{{External: "https://1.2.3.4:5443"}},
				},
			},
			want: false,
		},
		"EIPSwapDetected": {
			spec: ccev1alpha1.ClusterParameters{EIP: pointer.To("9.9.9.9")},
			observed: &clusters.Clusters{
				Status: clusters.Status{
					Endpoints: []clusters.Endpoints{{External: "https://1.2.3.4:5443"}},
				},
			},
			want: false,
		},
		"EIPMatches": {
			spec: ccev1alpha1.ClusterParameters{EIP: pointer.To("1.2.3.4")},
			observed: &clusters.Clusters{
				Status: clusters.Status{
					Endpoints: []clusters.Endpoints{{External: "https://1.2.3.4:5443"}},
				},
			},
			want: true,
		},
		"ComponentConfigsMatchAnnotation": {
			spec: ccev1alpha1.ClusterParameters{
				ComponentConfigurations: []ccev1alpha1.ComponentConfiguration{{
					Name: "kube-apiserver",
					Configurations: []ccev1alpha1.ConfigurationItem{
						{Name: "mode", Value: "strict"},
					},
				}},
			},
			observed: &clusters.Clusters{},
			lastAppliedCfgHash: componentConfigsHash([]ccev1alpha1.ComponentConfiguration{{
				Name: "kube-apiserver",
				Configurations: []ccev1alpha1.ConfigurationItem{
					{Name: "mode", Value: "strict"},
				},
			}}),
			want: true,
		},
		"ComponentConfigsDifferFromAnnotation": {
			spec: ccev1alpha1.ClusterParameters{
				ComponentConfigurations: []ccev1alpha1.ComponentConfiguration{{
					Name: "kube-apiserver",
					Configurations: []ccev1alpha1.ConfigurationItem{
						{Name: "mode", Value: "strict"},
					},
				}},
			},
			observed:           &clusters.Clusters{},
			lastAppliedCfgHash: "stale-hash",
			want:               false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isClusterUpToDate(tc.spec, tc.observed, tc.lastAppliedCfgHash)
			if got != tc.want {
				t.Errorf("isClusterUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsEIPUpToDate(t *testing.T) {
	endpoint := func(ip string) *clusters.Clusters {
		return &clusters.Clusters{
			Status: clusters.Status{
				Endpoints: []clusters.Endpoints{{External: "https://" + ip + ":5443"}},
			},
		}
	}

	cases := map[string]struct {
		spec     ccev1alpha1.ClusterParameters
		observed *clusters.Clusters
		want     bool
	}{
		"NoneWantedNoneBound": {
			spec:     ccev1alpha1.ClusterParameters{},
			observed: &clusters.Clusters{},
			want:     true,
		},
		"WantedNotBound": {
			spec:     ccev1alpha1.ClusterParameters{EIP: pointer.To("1.2.3.4")},
			observed: &clusters.Clusters{},
			want:     false,
		},
		"NotWantedButBound": {
			spec:     ccev1alpha1.ClusterParameters{},
			observed: endpoint("1.2.3.4"),
			want:     false,
		},
		"BoundIPMatches": {
			spec:     ccev1alpha1.ClusterParameters{EIP: pointer.To("1.2.3.4")},
			observed: endpoint("1.2.3.4"),
			want:     true,
		},
		"BoundIPDiffersTriggersSwap": {
			spec:     ccev1alpha1.ClusterParameters{EIP: pointer.To("9.9.9.9")},
			observed: endpoint("1.2.3.4"),
			want:     false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := isEIPUpToDate(tc.spec, tc.observed)
			if got != tc.want {
				t.Errorf("isEIPUpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestComponentConfigsHash(t *testing.T) {
	cfgs := []ccev1alpha1.ComponentConfiguration{{
		Name: "kube-apiserver",
		Configurations: []ccev1alpha1.ConfigurationItem{
			{Name: "mode", Value: "strict"},
		},
	}}

	if got := componentConfigsHash(nil); got != "" {
		t.Errorf("nil input = %q, want empty", got)
	}
	if got := componentConfigsHash([]ccev1alpha1.ComponentConfiguration{}); got != "" {
		t.Errorf("empty slice = %q, want empty", got)
	}

	h1 := componentConfigsHash(cfgs)
	h2 := componentConfigsHash(cfgs)
	if h1 == "" || h1 != h2 {
		t.Errorf("hash not stable: %q vs %q", h1, h2)
	}

	mutated := []ccev1alpha1.ComponentConfiguration{{
		Name: "kube-apiserver",
		Configurations: []ccev1alpha1.ConfigurationItem{
			{Name: "mode", Value: "permissive"},
		},
	}}
	if componentConfigsHash(mutated) == h1 {
		t.Error("hash did not change on value mutation")
	}
}

func TestParseEndpointHost(t *testing.T) {
	cases := map[string]string{
		"":                       "",
		"https://1.2.3.4:5443":   "1.2.3.4",
		"https://example.com":    "example.com",
		"not a url":              "",
		"http://[::1]:8080/path": "::1",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := parseEndpointHost(in); got != want {
				t.Errorf("parseEndpointHost(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestBuildDeleteQueryParams(t *testing.T) {
	cases := map[string]struct {
		spec ccev1alpha1.ClusterParameters
		want clusters.DeleteQueryParams
	}{
		"Empty": {
			spec: ccev1alpha1.ClusterParameters{},
			want: clusters.DeleteQueryParams{},
		},
		"DirectParams": {
			spec: ccev1alpha1.ClusterParameters{
				DeleteEFS: pointer.To("true"),
				DeleteNet: pointer.To("try"),
			},
			want: clusters.DeleteQueryParams{
				DeleteEfs: "true",
				DeleteNet: "try",
			},
		},
		"DeleteAllStorageOverrides": {
			spec: ccev1alpha1.ClusterParameters{
				DeleteAllStorage: pointer.To("true"),
			},
			want: clusters.DeleteQueryParams{
				DeleteEfs: "true",
				DeleteEvs: "true",
				DeleteObs: "true",
				DeleteSfs: "true",
			},
		},
		"DeleteAllNetworkOverrides": {
			spec: ccev1alpha1.ClusterParameters{
				DeleteAllNetwork: pointer.To("true"),
			},
			want: clusters.DeleteQueryParams{
				DeleteNet: "true",
				DeleteENI: "true",
			},
		},
		"DeleteAllStorageTryPropagates": {
			spec: ccev1alpha1.ClusterParameters{
				DeleteAllStorage: pointer.To("try"),
			},
			want: clusters.DeleteQueryParams{
				DeleteEfs: "try",
				DeleteEvs: "try",
				DeleteObs: "try",
				DeleteSfs: "try",
			},
		},
		"DeleteAllNetworkFalseDoesNotOverride": {
			spec: ccev1alpha1.ClusterParameters{
				DeleteNet:        pointer.To("try"),
				DeleteAllNetwork: pointer.To("false"),
			},
			want: clusters.DeleteQueryParams{
				DeleteNet: "try",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := buildDeleteQueryParams(tc.spec)
			if got != tc.want {
				t.Errorf("buildDeleteQueryParams() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestApplyExtendParam(t *testing.T) {
	cases := map[string]struct {
		spec ccev1alpha1.ClusterParameters
		want map[string]string
	}{
		"Empty": {
			spec: ccev1alpha1.ClusterParameters{},
			want: nil,
		},
		"NoAddons": {
			spec: ccev1alpha1.ClusterParameters{NoAddons: pointer.To(true)},
			want: map[string]string{"alpha.installDefaultAddons": "false"},
		},
		"MultiAZ": {
			spec: ccev1alpha1.ClusterParameters{MultiAZ: pointer.To(true)},
			want: map[string]string{"clusterAZ": "multi_az"},
		},
		"EIP": {
			spec: ccev1alpha1.ClusterParameters{EIP: pointer.To("1.2.3.4")},
			want: map[string]string{"clusterExternalIP": "1.2.3.4"},
		},
		"UserExtendParamMerged": {
			spec: ccev1alpha1.ClusterParameters{
				ExtendParam: map[string]string{"kubeProxyMode": "ipvs"},
				MultiAZ:     pointer.To(true),
			},
			want: map[string]string{
				"kubeProxyMode": "ipvs",
				"clusterAZ":     "multi_az",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &clusters.Spec{}
			applyExtendParam(s, tc.spec)
			if len(s.ExtendParam) != len(tc.want) {
				t.Fatalf(
					"ExtendParam size = %d, want %d (got %v)",
					len(s.ExtendParam),
					len(tc.want),
					s.ExtendParam,
				)
			}
			for k, v := range tc.want {
				if s.ExtendParam[k] != v {
					t.Errorf("ExtendParam[%q] = %q, want %q", k, s.ExtendParam[k], v)
				}
			}
		})
	}
}

func TestApplyAuthenticationProxy(t *testing.T) {
	mode := "authenticating_proxy"
	spec := ccev1alpha1.ClusterParameters{
		AuthenticationMode: &mode,
		AuthenticatingProxy: &ccev1alpha1.AuthenticatingProxySpec{
			CA:         "raw-ca-pem",
			Cert:       "raw-cert-pem",
			PrivateKey: "raw-key-pem",
		},
	}
	s := &clusters.Spec{}
	applyAuthentication(s, spec)

	got := s.Authentication.AuthenticatingProxy
	for _, key := range []string{"ca", "cert", "privateKey"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing key %q in payload: %v", key, got)
		}
	}

	if _, exists := got["key"]; exists {
		t.Errorf("payload must not carry legacy %q field", "key")
	}

	decoded, err := base64.StdEncoding.DecodeString(got["privateKey"])
	if err != nil {
		t.Fatalf("privateKey not base64-encoded: %v", err)
	}
	if string(decoded) != "raw-key-pem" {
		t.Errorf("privateKey decoded = %q, want %q", decoded, "raw-key-pem")
	}
}

func TestSetConditions(t *testing.T) {
	cases := map[string]xpv1.ConditionReason{
		"Available":   xpv1.ReasonAvailable,
		"Creating":    xpv1.ReasonCreating,
		"Deleting":    xpv1.ReasonDeleting,
		"Upgrading":   xpv1.ReasonUnavailable,
		"Unavailable": xpv1.ReasonUnavailable,
	}

	e := &external{}
	for phase, wantReason := range cases {
		t.Run(phase, func(t *testing.T) {
			cr := &ccev1alpha1.Cluster{}
			e.setConditions(cr, phase)
			got := cr.Status.GetCondition(xpv1.TypeReady)
			if got.Reason != wantReason {
				t.Errorf("phase %q → reason %q, want %q", phase, got.Reason, wantReason)
			}
		})
	}
}

func TestValidateClusterSpec(t *testing.T) {
	cases := map[string]struct {
		spec    ccev1alpha1.ClusterParameters
		wantErr bool
	}{
		"Valid": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-123"),
			},
			wantErr: false,
		},
		"MissingVPCID": {
			spec: ccev1alpha1.ClusterParameters{
				SubnetID: pointer.To("subnet-123"),
			},
			wantErr: true,
		},
		"MissingSubnetID": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID: pointer.To("vpc-123"),
			},
			wantErr: true,
		},
		"EmptyVPCID": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID:    pointer.To(""),
				SubnetID: pointer.To("subnet-123"),
			},
			wantErr: true,
		},
		"S1FlavorRejectsThreeMasters": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-123"),
				FlavorID: "cce.s1.small",
				Masters: []ccev1alpha1.MasterSpec{
					{AvailabilityZone: "eu-de-01"},
					{AvailabilityZone: "eu-de-02"},
					{AvailabilityZone: "eu-de-03"},
				},
			},
			wantErr: true,
		},
		"S2FlavorRequiresThreeMasters": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-123"),
				FlavorID: "cce.s2.medium",
				Masters:  []ccev1alpha1.MasterSpec{{AvailabilityZone: "eu-de-01"}},
			},
			wantErr: true,
		},
		"S2FlavorAcceptsThreeMasters": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-123"),
				FlavorID: "cce.s2.medium",
				Masters: []ccev1alpha1.MasterSpec{
					{AvailabilityZone: "eu-de-01"},
					{AvailabilityZone: "eu-de-02"},
					{AvailabilityZone: "eu-de-03"},
				},
			},
			wantErr: false,
		},
		"NoMastersIsAllowed": {
			spec: ccev1alpha1.ClusterParameters{
				VPCID:    pointer.To("vpc-123"),
				SubnetID: pointer.To("subnet-123"),
				FlavorID: "cce.s2.medium",
			},
			wantErr: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateClusterSpec(tc.spec)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateClusterSpec() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
