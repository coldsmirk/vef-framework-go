package dbtls_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/dbtls"
)

func TestConfig(t *testing.T) {
	t.Run("DisableReturnsNilForPlaintext", func(t *testing.T) {
		got, err := dbtls.Config(config.SSLModeDisable, "", "db.example.com")

		require.NoError(t, err, "disable mode should not error")
		assert.Nil(t, got, "disable mode should yield a nil config signaling plaintext")
	})

	t.Run("EmptyModeDefaultsToDisable", func(t *testing.T) {
		got, err := dbtls.Config("", "", "db.example.com")

		require.NoError(t, err, "empty mode should not error")
		assert.Nil(t, got, "empty mode should behave exactly like disable (plaintext)")
	})

	t.Run("RequireSkipsAllVerification", func(t *testing.T) {
		got, err := dbtls.Config(config.SSLModeRequire, "", "db.example.com")

		require.NoError(t, err, "require mode should not error")
		require.NotNil(t, got, "require mode should yield a TLS config")
		assert.True(t, got.InsecureSkipVerify, "require mode must skip certificate verification")
		assert.Nil(t, got.VerifyConnection, "require mode performs no custom chain verification")
		assert.Empty(t, got.ServerName, "require mode does not pin a server name")
	})

	t.Run("VerifyCASkipsHostnameButVerifiesChain", func(t *testing.T) {
		got, err := dbtls.Config(config.SSLModeVerifyCA, "", "db.example.com")

		require.NoError(t, err, "verify-ca mode should not error with system pool")
		require.NotNil(t, got, "verify-ca mode should yield a TLS config")
		assert.True(t, got.InsecureSkipVerify, "verify-ca disables stdlib verification to skip hostname")
		assert.NotNil(t, got.VerifyConnection, "verify-ca installs a custom chain-only verifier that also covers resumed sessions")
		assert.Empty(t, got.ServerName, "verify-ca does not pin a server name")
	})

	t.Run("VerifyFullPinsHostnameAndChain", func(t *testing.T) {
		got, err := dbtls.Config(config.SSLModeVerifyFull, "", "db.example.com")

		require.NoError(t, err, "verify-full mode should not error with system pool")
		require.NotNil(t, got, "verify-full mode should yield a TLS config")
		assert.False(t, got.InsecureSkipVerify, "verify-full relies on stdlib full verification")
		assert.Nil(t, got.VerifyConnection, "verify-full uses stdlib verification, not a custom hook")
		assert.Equal(t, "db.example.com", got.ServerName, "verify-full pins the server name for hostname matching")
	})

	t.Run("UnknownModeErrors", func(t *testing.T) {
		got, err := dbtls.Config("bogus", "", "db.example.com")

		require.ErrorIs(t, err, dbtls.ErrUnknownSSLMode, "unknown mode should report ErrUnknownSSLMode")
		assert.Nil(t, got, "no config should be returned on error")
	})

	t.Run("RootCertLoadedIntoPool", func(t *testing.T) {
		certPath := writeTestCA(t)

		for _, mode := range []config.SSLMode{config.SSLModeVerifyCA, config.SSLModeVerifyFull} {
			got, err := dbtls.Config(mode, certPath, "db.example.com")

			require.NoError(t, err, "valid CA file should load for mode %s", mode)
			require.NotNil(t, got, "config should be returned for mode %s", mode)
			require.NotNil(t, got.RootCAs, "explicit CA file should populate RootCAs for mode %s", mode)
		}
	})

	t.Run("MissingRootCertErrors", func(t *testing.T) {
		got, err := dbtls.Config(config.SSLModeVerifyFull, "/no/such/ca.pem", "db.example.com")

		require.Error(t, err, "a missing CA file should surface an error")
		assert.Nil(t, got, "no config should be returned when the CA file cannot be read")
	})

	t.Run("InvalidRootCertErrors", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "bad.pem")
		require.NoError(t, os.WriteFile(bad, []byte("not a certificate"), 0o600), "writing the bad PEM should succeed")

		got, err := dbtls.Config(config.SSLModeVerifyCA, bad, "db.example.com")

		require.ErrorIs(t, err, dbtls.ErrNoRootCert, "a PEM file with no certificates should report ErrNoRootCert")
		assert.Nil(t, got, "no config should be returned for an invalid CA file")
	})

	t.Run("RootCertIgnoredWhenNotVerifying", func(t *testing.T) {
		// disable/require never read the CA file, so a bogus path must not fail them.
		disable, err := dbtls.Config(config.SSLModeDisable, "/no/such/ca.pem", "db.example.com")
		require.NoError(t, err, "disable must not read the CA file")
		assert.Nil(t, disable, "disable still yields plaintext")

		require0, err := dbtls.Config(config.SSLModeRequire, "/no/such/ca.pem", "db.example.com")
		require.NoError(t, err, "require must not read the CA file")
		require.NotNil(t, require0, "require still yields a TLS config")
	})
}

// writeTestCA generates a throwaway self-signed CA certificate, writes it to a
// temp PEM file, and returns the path. It exercises the rootCAs PEM-loading
// path without needing a real database handshake.
func writeTestCA(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "generating the test CA key should succeed")

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "vef-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err, "creating the test CA certificate should succeed")

	path := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600), "writing the CA PEM should succeed")

	return path
}
