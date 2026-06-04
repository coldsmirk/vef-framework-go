package security

import (
	"context"
	"crypto/hmac"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/id"
)

// SignatureCredentials represents the credentials extracted from HTTP headers
// for signature-based authentication.
type SignatureCredentials struct {
	// Timestamp is the Unix timestamp (seconds) when the request was created.
	Timestamp int64

	// Nonce is a random string to prevent replay attacks.
	Nonce string

	// Signature is the HMAC signature in hex encoding.
	Signature string
}

// SignatureAlgorithm represents the HMAC algorithm used for signing.
type SignatureAlgorithm string

const (
	SignatureAlgHmacSHA256 SignatureAlgorithm = "HMAC-SHA256"
	SignatureAlgHmacSHA512 SignatureAlgorithm = "HMAC-SHA512"
	SignatureAlgHmacSM3    SignatureAlgorithm = "HMAC-SM3"
)

const (
	defaultSignatureAlgorithm          = SignatureAlgHmacSHA256
	defaultSignatureTimestampTolerance = 5 * time.Minute
	nonceTTLBuffer                     = 1 * time.Minute
)

// SignatureOption configures a Signature instance.
type SignatureOption func(*Signature)

// WithAlgorithm sets the HMAC algorithm. Defaults to HMAC-SHA256.
func WithAlgorithm(algorithm SignatureAlgorithm) SignatureOption {
	return func(s *Signature) {
		s.algorithm = algorithm
	}
}

// WithTimestampTolerance sets the maximum allowed time difference.
// Defaults to 5 minutes.
func WithTimestampTolerance(tolerance time.Duration) SignatureOption {
	return func(s *Signature) {
		s.timestampTolerance = tolerance
	}
}

// WithNonceStore sets the nonce store for replay attack prevention.
// If not set, nonce validation is skipped.
func WithNonceStore(store NonceStore) SignatureOption {
	return func(s *Signature) {
		s.nonceStore = store
	}
}

// Signature provides HMAC-based signature generation and verification.
// It handles timestamp validation and supports optional data hash for integrity.
type Signature struct {
	secret             []byte
	algorithm          SignatureAlgorithm
	timestampTolerance time.Duration
	nonceGenerator     id.IDGenerator
	nonceStore         NonceStore
}

// SignatureResult contains the result of a signature operation.
type SignatureResult struct {
	AppID     string
	Timestamp int64
	Nonce     string
	Signature string
}

// NewSignature creates a new Signature instance.
// The secret parameter is required and expects a hex-encoded string.
func NewSignature(secret string, opts ...SignatureOption) (*Signature, error) {
	secretBytes, err := hex.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeSignatureSecretFailed, err)
	}

	if len(secretBytes) == 0 {
		return nil, ErrSignatureSecretRequired
	}

	s := &Signature{
		secret:             secretBytes,
		algorithm:          defaultSignatureAlgorithm,
		timestampTolerance: defaultSignatureTimestampTolerance,
		nonceGenerator:     id.NewRandomIDGenerator(),
		nonceStore:         NewMemoryNonceStore(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Sign generates a signature for the given appID bound to the request's HTTP
// method and path. Callers pass the same method/path the server will see
// (e.g. "POST", "/api"). Returns a SignatureResult containing all components.
func (s *Signature) Sign(appID, method, path string) (*SignatureResult, error) {
	if appID == "" {
		return nil, ErrAppIDRequired
	}

	nonce := s.nonceGenerator.Generate()
	timestampSec := time.Now().Unix()
	payload := s.buildPayload(appID, method, path, timestampSec, nonce)
	signature := s.computeHMAC(payload)

	return &SignatureResult{
		AppID:     appID,
		Timestamp: timestampSec,
		Nonce:     nonce,
		Signature: signature,
	}, nil
}

// Verify validates the signature against the provided parameters, including
// the request's HTTP method and path (which must match what was signed).
// Returns nil if valid, or an error describing the validation failure.
func (s *Signature) Verify(ctx context.Context, appID, method, path string, timestamp int64, nonce, signature string) error {
	return s.verifyWithSecret(ctx, s.secret, appID, method, path, timestamp, nonce, signature)
}

// VerifyWithSecret validates the signature using an externally provided secret.
// This is useful when the secret is loaded dynamically per-request (e.g., from ExternalAppLoader).
// The secret parameter expects a hex-encoded string.
func (s *Signature) VerifyWithSecret(ctx context.Context, secret, appID, method, path string, timestamp int64, nonce, signature string) error {
	secretBytes, err := hex.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecodeSignatureSecretFailed, err)
	}

	return s.verifyWithSecret(ctx, secretBytes, appID, method, path, timestamp, nonce, signature)
}

