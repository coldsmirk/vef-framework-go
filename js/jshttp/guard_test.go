package jshttp

import (
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForbiddenIP tests the SSRF address classification.
func TestForbiddenIP(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		forbidden bool
	}{
		{name: "Loopback", ip: "127.0.0.1", forbidden: true},
		{name: "LoopbackV6", ip: "::1", forbidden: true},
		{name: "PrivateClassA", ip: "10.0.0.1", forbidden: true},
		{name: "PrivateClassB", ip: "172.16.0.1", forbidden: true},
		{name: "PrivateClassC", ip: "192.168.1.1", forbidden: true},
		{name: "UniqueLocalV6", ip: "fc00::1", forbidden: true},
		{name: "LinkLocal", ip: "169.254.1.1", forbidden: true},
		{name: "CloudMetadata", ip: "169.254.169.254", forbidden: true},
		{name: "LinkLocalV6", ip: "fe80::1", forbidden: true},
		{name: "Unspecified", ip: "0.0.0.0", forbidden: true},
		{name: "UnspecifiedV6", ip: "::", forbidden: true},
		{name: "Multicast", ip: "224.0.0.1", forbidden: true},
		{name: "PublicV4", ip: "8.8.8.8", forbidden: false},
		{name: "PublicV6", ip: "2606:4700:4700::1111", forbidden: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "Test address should parse")
			assert.Equal(t, tt.forbidden, forbiddenIP(ip), "Classification should match for %s", tt.ip)
		})
	}
}

// TestValidateURL tests the request-level URL policy.
func TestValidateURL(t *testing.T) {
	parse := func(t *testing.T, raw string) *url.URL {
		t.Helper()

		u, err := url.Parse(raw)
		require.NoError(t, err, "Test URL should parse")

		return u
	}

	t.Run("SchemeWhitelist", func(t *testing.T) {
		l := New().(*lib)

		require.NoError(t, l.validateURL(parse(t, "https://example.com/x")), "HTTPS should be allowed")
		require.NoError(t, l.validateURL(parse(t, "http://example.com/x")), "HTTP should be allowed")
		require.ErrorIs(t, l.validateURL(parse(t, "file:///etc/passwd")), ErrSchemeNotAllowed, "File scheme should be rejected")
		require.ErrorIs(t, l.validateURL(parse(t, "ftp://example.com/x")), ErrSchemeNotAllowed, "FTP scheme should be rejected")
	})

	t.Run("HostAllowlist", func(t *testing.T) {
		l := New(WithAllowedHosts("API.Example.com")).(*lib)

		require.NoError(t, l.validateURL(parse(t, "https://api.example.com/x")), "Allowlisted host should pass case-insensitively")
		require.ErrorIs(t, l.validateURL(parse(t, "https://evil.example.com/x")), ErrHostNotAllowed, "Host outside the allowlist should be rejected")
	})

	t.Run("UnrestrictedByDefault", func(t *testing.T) {
		l := New().(*lib)

		require.NoError(t, l.validateURL(parse(t, "http://127.0.0.1:8080/x")), "Loopback should pass without WithPublicNetworkOnly")
		require.NoError(t, l.validateURL(parse(t, "http://169.254.169.254/meta")), "Metadata endpoint should pass without WithPublicNetworkOnly")
		require.NoError(t, l.validateURL(parse(t, "http://internal.service/x")), "Any hostname should pass without an allowlist")
	})

	t.Run("PublicNetworkOnly", func(t *testing.T) {
		l := New(WithPublicNetworkOnly()).(*lib)

		require.ErrorIs(t, l.validateURL(parse(t, "http://127.0.0.1:8080/x")), ErrAddressNotAllowed, "Literal loopback should be rejected")
		require.ErrorIs(t, l.validateURL(parse(t, "http://169.254.169.254/meta")), ErrAddressNotAllowed, "Metadata endpoint should be rejected")
		require.NoError(t, l.validateURL(parse(t, "http://93.184.216.34/x")), "Literal public address should pass")
		require.NoError(t, l.validateURL(parse(t, "http://internal.service/x")), "DNS names pass the URL check; the dial guard vets the resolved address")
	})
}
