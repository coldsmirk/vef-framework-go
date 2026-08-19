package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Built-in inbound scheme names. They mirror the outbound vocabulary — the
// same name verifies the wire format its outbound counterpart sends; ip is
// inbound-only because a source address is only verifiable on receive.
const (
	InboundSchemeNone      = "none"
	InboundSchemeIP        = "ip"
	InboundSchemeHTTPBasic = "http_basic"
	InboundSchemeBearer    = "bearer"
	InboundSchemeHeader    = "header"
	InboundSchemeQuery     = "query"
	InboundSchemeSignature = "signature"
	InboundSchemeScript    = "script"
)

// ErrVerificationFailed is the uniform inbound credential rejection. Schemes
// return it (or wrap it) for every mismatch so callers cannot probe which
// part of a credential was wrong; configuration faults use ErrMissingParam
// instead and classify as config, not auth.
var ErrVerificationFailed = errors.New("integration inbound auth: verification failed")

// builtinInboundSchemes returns the framework-provided inbound auth schemes.
// The script scheme compiles verification bodies through its own cache.
func builtinInboundSchemes(engine *js.Engine, runTimeout time.Duration, nonceStore security.NonceStore) []integration.InboundAuthScheme {
	return []integration.InboundAuthScheme{
		new(noneInboundScheme),
		new(ipInboundScheme),
		new(httpBasicInboundScheme),
		new(bearerInboundScheme),
		new(headerInboundScheme),
		new(queryInboundScheme),
		newSignatureInboundScheme(nonceStore),
		newScriptInboundScheme(engine, runTimeout),
	}
}

// noneInboundScheme deliberately accepts every caller — the explicit opt-out
// for external systems that cannot authenticate (pair it with network-level controls).
type noneInboundScheme struct{}

func (*noneInboundScheme) Name() string {
	return InboundSchemeNone
}

func (*noneInboundScheme) Verify(context.Context, *integration.InboundRequest, *integration.InboundAuthConfig) error {
	return nil
}

func (*noneInboundScheme) SensitiveParams() []string {
	return nil
}

// ipInboundScheme verifies the caller's network address against the system's
// own whitelist (param: whitelist — comma-separated IP/CIDR entries).
type ipInboundScheme struct{}

func (*ipInboundScheme) Name() string {
	return InboundSchemeIP
}

func (*ipInboundScheme) Verify(_ context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	whitelist, err := requireParam(auth.Params, "whitelist")
	if err != nil {
		return err
	}

	validator := security.NewIPWhitelistValidator(whitelist)
	if validator.IsEmpty() {
		// An empty validator allows every IP, which would turn a misconfigured
		// whitelist into an open endpoint — treat it as a config fault.
		return fmt.Errorf("%w: whitelist", ErrMissingParam)
	}

	if req.ClientAddr == "" || !validator.IsAllowed(req.ClientAddr) {
		return fmt.Errorf("%w: client address not whitelisted", ErrVerificationFailed)
	}

	return nil
}

func (*ipInboundScheme) SensitiveParams() []string {
	return nil
}

// headerInboundScheme verifies static credential headers: each params entry
// is one header-name → expected-value pair, and the request must present
// every configured pair (AND). All values are stored encrypted — the names
// are user-defined, so sensitivity cannot be declared per parameter. It
// verifies what the outbound header scheme sends.
type headerInboundScheme struct{}

func (*headerInboundScheme) Name() string {
	return InboundSchemeHeader
}

func (*headerInboundScheme) Verify(_ context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	if len(auth.Params) == 0 {
		return fmt.Errorf("%w: at least one header", ErrMissingParam)
	}

	if !credentialPairsMatch(auth.Params, func(name string) string {
		return req.Headers[strings.ToLower(name)]
	}) {
		return fmt.Errorf("%w: header credentials mismatch", ErrVerificationFailed)
	}

	return nil
}

func (*headerInboundScheme) SensitiveParams() []string {
	return []string{integration.SensitiveAll}
}

// queryInboundScheme verifies static credential query parameters: each params
// entry is one name → expected-value pair, and the request must present every
// configured pair (AND). It verifies what the outbound query scheme sends.
type queryInboundScheme struct{}

