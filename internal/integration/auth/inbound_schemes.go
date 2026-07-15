package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Built-in inbound scheme names. The "none" and "http_basic" wire formats
// match their outbound / api-strategy counterparts; the names differ where
// the verification semantics differ.
const (
	InboundSchemeNone      = "none"
	InboundSchemeIP        = "ip"
	InboundSchemeAPIKey    = "api_key"
	InboundSchemeHTTPBasic = "http_basic"
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
func builtinInboundSchemes(engine *js.Engine) []integration.InboundAuthScheme {
	return []integration.InboundAuthScheme{
		new(noneInboundScheme),
		new(ipInboundScheme),
		new(apiKeyInboundScheme),
		new(httpBasicInboundScheme),
		newSignatureInboundScheme(),
		newScriptInboundScheme(engine),
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

// apiKeyInboundScheme verifies a static key presented in a request header
// (params: key — sensitive; header — optional, defaults to x-api-key).
type apiKeyInboundScheme struct{}

func (*apiKeyInboundScheme) Name() string {
	return InboundSchemeAPIKey
}

func (*apiKeyInboundScheme) Verify(_ context.Context, req *integration.InboundRequest, auth *integration.InboundAuthConfig) error {
	key, err := requireParam(auth.Params, "key")
	if err != nil {
		return err
	}

	header := auth.Params["header"]
	if header == "" {
		header = "x-api-key"
	}

	presented := req.Headers[strings.ToLower(header)]
	if presented == "" || subtle.ConstantTimeCompare([]byte(key), []byte(presented)) != 1 {
		return fmt.Errorf("%w: api key mismatch", ErrVerificationFailed)
	}

	return nil
}

func (*apiKeyInboundScheme) SensitiveParams() []string {
	return []string{"key"}
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

func newSignatureInboundScheme() *signatureInboundScheme {
	// The verifier always authenticates with the per-system secret via
	// VerifyWithSecret; "00" is a syntactically valid placeholder that never
	// computes an HMAC (mirroring the api signature authenticator).
	verifier, err := security.NewSignature("00")
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
		req.SystemCode, req.Method, req.Path,
		timestamp, req.Headers["x-nonce"], req.Headers["x-signature"])
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
	engine   *js.Engine
	programs *definition.ProgramCache
}

func newScriptInboundScheme(engine *js.Engine) *scriptInboundScheme {
	return &scriptInboundScheme{engine: engine, programs: definition.NewProgramCache()}
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

	runtime, err := s.engine.NewRuntime()
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