// verifyWithSecret is the internal implementation for signature verification.
func (s *Signature) verifyWithSecret(ctx context.Context, secret []byte, appID, method, path string, timestamp int64, nonce, signature string) error {
	if appID == "" {
		return ErrAppIDRequired
	}

	if nonce == "" {
		return ErrNonceRequired
	}

	if signature == "" {
		return ErrSignatureRequired
	}

	if err := s.validateTimestamp(timestamp); err != nil {
		return err
	}

	payload := s.buildPayload(appID, method, path, timestamp, nonce)
	expectedSignature := s.computeHMACWithSecret(secret, payload)

	expectedMAC, err := hex.DecodeString(expectedSignature)
	if err != nil {
		return fmt.Errorf("failed to decode expected signature: %w", err)
	}

	providedMAC, err := hex.DecodeString(signature)
	if err != nil {
		return ErrSignatureInvalid
	}

	if !hmac.Equal(expectedMAC, providedMAC) {
		return ErrSignatureInvalid
	}

	return s.checkAndStoreNonce(ctx, appID, nonce)
}

// checkAndStoreNonce atomically stores a nonce and rejects replays if NonceStore is configured.
func (s *Signature) checkAndStoreNonce(ctx context.Context, appID, nonce string) error {
	if s.nonceStore == nil {
		return nil
	}

	stored, err := s.nonceStore.StoreIfAbsent(ctx, appID, nonce, s.timestampTolerance+nonceTTLBuffer)
	if err != nil {
		return fmt.Errorf("failed to store nonce: %w", err)
	}

	if !stored {
		return ErrNonceAlreadyUsed
	}

	return nil
}

// buildPayload assembles the canonical (alphabetically-keyed) string that is
// HMAC-signed. The HTTP method and path are bound so a captured signature
// cannot be replayed against a different endpoint. The request body is
// intentionally NOT bound: it is large and any benign re-serialization (key
// ordering, whitespace) would break otherwise-valid requests; replay of the
// same endpoint is already prevented by the nonce + timestamp.
func (*Signature) buildPayload(appID, method, path string, timestamp int64, nonce string) []byte {
	return fmt.Appendf(nil, "app_id=%s&method=%s&nonce=%s&path=%s&timestamp=%d",
		appID, method, nonce, path, timestamp)
}

// computeHMAC calculates the HMAC signature using the configured algorithm.
func (s *Signature) computeHMAC(data []byte) string {
	return s.computeHMACWithSecret(s.secret, data)
}

// computeHMACWithSecret calculates the HMAC signature with a provided secret.
func (s *Signature) computeHMACWithSecret(secret, data []byte) string {
	switch s.algorithm {
	case SignatureAlgHmacSHA512:
		return hashx.HmacSHA512(secret, data)
	case SignatureAlgHmacSM3:
		return hashx.HmacSM3(secret, data)
	default:
		return hashx.HmacSHA256(secret, data)
	}
}

// validateTimestamp checks if the timestamp is within the allowed tolerance.
func (s *Signature) validateTimestamp(timestamp int64) error {
	if diff := time.Since(time.Unix(timestamp, 0)).Abs(); diff > s.timestampTolerance {
		return ErrSignatureExpired
	}

	return nil
}
