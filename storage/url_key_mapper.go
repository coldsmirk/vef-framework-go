package storage

import (
	"net/url"
	"strings"
)

// URLKeyMapper translates between storage object keys (the canonical
// identifiers persisted in ClaimStore / DeleteQueue and used by the
// underlying Service) and the URLs that business templates embed in
// richtext / markdown fields.
//
// The framework uses the mapper in two directions:
//
//   - URLToKey is invoked while reconciling `meta:"richtext"` /
//     `meta:"markdown"` fields, so embedded URLs (e.g. proxy paths like
//     "/storage/files/priv/2026/05/12/foo.png", or CDN URLs like
//     "https://cdn.example.com/priv/2026/05/12/foo.png") are normalised
//     to the keys ClaimStore knows about ("priv/2026/05/12/foo.png")
//     before ConsumeMany / Schedule are called. Implementations decide
//     which URLs map to managed keys; the framework does not pre-filter
//     by scheme, so http(s) URLs reach the mapper too.
//
//   - KeyToURL is exported so business read paths can pair with
//     ReplaceHtmlURLs / ReplaceMarkdownURLs to render stored content
//     with whatever URL convention the frontend expects.
//
// Implementations MUST be deterministic and side-effect free; the
// framework caches nothing about mapper return values and may invoke
// it many times per request.
//
// The framework registers IdentityURLKeyMapper by default. Business
// modules that embed proxy / CDN URLs override it via vef.SupplyURLKeyMapper.
type URLKeyMapper interface {
	// URLToKey returns (key, ok) for the given embedded URL.
	//
	// ok=true: the URL refers to a storage object managed by this
	// system; key is the canonical storage key Files should use for
	// claim consumption and deletion scheduling.
	//
	// ok=false: the URL is unrelated to this system (external CDN,
	// mailto link, data: URI, etc.); Files drops the ref entirely so
	// the URL has no effect on reconciliation.
	URLToKey(url string) (key string, ok bool)

	// KeyToURL returns the URL a frontend should use to fetch the given
	// storage key. Implementations should return the input unchanged when
	// the key is unrecognized.
	KeyToURL(key string) string
}

// IdentityURLKeyMapper is the zero-config mapper that treats relative
// URLs as bare storage keys. Suitable when the frontend convention is
// to embed bare object keys (e.g. `<img src="priv/2026/05/12/foo.png">`)
// and to resolve them to a viewable URL at render time.
// DefaultProxyPrefix is the URL path prefix the framework's proxy
// middleware mounts at. ProxyURLKeyMapper uses it to translate between
// embedded URLs and storage keys.
const DefaultProxyPrefix = "/storage/files/"

// ProxyURLKeyMapper is the recommended default mapper for applications
// that embed the framework's proxy URL convention in richtext /
// markdown fields (e.g. `<img src="/storage/files/priv/2026/05/12/foo.png">`).
//
// URLToKey strips the configured Prefix (default "/storage/files/") and
// returns the remainder as the storage key. URLs that do not start with
// the prefix — including scheme-bearing URLs, data: URIs, and external
// links — are rejected with ok=false.
//
// KeyToURL prepends the Prefix to produce the proxy URL the frontend
// expects.
type ProxyURLKeyMapper struct {
	// Prefix is the URL path prefix to strip/add. Defaults to
	// DefaultProxyPrefix ("/storage/files/") when empty.
	Prefix string
}

func (m ProxyURLKeyMapper) prefix() string {
	if m.Prefix != "" {
		return m.Prefix
	}

	return DefaultProxyPrefix
}

// URLToKey strips the proxy prefix from rawURL and returns the storage
// key. Returns ok=false for URLs that do not match the prefix or carry
// a scheme (external links).
func (m ProxyURLKeyMapper) URLToKey(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}

	// Reject scheme-bearing URLs (http/https/data/mailto/etc.)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}

	if parsed.Scheme != "" {
		return "", false
	}

	p := m.prefix()
	if !strings.HasPrefix(trimmed, p) {
		// Also accept bare keys (no prefix) for backward compatibility
		// with content that was stored before the mapper was configured.
		if !strings.Contains(trimmed, "/") {
			return "", false
		}

		// Check if it looks like a storage key (starts with pub/ or priv/).
		if strings.HasPrefix(trimmed, PublicPrefix) || strings.HasPrefix(trimmed, PrivatePrefix) {
			return trimmed, true
		}

		return "", false
	}

	key := strings.TrimPrefix(trimmed, p)
	if key == "" {
		return "", false
	}

	return key, true
}

// KeyToURL prepends the proxy prefix to produce the URL the frontend
// should embed.
func (m ProxyURLKeyMapper) KeyToURL(key string) string {
	return m.prefix() + key
}

// IdentityURLKeyMapper is a simple mapper that treats relative URLs as
// bare storage keys. Suitable only when the frontend embeds bare object
// keys directly (e.g. `<img src="priv/2026/05/12/foo.png">`).
//
// Any URL with an explicit scheme is rejected with ok=false.
type IdentityURLKeyMapper struct{}

// URLToKey returns (url, true) for empty-scheme URLs (plain relative
// paths like "priv/foo.png"). Any URL carrying a scheme is rejected.
func (*IdentityURLKeyMapper) URLToKey(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}

	if parsed.Scheme != "" {
		return "", false
	}

	return trimmed, true
}

// KeyToURL returns key unchanged.
func (*IdentityURLKeyMapper) KeyToURL(key string) string { return key }
