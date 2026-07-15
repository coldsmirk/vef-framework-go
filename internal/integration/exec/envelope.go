package exec

import (
	"context"

	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/js"
)

// envelopePrograms caches the compiled envelope scripts of both directions,
// keyed by content hash like every other definition-derived artifact — a
// saved envelope edit takes effect on the next invocation.
type envelopePrograms struct {
	requests  *definition.ProgramCache
	responses *definition.ProgramCache
}

func newEnvelopePrograms() *envelopePrograms {
	return &envelopePrograms{
		requests:  definition.NewProgramCache(definition.CompileEnvelopeRequestScript),
		responses: definition.NewProgramCache(definition.CompileEnvelopeResponseScript),
	}
}

// materialize evaluates cfg's scripts into runtime and returns the callable
// envelope, or nil when the system carries none. Failures are configuration
// faults: the definition was saved broken.
func (p *envelopePrograms) materialize(ctx context.Context, runtime *js.Runtime, cfg *integration.OutboundEnvelopeConfig) (*envelope, error) {
	if cfg == nil {
		return nil, nil
	}

	env := new(envelope)

	if cfg.Request != "" {
		fn, err := evaluateEnvelopeFunction(ctx, runtime, p.requests, cfg.Request)
		if err != nil {
			return nil, err
		}

		env.wrapRequest = fn
	}

	if cfg.Response != "" {
		fn, err := evaluateEnvelopeFunction(ctx, runtime, p.responses, cfg.Response)
		if err != nil {
			return nil, err
		}

		env.unwrapResponse = fn
	}

	return env, nil
}

// evaluateEnvelopeFunction compiles script through programs and evaluates the
// function expression it wraps into runtime.
func evaluateEnvelopeFunction(ctx context.Context, runtime *js.Runtime, programs *definition.ProgramCache, script string) (js.Func, error) {
	program, err := programs.Get(script)
	if err != nil {
		return nil, integration.ErrInvalidEnvelope(err.Error())
	}

	value, err := runtime.RunProgram(ctx, program)
	if err != nil {
		return nil, integration.ErrInvalidEnvelope(err.Error())
	}

	fn, ok := runtime.AsFunction(value)
	if !ok {
		return nil, integration.ErrInvalidEnvelope("script did not evaluate to a function")
	}

	return fn, nil
}

// envelope holds a system's wrap/unwrap functions evaluated into one runtime;
// a nil side passes through untouched.
type envelope struct {
	wrapRequest    js.Func
	unwrapResponse js.Func
}

// wireRequest is the logical outbound request an envelope script wraps: the
// adapter's request before serialization.
type wireRequest struct {
	method  string
	path    string
	headers map[string]string
	query   map[string]string
	body    any
}

// applyRequest runs the wrap script over req and returns the request to put
// on the wire. The script receives `request` as { method, path, headers,
// query, body } and returns the same shape; fields it omits keep the
// adapter's values. A script exception propagates to the adapter's call.
func (e *envelope) applyRequest(req *wireRequest) (*wireRequest, error) {
	if e == nil || e.wrapRequest == nil {
		return req, nil
	}

	value, err := e.wrapRequest(map[string]any{
		"method":  req.method,
		"path":    req.path,
		"headers": widenStringMap(req.headers),
		"query":   widenStringMap(req.query),
		"body":    req.body,
	})
	if err != nil {
		return nil, err
	}

	if value == nil || js.IsUndefined(value) || js.IsNull(value) {
		return nil, ErrEnvelopeRequestNotObject
	}

	exported, ok := value.Export().(map[string]any)
	if !ok {
		return nil, ErrEnvelopeRequestNotObject
	}

	wire := *req
	if v, ok := exported["method"]; ok {
		wire.method = cast.ToString(v)
	}

	if v, ok := exported["path"]; ok {
		wire.path = cast.ToString(v)
	}

	if v, ok := exported["headers"]; ok {
		wire.headers = cast.ToStringMapString(v)
	}

	if v, ok := exported["query"]; ok {
		wire.query = cast.ToStringMapString(v)
	}

	if v, ok := exported["body"]; ok {
		wire.body = v
	}

	return &wire, nil
}

// applyResponse runs the unwrap script over the response object; whatever it
// returns is what the adapter's call yields. A script exception (including
// errors.upstream for vendor-level error codes) propagates to the adapter's
// call.
func (e *envelope) applyResponse(response map[string]any) (any, error) {
	if e == nil || e.unwrapResponse == nil {
		return response, nil
	}

	value, err := e.unwrapResponse(response)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// widenStringMap copies a string map into the plain object shape scripts
// mutate freely; the envelope contract reads values back through cast.
func widenStringMap(m map[string]string) map[string]any {
	widened := make(map[string]any, len(m))
	for k, v := range m {
		widened[k] = v
	}

	return widened
}
