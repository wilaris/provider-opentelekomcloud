package clients

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
)

func TestGetClient_ReusesCachedClientWhenHashMatches(t *testing.T) {
	previousNewClient := newClient
	previousAuthenticate := authenticate
	t.Cleanup(func() {
		newClient = previousNewClient
		authenticate = previousAuthenticate
	})

	authCalls := 0
	newClient = func(_ string) (*golangsdk.ProviderClient, error) {
		return &golangsdk.ProviderClient{}, nil
	}
	authenticate = func(_ *golangsdk.ProviderClient, _ golangsdk.AuthOptionsProvider) error {
		authCalls++
		return nil
	}

	cache := NewCache(
		newFakeClientWithCredentialsSecret(t, `{"accessKey":"ak-1","secretKey":"sk-1"}`),
	)
	spec := testProviderConfigSpec("project-1")

	first, err := cache.GetClient(context.Background(), "pc", spec)
	if err != nil {
		t.Fatalf("GetClient(first): %v", err)
	}

	second, err := cache.GetClient(context.Background(), "pc", spec)
	if err != nil {
		t.Fatalf("GetClient(second): %v", err)
	}

	if authCalls != 1 {
		t.Fatalf("expected exactly one auth call for cache hit, got %d", authCalls)
	}

	if first.ProviderClient != second.ProviderClient {
		t.Fatalf("expected cached provider client pointer to be reused")
	}
}

func TestGetClient_RecreatesClientWhenHashChanges(t *testing.T) {
	previousNewClient := newClient
	previousAuthenticate := authenticate
	t.Cleanup(func() {
		newClient = previousNewClient
		authenticate = previousAuthenticate
	})

	authCalls := 0
	newClient = func(_ string) (*golangsdk.ProviderClient, error) {
		return &golangsdk.ProviderClient{}, nil
	}
	authenticate = func(_ *golangsdk.ProviderClient, _ golangsdk.AuthOptionsProvider) error {
		authCalls++
		return nil
	}

	cache := NewCache(
		newFakeClientWithCredentialsSecret(t, `{"accessKey":"ak-1","secretKey":"sk-1"}`),
	)
	specA := testProviderConfigSpec("project-1")
	specB := testProviderConfigSpec("project-2")

	first, err := cache.GetClient(context.Background(), "pc", specA)
	if err != nil {
		t.Fatalf("GetClient(first): %v", err)
	}

	second, err := cache.GetClient(context.Background(), "pc", specB)
	if err != nil {
		t.Fatalf("GetClient(second): %v", err)
	}

	if authCalls != 2 {
		t.Fatalf("expected auth to be called for changed hash, got %d calls", authCalls)
	}

	if first.ProviderClient == second.ProviderClient {
		t.Fatalf("expected different provider client pointer after hash change")
	}
}

func TestGetClient_RecreatesClientWhenEndpointChanges(t *testing.T) {
	previousNewClient := newClient
	previousAuthenticate := authenticate
	t.Cleanup(func() {
		newClient = previousNewClient
		authenticate = previousAuthenticate
	})

	authCalls := 0
	newClient = func(_ string) (*golangsdk.ProviderClient, error) {
		return &golangsdk.ProviderClient{}, nil
	}
	authenticate = func(_ *golangsdk.ProviderClient, _ golangsdk.AuthOptionsProvider) error {
		authCalls++
		return nil
	}

	cache := NewCache(
		newFakeClientWithCredentialsSecret(t, `{"accessKey":"ak-1","secretKey":"sk-1"}`),
	)

	ep1 := "https://iam.eu-de.otc.t-systems.com/v3"
	ep2 := "https://iam.eu-nl.otc.t-systems.com/v3"

	specA := testProviderConfigSpec("project-1")
	specA.IdentityEndpoint = &ep1

	specB := testProviderConfigSpec("project-1")
	specB.IdentityEndpoint = &ep2

	_, err := cache.GetClient(context.Background(), "pc", specA)
	if err != nil {
		t.Fatalf("GetClient(first): %v", err)
	}

	_, err = cache.GetClient(context.Background(), "pc", specB)
	if err != nil {
		t.Fatalf("GetClient(second): %v", err)
	}

	if authCalls != 2 {
		t.Fatalf("expected 2 auth calls when endpoint changes, got %d", authCalls)
	}
}

func newFakeClientWithCredentialsSecret(t *testing.T, payload string) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme(corev1): %v", err)
	}

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "otc-creds",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"creds": []byte(payload),
			},
		}).
		Build()
}

func testProviderConfigSpec(projectID string) v1alpha1.ProviderConfigSpec {
	return v1alpha1.ProviderConfigSpec{
		DomainName: "example-domain",
		ProjectID:  projectID,
		Region:     "eu-de",
		Credentials: v1alpha1.ProviderCredentials{
			Source: xpv1.CredentialsSourceSecret,
			CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
				SecretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      "otc-creds",
						Namespace: "default",
					},
					Key: "creds",
				},
			},
		},
	}
}
