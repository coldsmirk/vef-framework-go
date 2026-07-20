package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Invoker executes integration calls: it resolves the target system, runs
// the bound adapter script, and returns the contract's standard model.
// Business code depends on contracts only — never on a concrete provider.
type Invoker interface {
	// Invoke executes the named contract with input against the system
	// selected by WithSystem, or resolved through the RouteResolver from the
	// WithRoute key (no target option resolves the empty route key, i.e. the
	// default route). The input is validated against the contract's input
	// schema before the script runs and the script's return value against
	// the output schema after, so the Result always carries a valid standard
	// model.
	Invoke(ctx context.Context, contract string, input any, opts ...InvokeOption) (*Result, error)
}

// InvokeOption customizes a single invocation.
type InvokeOption func(*InvokeConfig)

// InvokeConfig collects the settings resolved from InvokeOptions. It is
// consumed by the Invoker implementation; applications use the With*
// options instead of constructing it directly.
type InvokeConfig struct {
	// SystemCode targets a system directly, bypassing route resolution.
	SystemCode string
	// RouteKey selects the system through the RouteResolver; empty is the
	// default route.
	RouteKey string
	// Timeout overrides the adapter/system call timeout when positive.
	Timeout time.Duration
	// CacheTTL caches the validated output for the given duration when
	// positive; invocations are never cached otherwise.
	CacheTTL time.Duration
}

// NewInvokeConfig resolves opts into an InvokeConfig.
func NewInvokeConfig(opts ...InvokeOption) InvokeConfig {
	var cfg InvokeConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

// WithSystem targets the system with the given code directly. Mutually
// exclusive with WithRoute.
func WithSystem(code string) InvokeOption {
	return func(c *InvokeConfig) {
		c.SystemCode = code
	}
}

// WithRoute selects the system by resolving key through the RouteResolver.
// Mutually exclusive with WithSystem.
func WithRoute(key string) InvokeOption {
	return func(c *InvokeConfig) {
		c.RouteKey = key
	}
}

// WithTimeout overrides the adapter/system call timeout for this invocation.
func WithTimeout(d time.Duration) InvokeOption {
	return func(c *InvokeConfig) {
		c.Timeout = d
	}
}

// WithCache caches the validated output for ttl, keyed by system, contract,
// and input. Caching is per invocation site and off by default — opt in only
// where the business tolerates data of that age.
func WithCache(ttl time.Duration) InvokeOption {
	return func(c *InvokeConfig) {
		c.CacheTTL = ttl
	}
}

// Result is the validated outcome of an invocation.
type Result struct {
	output   any
	system   string
	duration time.Duration
	cached   bool
}

// NewResult assembles a Result. It exists for the framework's Invoker
// implementation; applications only read Results.
func NewResult(output any, system string, duration time.Duration, cached bool) *Result {
	return &Result{output: output, system: system, duration: duration, cached: cached}
}

// Output returns the standard model the adapter script produced, already
// validated against the contract's output schema.
func (r *Result) Output() any {
	return r.output
}

// Decode unmarshals the output into v through a JSON round-trip.
func (r *Result) Decode(v any) error {
	return remarshal(r.output, v, "result output")
}

// remarshal moves src into dst through a JSON round-trip; what names the
// payload in error messages.
func remarshal(src, dst any, what string) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("integration: encode %s: %w", what, err)
	}

	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("integration: decode %s: %w", what, err)
	}

	return nil
}

// System returns the code of the system that served the invocation.
func (r *Result) System() string {
	return r.system
}

// Duration returns the wall time of the invocation.
func (r *Result) Duration() time.Duration {
	return r.duration
}

// Cached reports whether the output came from the response cache.
func (r *Result) Cached() bool {
	return r.cached
}

// Call is the typed convenience wrapper over Invoker.Invoke: it decodes the
// standard model into T.
func Call[T any](ctx context.Context, inv Invoker, contract string, input any, opts ...InvokeOption) (T, error) {
	var out T

	res, err := inv.Invoke(ctx, contract, input, opts...)
	if err != nil {
		return out, err
	}

	if err := res.Decode(&out); err != nil {
		return out, err
	}

	return out, nil
}
