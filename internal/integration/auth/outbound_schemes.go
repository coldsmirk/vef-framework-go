package auth

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Built-in outbound scheme names. They mirror the inbound vocabulary — the
// same name authenticates the same wire format in the opposite direction; ip
// stays inbound-only because a source address is only verifiable on receive.
const (
	OutboundSchemeNone      = "none"
	OutboundSchemeHTTPBasic = "http_basic"
	OutboundSchemeBearer    = "bearer"
	OutboundSchemeHeader    = "header"
	OutboundSchemeQuery     = "query"
	OutboundSchemeSignature = "signature"
	OutboundSchemeScript    = "script"
)

// builtinOutboundSchemes returns the framework-provided auth schemes. The
// script scheme compiles signing bodies through its own cache.
func builtinOutboundSchemes(engine *js.Engine, runTimeout time.Duration) []integration.OutboundAuthScheme {
	return []integration.OutboundAuthScheme{
		new(noneOutboundScheme),
		new(httpBasicOutboundScheme),
		new(bearerOutboundScheme),
		new(headerOutboundScheme),
		new(queryOutboundScheme),
		new(signatureOutboundScheme),
		newScriptOutboundScheme(engine, runTimeout),
	}
}

// requireParam returns the named parameter or ErrMissingParam when absent or
// empty.
func requireParam(params map[string]string, name string) (string, error) {
	value := params[name]
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingParam, name)
	}

	return value, nil
}

// OutboundAuthError marks a failure inside an outbound auth scheme's request
// hook (signing script, signature computation). The invoker classifies it as
// a configuration fault — the system's auth definition is broken — rather
// than a transport failure of the upstream.
type OutboundAuthError struct {
	err error
}

func (e *OutboundAuthError) Error() string {
	return e.err.Error()
}

func (e *OutboundAuthError) Unwrap() error {
	return e.err
}

// noneOutboundScheme sends requests unauthenticated.
type noneOutboundScheme struct{}

func (*noneOutboundScheme) Name() string {
	return OutboundSchemeNone
}

func (*noneOutboundScheme) Apply(*integration.OutboundAuthConfig) ([]httpx.Option, error) {
	return nil, nil
}

func (*noneOutboundScheme) SensitiveParams() []string {
	return nil
}

// httpBasicOutboundScheme authenticates with RFC 7617 Basic credentials
// (params: username, password).
type httpBasicOutboundScheme struct{}

func (*httpBasicOutboundScheme) Name() string {
	return OutboundSchemeHTTPBasic
}

func (*httpBasicOutboundScheme) Apply(cfg *integration.OutboundAuthConfig) ([]httpx.Option, error) {
	username, err := requireParam(cfg.Params, "username")
	if err != nil {
		return nil, err
	}

	password, err := requireParam(cfg.Params, "password")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithBasicAuth(username, password)}, nil
}

func (*httpBasicOutboundScheme) SensitiveParams() []string {
	return []string{"password"}
}

// bearerOutboundScheme authenticates with a static bearer token — sugar over
// the header scheme for the Authorization header's "Bearer " prefix
// (params: token).
type bearerOutboundScheme struct{}

func (*bearerOutboundScheme) Name() string {
	return OutboundSchemeBearer
}

func (*bearerOutboundScheme) Apply(cfg *integration.OutboundAuthConfig) ([]httpx.Option, error) {
	token, err := requireParam(cfg.Params, "token")
	if err != nil {
		return nil, err
	}

	return []httpx.Option{httpx.WithBearerToken(token)}, nil
}

func (*bearerOutboundScheme) SensitiveParams() []string {
	return []string{"token"}
}

// headerOutboundScheme sends every configured parameter as a static credential
// header: each params entry is one header-name → value pair, so a system
// needing several credential headers configures several entries. All values
// are stored encrypted — the names are user-defined, so sensitivity cannot be
// declared per parameter.
type headerOutboundScheme struct{}

func (*headerOutboundScheme) Name() string {
	return OutboundSchemeHeader
}

func (*headerOutboundScheme) Apply(cfg *integration.OutboundAuthConfig) ([]httpx.Option, error) {
	if len(cfg.Params) == 0 {
		return nil, fmt.Errorf("%w: at least one header", ErrMissingParam)
	}

	opts := make([]httpx.Option, 0, len(cfg.Params))
	for name, value := range cfg.Params {
		opts = append(opts, httpx.WithHeader(name, value))
	}

	return opts, nil
}

func (*headerOutboundScheme) SensitiveParams() []string {
	return []string{integration.SensitiveAll}
}

// queryOutboundScheme sends every configured parameter as a static credential
// query parameter: each params entry is one name → value pair, mirroring the
// header scheme for APIs that expect credentials in the query string.
type queryOutboundScheme struct{}

func (*queryOutboundScheme) Name() string {
	return OutboundSchemeQuery
}

func (*queryOutboundScheme) Apply(cfg *integration.OutboundAuthConfig) ([]httpx.Option, error) {
	if len(cfg.Params) == 0 {
		return nil, fmt.Errorf("%w: at least one query parameter", ErrMissingParam)
	}

	opts := make([]httpx.Option, 0, len(cfg.Params))
	for name, value := range cfg.Params {
		opts = append(opts, httpx.WithQuery(name, value))
	}

	return opts, nil
}

func (*queryOutboundScheme) SensitiveParams() []string {
	return []string{integration.SensitiveAll}
}

