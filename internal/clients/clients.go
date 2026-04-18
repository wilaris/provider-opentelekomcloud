package clients

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.wilaris.de/provider-opentelekomcloud/apis/v1alpha1"
)

const (
	// DefaultIdentityEndpoint is the default OTC identity endpoint.
	DefaultIdentityEndpoint = "https://iam.eu-de.otc.t-systems.com/v3"
)

var (
	sharedCache     *Cache
	sharedCacheOnce sync.Once

	newClient    = openstack.NewClient
	authenticate = openstack.Authenticate
)

// Credentials represents AK/SK credentials.
type Credentials struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type Client struct {
	ProviderClient *golangsdk.ProviderClient
	Region         string
}

// session holds an active connection and metadata.
type session struct {
	client *golangsdk.ProviderClient
	hash   string
}

// Cache manages the lifecycle of OTC connections to prevent rate limiting.
type Cache struct {
	mu       sync.RWMutex
	sessions map[string]*session // Key: ProviderConfig Name
	client   client.Client
}

func NewCache(kube client.Client) *Cache {
	return &Cache{
		sessions: make(map[string]*session),
		client:   kube,
	}
}

// SharedCache returns a process-wide shared Cache instance.
func SharedCache(kube client.Client) *Cache {
	sharedCacheOnce.Do(func() {
		sharedCache = NewCache(kube)
	})
	return sharedCache
}

// GetClient returns a cached client or creates a new one.
func (c *Cache) GetClient(
	ctx context.Context,
	key string,
	spec v1alpha1.ProviderConfigSpec,
) (*Client, error) {
	// Resolve credentials (AK/SK) from secret
	creds, err := extractCredentials(ctx, c.client, spec)
	if err != nil {
		return nil, errors.Wrap(err, "cannot extract credentials")
	}

	// Resolve endpoint early so it participates in the cache hash
	endpoint := DefaultIdentityEndpoint
	if spec.IdentityEndpoint != nil && *spec.IdentityEndpoint != "" {
		endpoint = *spec.IdentityEndpoint
	}

	configHash := calculateHash(spec, creds, endpoint)

	// Check the cache
	c.mu.RLock()
	cached, ok := c.sessions[key]
	c.mu.RUnlock()

	if ok && cached.hash == configHash {
		return &Client{
			ProviderClient: cached.client,
			Region:         spec.Region,
		}, nil
	}

	// Create a (unauthenticated) provider client
	providerClient, err := newClient(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create provider client")
	}

	// Configure redirect re-signing for AK/SK auth.
	providerClient.HTTPClient = http.Client{
		Transport: providerClient.HTTPClient.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			golangsdk.ReSign(req, golangsdk.SignOptions{
				AccessKey: creds.AccessKey,
				SecretKey: creds.SecretKey,
			})
			return nil
		},
	}

	authOpts := golangsdk.AKSKAuthOptions{
		IdentityEndpoint: endpoint,
		AccessKey:        creds.AccessKey,
		SecretKey:        creds.SecretKey,
		ProjectId:        spec.ProjectID,
		Region:           spec.Region,
	}

	if err := authenticate(providerClient, authOpts); err != nil {
		return nil, errors.Wrap(err, "cannot authenticate with Open Telekom Cloud")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessions[key] = &session{
		client: providerClient,
		hash:   configHash,
	}

	return &Client{
		ProviderClient: providerClient,
		Region:         spec.Region,
	}, nil
}

// extractCredentials extracts AK/SK credentials from the configured source.
func extractCredentials(
	ctx context.Context,
	kube client.Client,
	spec v1alpha1.ProviderConfigSpec,
) (*Credentials, error) {
	if spec.Credentials.Source != xpv1.CredentialsSourceSecret {
		return nil, errors.Errorf("unsupported credentials source: %s", spec.Credentials.Source)
	}

	ref := spec.Credentials.SecretRef
	if ref == nil {
		return nil, errors.New("secretRef is required")
	}

	// Use Crossplane helper to fetch secret data
	data, err := resource.CommonCredentialExtractor(
		ctx,
		spec.Credentials.Source,
		kube,
		spec.Credentials.CommonCredentialSelectors,
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get credentials from secret")
	}

	// Unmarshal JSON into AK/SK struct
	creds := &Credentials{}
	if err := json.Unmarshal(data, creds); err != nil {
		return nil, errors.Wrap(
			err,
			"cannot unmarshal credentials JSON, expect keys: accessKey, secretKey",
		)
	}

	if creds.AccessKey == "" || creds.SecretKey == "" {
		return nil, errors.New("accessKey and secretKey are required in credentials secret")
	}

	return creds, nil
}

func calculateHash(spec v1alpha1.ProviderConfigSpec, creds *Credentials, endpoint string) string {
	s := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		endpoint,
		spec.DomainName,
		spec.ProjectID,
		spec.Region,
		creds.AccessKey,
		creds.SecretKey,
	)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
