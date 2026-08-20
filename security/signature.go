package security

import (
	"context"
	"crypto/hmac"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/id"
)

// SignatureRequest identifies what a signature covers. Every field is folded
// into the signed payload, so signer and verifier must describe the request
// identically (see buildPayload for the canonical rendering).
type SignatureRequest struct {
	// AppID names the external application whose secret signs the request.
	AppID string
	// Method and Path are the request's HTTP method and path, bound so a
	// captured signature cannot be replayed against a different endpoint.
	Method string
	Path   string
	// BoundParams are the caller's own parameters to cover — the user
	// identifier and redirect target of a trust-login handoff, for instance.
	// Values are the decoded ones, never their URL-encoded wire form, and no
	// key may collide with the fixed payload fields (ErrSignatureBoundKeyReserved).
	BoundParams map[string]string
}

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
// Defaults to an in-memory store; pass WithNonceStore(nil) to disable nonce validation.
func WithNonceStore(store NonceStore) SignatureOption {
	return func(s *Signature) {
		s.nonceStore = store
	}
}

// signatureFixedKeys are the payload keys a Signature always contributes. A
// caller-supplied bound parameter may not reuse one: the payload renders each
// key exactly once, so a duplicate would silently shadow the framework's own
// value — a bound "path" could unbind the endpoint the signature covers.
var signatureFixedKeys = collections.NewHashSetFrom("app_id", "method", "nonce", "path", "timestamp")

// Signature provides HMAC-based signature generation and verification.
// It handles timestamp validation and binds caller-supplied parameters into the
// signed payload (see buildPayload).
type Signature struct {
	secret             []byte
	algorithm          SignatureAlgorithm
	timestampTolerance time.Duration
	nonceGenerator     id.IDGenerator
	nonceStore         NonceStore
	// clock is the time source for signing and timestamp validation. It
	// defaults to time.Now; tests override it to drive the replay window
	// deterministically without real-time waits.
	clock func() time.Time
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
		nonceStore:         NewMemoryNonceStore(),
		nonceGenerator:     id.NewRandomIDGenerator(),
		clock:              time.Now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Sign generates a signature covering request. Callers describe the request
// exactly as the server will see it — the same method and path, and bound
// parameters holding the same decoded values. Returns a SignatureResult
// carrying the freshly minted timestamp, nonce, and signature.
func (s *Signature) Sign(request SignatureRequest) (*SignatureResult, error) {
	if request.AppID == "" {
		return nil, ErrAppIDRequired
	}

	if err := validateBoundKeys(request.BoundParams); err != nil {
		return nil, err
	}

	nonce := s.nonceGenerator.Generate()
	timestampSec := s.clock().Unix()
	signature := s.computeHMAC(s.buildPayload(request, timestampSec, nonce))

	return &SignatureResult{
		AppID:     request.AppID,
		Timestamp: timestampSec,
		Nonce:     nonce,
		Signature: signature,
	}, nil
}

// Verify validates credentials against request, all of which must describe what
// was signed. Returns nil if valid, or an error describing the validation
// failure.
func (s *Signature) Verify(ctx context.Context, request SignatureRequest, credentials SignatureCredentials) error {
	return s.verifyWithSecret(ctx, s.secret, request, credentials)
}

// VerifyWithSecret validates the signature using an externally provided secret.
// This is useful when the secret is loaded dynamically per-request (e.g., from ExternalAppLoader).
// The secret parameter expects a hex-encoded string.
func (s *Signature) VerifyWithSecret(ctx context.Context, secret string, request SignatureRequest, credentials SignatureCredentials) error {
	secretBytes, err := hex.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecodeSignatureSecretFailed, err)
	}

	return s.verifyWithSecret(ctx, secretBytes, request, credentials)
}

// verifyWithSecret is the internal implementation for signature verification.
func (s *Signature) verifyWithSecret(ctx context.Context, secret []byte, request SignatureRequest, credentials SignatureCredentials) error {
	if request.AppID == "" {
		return ErrAppIDRequired
	}

	if credentials.Nonce == "" {
		return ErrNonceRequired
	}

	if credentials.Signature == "" {
		return ErrSignatureRequired
	}

	if err := validateBoundKeys(request.BoundParams); err != nil {
		return err
	}

	if err := s.validateTimestamp(credentials.Timestamp); err != nil {
		return err
	}

	payload := s.buildPayload(request, credentials.Timestamp, credentials.Nonce)
	expectedSignature := s.computeHMACWithSecret(secret, payload)

	// computeHMACWithSecret returns lower-case hex.EncodeToString output. Lower-
	// case the client-provided signature so upper-case hex (some third-party SDKs
	// emit it) still verifies, then compare in constant time. strings.ToLower is
	// input-independent, so it introduces no timing oracle on the secret.
	if !hmac.Equal([]byte(expectedSignature), []byte(strings.ToLower(credentials.Signature))) {
		return ErrSignatureInvalid
	}

	return s.checkAndStoreNonce(ctx, request.AppID, credentials.Nonce)
}

