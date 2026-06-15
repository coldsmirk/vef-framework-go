package dbtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signedCert bundles a generated certificate with the key that signed it so a
// chain can be assembled in tests.
type signedCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// newCA generates a self-signed CA certificate usable as a trust root.
func newCA(t *testing.T) signedCert {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "generating the CA key should succeed")

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "vef-test-root-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err, "creating the CA certificate should succeed")

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err, "parsing the CA certificate should succeed")

	return signedCert{cert: cert, key: key}
}

// newLeaf generates a leaf certificate signed by issuer, with the given CN and
// DNS SAN, modeling a database server certificate.
func newLeaf(t *testing.T, issuer signedCert, commonName, dnsName string) signedCert {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "generating the leaf key should succeed")

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{dnsName},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, issuer.cert, &key.PublicKey, issuer.key)
	require.NoError(t, err, "signing the leaf certificate should succeed")

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err, "parsing the leaf certificate should succeed")

	return signedCert{cert: cert, key: key}
}

// poolOf returns a CertPool containing the given CA certificate.
func poolOf(ca signedCert) *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)

	return pool
}

// TestChainOnlyVerifier exercises the verify-ca chain-only verifier against real
// certificate chains, rather than only asserting tls.Config shape. It proves the
// verifier accepts a leaf chaining to the supplied root, rejects a leaf from an
// untrusted CA, fails when the peer presents no certificate, and — crucially for
// verify-ca semantics — ignores hostname mismatches.
func TestChainOnlyVerifier(t *testing.T) {
	trustedCA := newCA(t)
	untrustedCA := newCA(t)
	roots := poolOf(trustedCA)

	t.Run("AcceptsLeafChainingToTrustedRoot", func(t *testing.T) {
		leaf := newLeaf(t, trustedCA, "db.internal", "db.internal")
		verify := chainOnlyVerifier(roots)

		err := verify(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf.cert}})

		assert.NoError(t, err, "a leaf chaining to the trusted root must verify under verify-ca")
	})

	t.Run("RejectsLeafFromUntrustedCA", func(t *testing.T) {
		leaf := newLeaf(t, untrustedCA, "db.internal", "db.internal")
		verify := chainOnlyVerifier(roots)

		err := verify(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf.cert}})

		require.Error(t, err, "a leaf signed by an untrusted CA must be rejected")

		var unknownAuthority x509.UnknownAuthorityError
		assert.True(t, errors.As(err, &unknownAuthority),
			"rejection must stem from an untrusted chain, got: %v", err)
	})

	t.Run("ErrorsWhenNoPeerCertificate", func(t *testing.T) {
		verify := chainOnlyVerifier(roots)

		err := verify(tls.ConnectionState{PeerCertificates: nil})

		assert.ErrorIs(t, err, ErrNoPeerCert,
			"an empty peer chain must surface ErrNoPeerCert")
	})

	t.Run("IgnoresHostnameMismatch", func(t *testing.T) {
		// verify-ca verifies the chain but deliberately skips hostname matching.
		// A leaf whose CN/SAN does not match the dialed host must still verify, so
		// long as it chains to a trusted root. A hostname-checking verifier would
		// reject this; chainOnlyVerifier must not.
		leaf := newLeaf(t, trustedCA, "wrong-host.example", "wrong-host.example")
		verify := chainOnlyVerifier(roots)

		err := verify(tls.ConnectionState{
			ServerName:       "db.internal", // dialed host differs from the cert's CN/SAN
			PeerCertificates: []*x509.Certificate{leaf.cert},
		})

		assert.NoError(t, err,
			"verify-ca must ignore hostname: a chain-valid cert with a mismatched CN/SAN still verifies")
	})

	t.Run("IgnoresIPHostnameMismatch", func(t *testing.T) {
		// Same intent as above with an IP ServerName, ensuring no hostname/IP SAN
		// matching leaks into the chain-only path.
		leaf := newLeaf(t, trustedCA, "10.0.0.5", "db.internal")
		verify := chainOnlyVerifier(roots)

		err := verify(tls.ConnectionState{
			ServerName:       net.IPv4(10, 0, 0, 9).String(),
			PeerCertificates: []*x509.Certificate{leaf.cert},
		})

		assert.NoError(t, err, "verify-ca must not perform IP SAN matching either")
	})
}
