package storage_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/storage"
)

// TestIdentityURLKeyMapperURLToKey covers the new (key, ok) contract for
// the zero-config mapper. The mapper accepts relative paths (no scheme)
// and rejects every URL carrying a scheme so business modules embedding
// http/https / data / mailto / javascript URLs do not accidentally
// trigger reconciliation on objects this system does not own.
func TestIdentityURLKeyMapperURLToKey(t *testing.T) {
	m := new(storage.IdentityURLKeyMapper)

	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantOK    bool
		assertMsg string
	}{
		{
			name:      "EmptyStringRejected",
			input:     "",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "Empty URL is never a managed key",
		},
		{
			name:      "WhitespaceOnlyRejected",
			input:     "   ",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "Whitespace-only URL is never a managed key",
		},
		{
			name:      "RelativeKeyAccepted",
			input:     "priv/2026/05/12/foo.png",
			wantKey:   "priv/2026/05/12/foo.png",
			wantOK:    true,
			assertMsg: "Bare storage key (no scheme) is the identity case",
		},
		{
			name:      "LeadingSlashKeyAccepted",
			input:     "/storage/files/priv/foo.png",
			wantKey:   "/storage/files/priv/foo.png",
			wantOK:    true,
			assertMsg: "Path with leading slash but no scheme is still relative",
		},
		{
			name:      "RelativePathTrimmed",
			input:     "  priv/foo.png  ",
			wantKey:   "priv/foo.png",
			wantOK:    true,
			assertMsg: "Surrounding whitespace must be stripped before storing",
		},
		{
			name:      "HttpRejected",
			input:     "http://example.com/pic.jpg",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "Identity mapper has no way to know whether http URLs are managed",
		},
		{
			name:      "HttpsRejected",
			input:     "https://cdn.example.com/priv/foo.png",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "Identity mapper has no way to know whether https URLs are managed",
		},
		{
			name:      "DataURIRejected",
			input:     "data:image/png;base64,iVBORw0KGgoAAA",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "data: URIs are inline payloads, never storage keys",
		},
		{
			name:      "MailtoRejected",
			input:     "mailto:foo@bar.com",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "mailto: links are not storage references",
		},
		{
			name:      "JavascriptRejected",
			input:     "javascript:alert(1)",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "javascript: pseudo-URLs are not storage references",
		},
		{
			name:      "FtpRejected",
			input:     "ftp://example.com/file.zip",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "Any scheme other than empty means the URL is not a bare key",
		},
		{
			name:      "MalformedURLRejected",
			input:     "http://[::1:80/",
			wantKey:   "",
			wantOK:    false,
			assertMsg: "Inputs that fail to parse are treated as not-managed rather than panicking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, ok := m.URLToKey(tt.input)
			assert.Equal(t, tt.wantOK, ok, tt.assertMsg)
			assert.Equal(t, tt.wantKey, key, tt.assertMsg)
		})
	}
}

// TestIdentityURLKeyMapperKeyToURL guards the inverse direction: the
// identity mapper passes keys through unchanged so business read paths
// can compose KeyToURL with their own templating.
func TestIdentityURLKeyMapperKeyToURL(t *testing.T) {
	m := new(storage.IdentityURLKeyMapper)

	tests := []struct {
		name string
		key  string
	}{
		{"BareKey", "priv/2026/05/12/foo.png"},
		{"EmptyKey", ""},
		{"KeyWithSpaces", "priv/foo bar.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.key, m.KeyToURL(tt.key), "Identity KeyToURL must be a pass-through")
		})
	}
}

// cdnHostMapper is a documented example of how a business module wires
// a custom URLKeyMapper to recognize CDN URLs of the form
// "https://cdn.example.com/<key>". The key extractor strips the host
// prefix; the URL composer puts it back. This pattern is what the new
// (key, ok) signature is designed to enable: previously, http/https
// URLs never reached the mapper because the extractor pre-filtered
// them, so business code could not opt in even by writing a custom
// mapper.
type cdnHostMapper struct {
	prefix string
}

func (m cdnHostMapper) URLToKey(raw string) (string, bool) {
	if rest, ok := strings.CutPrefix(raw, m.prefix); ok {
		return rest, true
	}

	// Relative paths still resolve through the same mapper for
	// completeness; absolute non-CDN URLs are rejected so the mapper
	// remains a deterministic contract for what counts as managed.
	if !strings.Contains(raw, "://") {
		return raw, raw != ""
	}

	return "", false
}

func (m cdnHostMapper) KeyToURL(key string) string {
	return m.prefix + key
}

// TestCustomMapperRecognizesCDNHost demonstrates that http(s) URLs can
// now flow through to a custom URLKeyMapper. With the old extractor
// pre-filter, "https://cdn.example.com/priv/foo.png" would never reach
// URLToKey and the mapper would have no way to claim it.
func TestCustomMapperRecognizesCDNHost(t *testing.T) {
	m := cdnHostMapper{prefix: "https://cdn.example.com/"}

	t.Run("MatchingCDNURLBecomesKey", func(t *testing.T) {
		key, ok := m.URLToKey("https://cdn.example.com/priv/2026/05/12/foo.png")
		assert.True(t, ok, "URL with the expected CDN prefix must resolve to a managed key")
		assert.Equal(t, "priv/2026/05/12/foo.png", key, "Mapper must strip the CDN prefix to recover the storage key")
	})

	t.Run("UnknownHostRejected", func(t *testing.T) {
		_, ok := m.URLToKey("https://other.example.com/foo.png")
		assert.False(t, ok, "URL with a foreign host must be treated as not-managed so reconciliation skips it")
	})

	t.Run("RelativeKeyStillAccepted", func(t *testing.T) {
		// Backwards-compatible: a custom mapper that wants to also
		// accept bare keys (e.g. older content predating the CDN
		// rollout) can do so without affecting the new branch.
		key, ok := m.URLToKey("priv/2026/05/12/foo.png")
		assert.True(t, ok, "Relative key should still resolve when the custom mapper opts in to that behavior")
		assert.Equal(t, "priv/2026/05/12/foo.png", key, "Relative key must round-trip unchanged")
	})

	t.Run("EmptyRejected", func(t *testing.T) {
		_, ok := m.URLToKey("")
		assert.False(t, ok, "Empty URL is never a managed key, regardless of mapper")
	})

	t.Run("KeyToURLRoundTrip", func(t *testing.T) {
		assert.Equal(t,
			"https://cdn.example.com/priv/foo.png",
			m.KeyToURL("priv/foo.png"),
			"KeyToURL must compose the CDN prefix so frontends receive a fetchable URL",
		)
	})
}
