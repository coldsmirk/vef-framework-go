package dbtls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/coldsmirk/vef-framework-go/config"
)

var (
	// ErrUnknownSSLMode is returned for an SSLMode value outside the supported set.
	ErrUnknownSSLMode = errors.New("unknown ssl mode")
	// ErrNoRootCert is returned when an ssl_root_cert file contains no usable
	// PEM certificates.
	ErrNoRootCert = errors.New("ssl root cert contains no valid PEM certificates")
	// ErrNoPeerCert is returned when the server completes a handshake without
	// presenting any certificate (only reachable in the verify modes).
	ErrNoPeerCert = errors.New("server presented no certificates")
)

// Config translates a config.SSLMode into a *tls.Config for a network database
// driver. A nil result means TLS is disabled and the caller should connect over
// plaintext. serverName is the host used for verify-full hostname matching and
// may be empty for the non-verifying modes.
//
//   - disable (and the empty default): nil, nil — plaintext.
//   - require: encryption only, no certificate or hostname verification.
//   - verify-ca: verify the certificate chains to a trusted CA, skip hostname.
//   - verify-full: verify the CA chain and that the certificate matches
//     serverName.
//
// rootCertPath optionally points at a PEM CA bundle used by the verify modes;
// when empty the host system pool is used.
func Config(mode config.SSLMode, rootCertPath, serverName string) (*tls.Config, error) {
	switch mode {
	case "", config.SSLDisable:
		return nil, nil

	case config.SSLRequire:
		// Encryption without authentication: deliberately skip verification.
		return &tls.Config{InsecureSkipVerify: true}, nil //nolint:gosec // require mode is encryption-only by design

	case config.SSLVerifyCA:
		roots, err := rootCAs(rootCertPath)
		if err != nil {
			return nil, err
		}

		// The stdlib handshake verifies both the chain and the hostname; verify-ca
		// wants the chain only. Disable the built-in verification and run a custom
		// chain build with no DNSName so hostname matching is skipped.
		// VerifyConnection (not VerifyPeerCertificate) is used so the check also
		// runs on resumed TLS sessions.
		return &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // chain verified manually in VerifyConnection; hostname intentionally skipped for verify-ca
			RootCAs:            roots,
			VerifyConnection:   chainOnlyVerifier(roots),
		}, nil

	case config.SSLVerifyFull:
		roots, err := rootCAs(rootCertPath)
		if err != nil {
			return nil, err
		}

		return &tls.Config{
			ServerName: serverName,
			RootCAs:    roots,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownSSLMode, mode)
	}
}

// rootCAs returns the certificate pool used to verify the server. With no
// explicit path it falls back to the system pool (nil RootCAs lets crypto/tls
// use the host roots).
func rootCAs(rootCertPath string) (*x509.CertPool, error) {
	if rootCertPath == "" {
		return nil, nil
	}

	pem, err := os.ReadFile(rootCertPath)
	if err != nil {
		return nil, fmt.Errorf("read ssl root cert %q: %w", rootCertPath, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%w: %s", ErrNoRootCert, rootCertPath)
	}

	return pool, nil
}

// chainOnlyVerifier builds a VerifyConnection callback that verifies the
// presented chain against roots without checking the hostname. roots may be nil
// to use the system pool. Running as VerifyConnection (rather than
// VerifyPeerCertificate) ensures the check is enforced on resumed sessions too.
func chainOnlyVerifier(roots *x509.CertPool) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return ErrNoPeerCert
		}

		intermediates := x509.NewCertPool()
		for _, cert := range cs.PeerCertificates[1:] {
			intermediates.AddCert(cert)
		}

		if _, err := cs.PeerCertificates[0].Verify(x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
		}); err != nil {
			return fmt.Errorf("verify server certificate chain: %w", err)
		}

		return nil
	}
}