func (*queryInboundScheme) Name() string {
	return InboundSchemeQuery
}

func (*queryInboundScheme) Verify(_ context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	if len(auth.Params) == 0 {
		return fmt.Errorf("%w: at least one query parameter", ErrMissingParam)
	}

	if !credentialPairsMatch(auth.Params, func(name string) string {
		return req.Query[name]
	}) {
		return fmt.Errorf("%w: query credentials mismatch", ErrVerificationFailed)
	}

	return nil
}

func (*queryInboundScheme) SensitiveParams() []string {
	return []string{integration.SensitiveAll}
}

// credentialPairsMatch compares every expected pair against the presented
// values, folding the outcomes so the timing does not reveal which pair
// mismatched.
func credentialPairsMatch(expected map[string]string, presented func(name string) string) bool {
	if len(expected) == 0 {
		return false
	}

	match := 1
	for name, value := range expected {
		// An empty configured value must never authenticate: ConstantTimeCompare
		// of two empty slices returns 1, so a blank credential would match an
		// absent header — a fail-open in a fail-closed design. Fold in a
		// non-empty requirement without short-circuiting the compare, so the
		// timing stays independent of the caller's presented values.
		nonEmpty := 0
		if value != "" {
			nonEmpty = 1
		}

		match &= nonEmpty & subtle.ConstantTimeCompare([]byte(value), []byte(presented(name)))
	}

	return match == 1
}

// bearerInboundScheme verifies a static bearer token — sugar over the header
// scheme for the Authorization header's "Bearer " prefix (params: token —
// sensitive). It verifies what the outbound bearer scheme sends.
type bearerInboundScheme struct{}

func (*bearerInboundScheme) Name() string {
	return InboundSchemeBearer
}

func (*bearerInboundScheme) Verify(_ context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	token, err := requireParam(auth.Params, "token")
	if err != nil {
		return err
	}

	presented, ok := extractBearerToken(req.Headers["authorization"])
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(presented)) != 1 {
		return fmt.Errorf("%w: bearer token mismatch", ErrVerificationFailed)
	}

	return nil
}

func (*bearerInboundScheme) SensitiveParams() []string {
	return []string{"token"}
}

// extractBearerToken pulls the token from an "Authorization: Bearer" header
// value, case-insensitive on the scheme.
func extractBearerToken(header string) (token string, ok bool) {
	const prefix = "Bearer "

	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}

	return header[len(prefix):], true
}

// httpBasicInboundScheme verifies RFC 7617 Basic credentials
// (params: username, password — sensitive).
type httpBasicInboundScheme struct{}

func (*httpBasicInboundScheme) Name() string {
	return InboundSchemeHTTPBasic
}

func (*httpBasicInboundScheme) Verify(_ context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	username, err := requireParam(auth.Params, "username")
	if err != nil {
		return err
	}

	password, err := requireParam(auth.Params, "password")
	if err != nil {
		return err
	}

	presentedUser, presentedPass, ok := decodeBasicAuthorization(req.Headers["authorization"])
	if !ok {
		return fmt.Errorf("%w: malformed basic credentials", ErrVerificationFailed)
	}

	// Compare both parts unconditionally so the outcome timing does not
	// reveal whether the username matched.
	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(presentedUser))
	passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(presentedPass))

	if userMatch&passMatch != 1 {
		return fmt.Errorf("%w: basic credentials mismatch", ErrVerificationFailed)
	}

	return nil
}

func (*httpBasicInboundScheme) SensitiveParams() []string {
	return []string{"password"}
}