// checkAndStoreNonce atomically stores a nonce and rejects replays if NonceStore is configured.
func (s *Signature) checkAndStoreNonce(ctx context.Context, appID, nonce string) error {
	if s.nonceStore == nil {
		return nil
	}

	stored, err := s.nonceStore.StoreIfAbsent(ctx, appID, nonce, s.nonceTTL())
	if err != nil {
		return fmt.Errorf("failed to store nonce: %w", err)
	}

	if !stored {
		return ErrNonceAlreadyUsed
	}

	return nil
}

// buildPayload assembles the canonical string that is HMAC-signed: every
// parameter rendered as key=value and joined by "&" in ascending key order.
//
// The request body is intentionally NOT covered: it is large and any benign
// re-serialization (key ordering, whitespace) would break otherwise-valid
// requests; replay of the same endpoint is already prevented by the nonce +
// timestamp.
//
// The fixed keys are already ascending among themselves, so a request with no
// bound parameters reproduces the payload of a scheme that covers nothing else.
//
// Bound keys and values are percent-encoded, the fixed ones are not. The
// asymmetry is the point on both sides. Bound parameters are caller-supplied and
// arbitrary — a signed redirect URL routinely carries "&" and "=" — and rendered
// raw they make the payload ambiguous: {"x": "1&y=2", "y": "3"} and
// {"x": "1", "y": "2&y=3"} both flatten to x=1&y=2&y=3, so a signature minted
// for one verifies the other. Encoding the fixed values instead would change the
// string every third party already generates for plain API signature auth, which
// the byte-for-byte lock in TestSignatureBoundParameters exists to prevent.
func (*Signature) buildPayload(request SignatureRequest, timestamp int64, nonce string) []byte {
	params := map[string]string{
		"app_id":    request.AppID,
		"method":    request.Method,
		"nonce":     nonce,
		"path":      request.Path,
		"timestamp": strconv.FormatInt(timestamp, 10),
	}

	// Safe to overlay: validateBoundKeys has already rejected any collision,
	// and percent-encoding is injective over unreserved keys, so an encoded
	// bound key can only equal a fixed key if the raw one already did.
	for key, value := range request.BoundParams {
		params[encodeSignatureComponent(key)] = encodeSignatureComponent(value)
	}

	var payload []byte

	for i, key := range slices.Sorted(maps.Keys(params)) {
		if i > 0 {
			payload = append(payload, '&')
		}

		payload = fmt.Appendf(payload, "%s=%s", key, params[key])
	}

	return payload
}

// encodeSignatureComponent percent-encodes one bound key or value for the
// canonical payload.
//
// The rule is RFC 3986 verbatim so third parties can reproduce it in any
// language: every byte outside the unreserved set (A-Z a-z 0-9 - . _ ~) becomes
// %XX with uppercase hex digits, a space included. It is deliberately not
// url.QueryEscape — that renders a space as "+" and is a URL-form convention
// rather than a signing one — and deliberately not encodeURIComponent, which
// leaves !'()* unescaped. AWS SigV4 canonicalizes the same way.
func encodeSignatureComponent(s string) string {
	const upperhex = "0123456789ABCDEF"

	var builder strings.Builder

	for i := range len(s) {
		c := s[i]
		if isUnreservedByte(c) {
			builder.WriteByte(c)

			continue
		}

		builder.WriteByte('%')
		builder.WriteByte(upperhex[c>>4])
		builder.WriteByte(upperhex[c&0x0f])
	}

	return builder.String()
}

// isUnreservedByte reports whether c is an RFC 3986 unreserved character, the
// only bytes encodeSignatureComponent passes through untouched.
func isUnreservedByte(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-', c == '.', c == '_', c == '~':
		return true
	default:
		return false
	}
}

// validateBoundKeys rejects bound parameters that would collide with the
// payload's fixed keys.
func validateBoundKeys(bound map[string]string) error {
	for _, key := range slices.Sorted(maps.Keys(bound)) {
		if signatureFixedKeys.Contains(key) {
			return fmt.Errorf("%w: %q", ErrSignatureBoundKeyReserved, key)
		}
	}

	return nil
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
	if diff := s.clock().Sub(time.Unix(timestamp, 0)).Abs(); diff > s.timestampTolerance {
		return ErrSignatureExpired
	}

	return nil
}

// nonceTTL is how long a verified nonce must be retained to fully cover a
// request's replay window. validateTimestamp accepts any timestamp within
// ±timestampTolerance of now, so a single signed request stays valid for the
// whole 2*timestampTolerance span [ts-tolerance, ts+tolerance]. A nonce first
// seen at the earliest accepting moment (ts-tolerance) must therefore live until
// the latest (ts+tolerance) — i.e. 2*timestampTolerance — or a maximally
// future-dated request could be replayed after the nonce expired but while its
// timestamp was still fresh. nonceTTLBuffer adds margin for store/clock jitter.
func (s *Signature) nonceTTL() time.Duration {
	return 2*s.timestampTolerance + nonceTTLBuffer
}
