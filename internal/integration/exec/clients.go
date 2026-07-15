package exec

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/internal/integration/lru"
)

const (
	// clientCacheCapacity bounds the per-system client cache.
	clientCacheCapacity = 64

	// defaultCallTimeout bounds each outbound call when the system does not
	// configure its own timeout, matching the httpx default.
	defaultCallTimeout = 30 * time.Second
)

// clientFactory builds and caches one httpx.Client per system, keyed by the
// hash of the system's connection-relevant fields — a definition change
// yields a new key, so stale clients age out of the LRU without any
// invalidation protocol.
type clientFactory struct {
	registry        *auth.OutboundRegistry
	codec           *definition.SecretCodec
	maxResponseBody int64

	mu    sync.Mutex
	cache *lru.Cache[*httpx.Client]
}

func newClientFactory(registry *auth.OutboundRegistry, codec *definition.SecretCodec, maxResponseBody int64) *clientFactory {
	return &clientFactory{
		registry:        registry,
		codec:           codec,
		maxResponseBody: maxResponseBody,
		cache:           lru.New[*httpx.Client](clientCacheCapacity),
	}
}

// ClientFor returns the client for system, building it on first sight. The
// errors it returns are integration API errors ready to surface.
func (f *clientFactory) ClientFor(system *integration.System) (*httpx.Client, error) {
	key := clientKey(system)

	f.mu.Lock()
	defer f.mu.Unlock()

	if client, ok := f.cache.Get(key); ok {
		return client, nil
	}

	client, err := f.build(system)
	if err != nil {
		return nil, err
	}

	f.cache.Put(key, client)

	return client, nil
}

// CallTimeout returns the per-request timeout of system's client, the bound
// script-supplied request timeouts may only shorten.
func CallTimeout(system *integration.System) time.Duration {
	if system.TimeoutMs > 0 {
		return time.Duration(system.TimeoutMs) * time.Millisecond
	}

	return defaultCallTimeout
}

// build assembles the httpx client implementing the system's connection
// settings and auth scheme.
func (f *clientFactory) build(system *integration.System) (*httpx.Client, error) {
	scheme, ok := f.registry.Resolve(system.OutboundAuth)
	if !ok {
		return nil, integration.ErrUnknownAuthScheme(system.OutboundAuth.Scheme)
	}

	opts := []httpx.Option{
		httpx.WithBaseURL(system.BaseURL),
		httpx.WithTimeout(CallTimeout(system)),
	}

	if f.maxResponseBody > 0 {
		opts = append(opts, httpx.WithMaxResponseBody(f.maxResponseBody))
	}

	if retry := system.Retry; retry != nil && retry.MaxAttempts > 1 {
		opts = append(opts, httpx.WithRetry(httpx.RetryConfig{
			MaxAttempts:    retry.MaxAttempts,
			InitialBackoff: time.Duration(retry.InitialBackoffMs) * time.Millisecond,
			MaxBackoff:     time.Duration(retry.MaxBackoffMs) * time.Millisecond,
		}))
	}

	params, err := f.codec.DecryptOutboundAuth(scheme, system.OutboundAuth)
	if err != nil {
		return nil, integration.ErrInvalidAuthParams(err.Error())
	}

	authOpts, err := scheme.Apply(params)
	if err != nil {
		return nil, integration.ErrInvalidAuthParams(err.Error())
	}

	client, err := httpx.New(append(opts, authOpts...)...)
	if err != nil {
		return nil, integration.ErrInvalidAuthParams(err.Error())
	}

	return client, nil
}

// clientKey derives the cache key from every field that shapes the client.
// Auth params are hashed in their encrypted form — sufficient for change
// detection without holding plaintext in the key.
func clientKey(system *integration.System) string {
	payload, _ := json.Marshal(struct {
		BaseURL      string                          `json:"baseUrl"`
		TimeoutMs    int                             `json:"timeoutMs"`
		Retry        *integration.RetryPolicy        `json:"retry"`
		OutboundAuth *integration.OutboundAuthConfig `json:"outboundAuth"`
	}{system.BaseURL, system.TimeoutMs, system.Retry, system.OutboundAuth})

	return hashx.SHA256Bytes(payload)
}