// decodeBasicAuthorization extracts the credentials from an
// "Authorization: Basic" header value, case-insensitive on the scheme.
func decodeBasicAuthorization(header string) (username, password string, ok bool) {
	const prefix = "Basic "

	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(header[len(prefix):])
	if err != nil {
		return "", "", false
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found || username == "" {
		return "", "", false
	}

	return username, password, true
}

// signatureInboundScheme verifies the framework's HMAC signature convention —
// x-timestamp / x-nonce / x-signature headers signed over the system code,
// method, and path (params: secret — hex-encoded, sensitive). It reuses the
// security package verifier, replay protection included.
type signatureInboundScheme struct {
	verifier *security.Signature
}

func newSignatureInboundScheme(nonceStore security.NonceStore) *signatureInboundScheme {
	// The verifier always authenticates with the per-system secret via
	// VerifyWithSecret; "00" is a syntactically valid placeholder that never
	// computes an HMAC (mirroring the api signature authenticator). The nonce
	// store is the framework-shared one, so replay protection holds across
	// nodes once it is swapped for the Redis store. A nil store is left to
	// NewSignature's in-memory default rather than passed through
	// WithNonceStore(nil), which would disable replay protection.
	var options []security.SignatureOption
	if nonceStore != nil {
		options = append(options, security.WithNonceStore(nonceStore))
	}

	verifier, err := security.NewSignature("00", options...)
	if err != nil {
		panic(err)
	}

	return &signatureInboundScheme{verifier: verifier}
}

func (*signatureInboundScheme) Name() string {
	return InboundSchemeSignature
}

func (s *signatureInboundScheme) Verify(ctx context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	secret, err := requireParam(auth.Params, "secret")
	if err != nil {
		return err
	}

	timestamp, err := strconv.ParseInt(req.Headers["x-timestamp"], 10, 64)
	if err != nil {
		return fmt.Errorf("%w: malformed timestamp", ErrVerificationFailed)
	}

	err = s.verifier.VerifyWithSecret(ctx, secret,
		security.SignatureRequest{AppID: req.SystemCode, Method: req.Method, Path: req.Path},
		security.SignatureCredentials{
			Timestamp: timestamp,
			Nonce:     req.Headers["x-nonce"],
			Signature: req.Headers["x-signature"],
		})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrVerificationFailed, err)
	}

	return nil
}

func (*signatureInboundScheme) SensitiveParams() []string {
	return []string{"secret"}
}

// scriptInboundScheme runs the system's custom verification body
// (InboundAuthConfig.Script): the most flexible tier, for external conventions
// no declarative scheme covers. The runtime deliberately carries no IO
// capability — only the engine baseline plus the request and params bindings
// — so a script can read decrypted secrets but has no channel to leak them;
// access is granted by returning a truthy value, and script errors stay
// server-side.
type scriptInboundScheme struct {
	engine     *js.Engine
	programs   *definition.ProgramCache
	runTimeout time.Duration
}

func newScriptInboundScheme(engine *js.Engine, runTimeout time.Duration) *scriptInboundScheme {
	return &scriptInboundScheme{
		engine:     engine,
		programs:   definition.NewProgramCache(definition.CompileScript),
		runTimeout: runTimeout,
	}
}

func (*scriptInboundScheme) Name() string {
	return InboundSchemeScript
}

func (s *scriptInboundScheme) Verify(ctx context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	if auth.Script == "" {
		return fmt.Errorf("%w: script", ErrMissingParam)
	}

	program, err := s.programs.Get(auth.Script)
	if err != nil {
		return err
	}

	runtime, err := s.engine.NewRuntime(js.WithRunTimeout(s.runTimeout))
	if err != nil {
		return err
	}

	params := auth.Params
	if params == nil {
		params = map[string]string{}
	}

	if err := runtime.Set("request", InboundRequestBinding(req)); err != nil {
		return err
	}

	if err := runtime.Set("params", params); err != nil {
		return err
	}

	value, err := runtime.RunProgram(ctx, program)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrVerificationFailed, err)
	}

	if value == nil || !value.ToBoolean() {
		return fmt.Errorf("%w: script denied access", ErrVerificationFailed)
	}

	return nil
}

func (*scriptInboundScheme) SensitiveParams() []string {
	return []string{integration.SensitiveAll}
}

// InboundRequestBinding is the read-only view of the request exposed to
// verification and adapter scripts.
func InboundRequestBinding(req *integration.InboundRequest) map[string]any {
	headers := req.Headers
	if headers == nil {
		headers = map[string]string{}
	}

	query := req.Query
	if query == nil {
		query = map[string]string{}
	}

	return map[string]any{
		"protocol":   req.Protocol,
		"method":     req.Method,
		"path":       req.Path,
		"headers":    headers,
		"query":      query,
		"body":       string(req.Body),
		"clientAddr": req.ClientAddr,
	}
}