// signatureOutboundScheme signs every request with the framework's HMAC
// signature convention — x-timestamp / x-nonce / x-signature headers over the
// configured identity, method, and path (params: appId — the identity the
// receiver knows the caller by, e.g. the system code its registry assigned;
// secret — hex-encoded, sensitive). It is the sending counterpart of the
// inbound signature scheme, so two framework deployments authenticate each
// other with configuration only; vendor-specific signing belongs to the
// script scheme.
type signatureOutboundScheme struct{}

func (*signatureOutboundScheme) Name() string {
	return OutboundSchemeSignature
}

func (*signatureOutboundScheme) Apply(cfg *integration.OutboundAuthConfig) ([]httpx.Option, error) {
	appID, err := requireParam(cfg.Params, "appId")
	if err != nil {
		return nil, err
	}

	secret, err := requireParam(cfg.Params, "secret")
	if err != nil {
		return nil, err
	}

	signer, err := security.NewSignature(secret)
	if err != nil {
		return nil, err
	}

	hook := func(req *httpx.Request) error {
		parsed, err := url.Parse(req.URL())
		if err != nil {
			return &OutboundAuthError{err: err}
		}

		result, err := signer.Sign(appID, req.Method(), parsed.Path)
		if err != nil {
			return &OutboundAuthError{err: err}
		}

		req.SetHeader("x-timestamp", strconv.FormatInt(result.Timestamp, 10))
		req.SetHeader("x-nonce", result.Nonce)
		req.SetHeader("x-signature", result.Signature)

		return nil
	}

	return []httpx.Option{httpx.WithRequestHook(hook)}, nil
}

func (*signatureOutboundScheme) SensitiveParams() []string {
	return []string{"secret"}
}

// scriptOutboundScheme runs the system's custom signing body
// (OutboundAuthConfig.Script) on every request: the most flexible tier, for
// vendor signing conventions no declarative scheme covers. The runtime
// deliberately carries no IO capability — only the engine baseline plus the
// built request and the decrypted params — so a script can read secrets but
// has no channel to leak them. The script returns an object of credential
// headers to add; returning nothing adds none.
type scriptOutboundScheme struct {
	engine     *js.Engine
	programs   *definition.ProgramCache
	runTimeout time.Duration
}

func newScriptOutboundScheme(engine *js.Engine, runTimeout time.Duration) *scriptOutboundScheme {
	return &scriptOutboundScheme{
		engine:     engine,
		programs:   definition.NewProgramCache(definition.CompileScript),
		runTimeout: runTimeout,
	}
}

func (*scriptOutboundScheme) Name() string {
	return OutboundSchemeScript
}

func (s *scriptOutboundScheme) Apply(cfg *integration.OutboundAuthConfig) ([]httpx.Option, error) {
	if cfg.Script == "" {
		return nil, fmt.Errorf("%w: script", ErrMissingParam)
	}

	program, err := s.programs.Get(cfg.Script)
	if err != nil {
		return nil, err
	}

	params := cfg.Params
	if params == nil {
		params = map[string]string{}
	}

	hook := func(req *httpx.Request) error {
		headers, err := s.run(req, program, params)
		if err != nil {
			return &OutboundAuthError{err: err}
		}

		for name, value := range headers {
			req.SetHeader(name, cast.ToString(value))
		}

		return nil
	}

	return []httpx.Option{httpx.WithRequestHook(hook)}, nil
}

// run executes the signing body in a fresh zero-IO runtime — hooks fire
// concurrently across invocations sharing one cached client, and a js.Runtime
// is single-goroutine.
func (s *scriptOutboundScheme) run(req *httpx.Request, program *js.Program, params map[string]string) (map[string]any, error) {
	runtime, err := s.engine.NewRuntime(js.WithRunTimeout(s.runTimeout))
	if err != nil {
		return nil, err
	}

	if err := runtime.Set("request", outboundRequestBinding(req)); err != nil {
		return nil, err
	}

	if err := runtime.Set("params", params); err != nil {
		return nil, err
	}

	value, err := runtime.RunProgram(req.Context(), program)
	if err != nil {
		return nil, err
	}

	if value == nil || js.IsUndefined(value) || js.IsNull(value) {
		return nil, nil
	}

	headers, ok := value.Export().(map[string]any)
	if !ok {
		return nil, ErrSigningScriptReturn
	}

	return headers, nil
}

func (*scriptOutboundScheme) SensitiveParams() []string {
	return []string{integration.SensitiveAll}
}

// outboundRequestBinding is the read-only view of the built request exposed
// to signing scripts: header names lowercased and the query parsed out of the
// resolved URL, matching the inbound request binding conventions.
func outboundRequestBinding(req *httpx.Request) map[string]any {
	headers := make(map[string]string, len(req.Headers()))
	for name, values := range req.Headers() {
		if len(values) > 0 {
			headers[strings.ToLower(name)] = strings.Join(values, ", ")
		}
	}

	binding := map[string]any{
		"method":  req.Method(),
		"url":     req.URL(),
		"path":    "",
		"query":   map[string]string{},
		"headers": headers,
		"body":    string(req.Body()),
	}

	if parsed, err := url.Parse(req.URL()); err == nil {
		binding["path"] = parsed.Path

		query := make(map[string]string, len(parsed.Query()))
		for name, values := range parsed.Query() {
			if len(values) > 0 {
				query[name] = strings.Join(values, ", ")
			}
		}

		binding["query"] = query
	}

	return binding
}
