package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/datasource"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jssql"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Invoker executes integration calls end to end: target resolution, input
// validation, script execution against the system-scoped client, output
// validation, and outcome recording. It implements integration.Invoker and
// integration.StatsInspector.
type Invoker struct {
	db       orm.DB
	engine   *js.Engine
	resolver integration.RouteResolver
	cfg      *config.IntegrationConfig

	programs  *definition.ProgramCache
	schemas   *schemaCache
	clients   *clientFactory
	databases *systemDatabases
	responses *responseCache
	stats     *statsRecorder
	recorder  *logRecorder
	capturer  *capturer
}

// NewInvoker assembles the invoker and its caches.
func NewInvoker(
	db orm.DB,
	engine *js.Engine,
	registry *auth.OutboundRegistry,
	codec *definition.SecretCodec,
	resolver integration.RouteResolver,
	sources datasource.Registry,
	cfg *config.IntegrationConfig,
) *Invoker {
	return &Invoker{
		db:        db,
		engine:    engine,
		resolver:  resolver,
		cfg:       cfg,
		programs:  definition.NewProgramCache(),
		schemas:   newSchemaCache(),
		clients:   newClientFactory(registry, codec, cfg.EffectiveMaxResponseBody()),
		databases: newSystemDatabases(sources, codec),
		responses: newResponseCache(),
		stats:     newStatsRecorder(),
		recorder:  newLogRecorder(db, cfg),
		capturer:  newCapturer(&cfg.Log),
	}
}

// ReleaseSystem drops the datasource registry entry of a deleted system (or
// one whose data source configuration was removed or renamed).
func (inv *Invoker) ReleaseSystem(ctx context.Context, systemCode string) error {
	return inv.databases.Release(ctx, systemCode)
}

// Invoke implements integration.Invoker.
func (inv *Invoker) Invoke(ctx context.Context, contract string, input any, opts ...integration.InvokeOption) (*integration.Result, error) {
	cfg := integration.NewInvokeConfig(opts...)
	if cfg.SystemCode != "" && cfg.RouteKey != "" {
		return nil, integration.ErrTargetAmbiguous
	}

	loadedContract, err := inv.loadContract(ctx, contract)
	if err != nil {
		return nil, err
	}

	systemCode := cfg.SystemCode
	if systemCode == "" {
		if systemCode, err = inv.resolver.Resolve(ctx, contract, cfg.RouteKey); err != nil {
			return nil, err
		}
	}

	system, err := inv.loadSystem(ctx, systemCode)
	if err != nil {
		return nil, err
	}

	adapter, err := inv.loadAdapter(ctx, system.ID, loadedContract.ID, integration.DirectionOutbound)
	if err != nil {
		return nil, err
	}

	inputValue, err := canonicalize(input)
	if err != nil {
		return nil, fmt.Errorf("integration: input is not JSON-serializable: %w", err)
	}

	start := time.Now()

	// A cache hit represents no upstream interaction, so it bypasses stats
	// and the invocation log — both track upstream health.
	var cacheKey string
	if cfg.CacheTTL > 0 {
		cacheKey = responseCacheKey(system.Code, contract, inputValue)
		if output, ok := inv.responses.Get(ctx, cacheKey); ok {
			return integration.NewResult(output, system.Code, time.Since(start), true), nil
		}
	}

	execution := &execution{
		contract:   loadedContract,
		system:     system,
		script:     adapter.Script,
		runTimeout: inv.runTimeout(&cfg, adapter),
		input:      inputValue,
	}

	output, trace, kind, execErr := inv.run(ctx, execution)
	duration := time.Since(start)

	inv.finish(ctx, &outcome{
		system:    system.Code,
		contract:  contract,
		direction: integration.DirectionOutbound,
		kind:      kind,
		err:       execErr,
		duration:  duration,
		input:     inputValue,
		output:    output,
		trace:     trace,
	})

	if execErr != nil {
		return nil, execErr
	}

	if cfg.CacheTTL > 0 {
		inv.responses.Set(ctx, cacheKey, output, cfg.CacheTTL)
	}

	return integration.NewResult(output, system.Code, duration, false), nil
}

// Stats implements integration.StatsInspector.
func (inv *Invoker) Stats() []integration.InvocationStats {
	return inv.stats.Stats()
}

// DryRunResult is the outcome of a DryRun: the trace is populated even when
// the run failed, so operators see how far the script got.
type DryRunResult struct {
	Output      any                        `json:"output"`
	Trace       []integration.HTTPExchange `json:"trace"`
	FailureKind integration.FailureKind    `json:"failureKind,omitempty"`
	Error       string                     `json:"error,omitempty"`
}

// DryRun executes script (possibly unsaved) against system under contract,
// bypassing the adapter table, the response cache, statistics, and the
// invocation log. It is the test-console entry point; the wire calls it
// makes are real.
func (inv *Invoker) DryRun(ctx context.Context, contract *integration.Contract, system *integration.System, script string, input any) *DryRunResult {
	dryRun := new(DryRunResult)

	inputValue, err := canonicalize(input)
	if err != nil {
		dryRun.FailureKind = integration.FailureInputInvalid
		dryRun.Error = err.Error()

		return dryRun
	}

	execution := &execution{
		contract:   contract,
		system:     system,
		script:     script,
		runTimeout: inv.cfg.EffectiveRunTimeout(),
		input:      inputValue,
	}

	output, trace, kind, execErr := inv.run(ctx, execution)

	dryRun.Output = output
	dryRun.Trace = trace
	dryRun.FailureKind = kind

	if execErr != nil {
		dryRun.Error = execErr.Error()
	}

	return dryRun
}

// execution is one script run against a system under a contract.
type execution struct {
	contract   *integration.Contract
	system     *integration.System
	script     string
	runTimeout time.Duration
	input      any
}

// run validates the input, executes the script in a fresh runtime with the
// system-scoped libraries, and validates the output. It returns the failure
// classification alongside the API error.
func (inv *Invoker) run(ctx context.Context, e *execution) (any, []integration.HTTPExchange, integration.FailureKind, error) {
	if err := inv.validateSchema(e.contract.InputSchema, e.input); err != nil {
		return nil, nil, integration.FailureInputInvalid, integration.ErrInputInvalid(err.Error())
	}

	program, err := inv.programs.Get(e.script)
	if err != nil {
		return nil, nil, integration.FailureScript, integration.ErrScriptFailed(err.Error())
	}

	runtime, err := inv.newRuntime(ctx, e)
	if err != nil {
		// A system database that cannot be dialed is a transport failure;
		// everything else that blocks runtime assembly is configuration.
		if _, ok := errors.AsType[*transportError](err); ok {
			return nil, nil, integration.FailureTransport, integration.ErrTransportFailed
		}

		return nil, nil, integration.FailureConfig, err
	}

	collector := newTraceCollector(inv.capturer)
	runCtx := withTrace(ctx, collector)

	value, err := runtime.RunProgram(runCtx, program)
	if err != nil {
		kind, apiErr := classify(runCtx, err)

		return nil, collector.Exchanges(), kind, apiErr
	}

	output, err := exportOutput(value)
	if err != nil {
		return nil, collector.Exchanges(), integration.FailureScript, integration.ErrScriptFailed(err.Error())
	}

	if err := inv.validateSchema(e.contract.OutputSchema, output); err != nil {
		return nil, collector.Exchanges(), integration.FailureOutputInvalid, integration.ErrOutputInvalid(err.Error())
	}

	return output, collector.Exchanges(), "", nil
}

// newRuntime assembles a fresh runtime carrying the engine baseline plus the
// system-scoped libraries and the per-execution bindings. Each scoped library
// joins only when the system configures its transport — http for systems with
// a base URL, sql (bound to the source, read-only) for systems with a data
// source — so a script reaching for an unconfigured capability fails with a
// plain ReferenceError instead of a misleading transport fault.
func (inv *Invoker) newRuntime(ctx context.Context, e *execution) (*js.Runtime, error) {
	runtime, err := inv.engine.NewRuntime(js.WithRunTimeout(e.runTimeout))
	if err != nil {
		return nil, err
	}

	libs := []js.Lib{newErrorsLib()}

	if e.system.BaseURL != "" {
		client, err := inv.clients.ClientFor(e.system)
		if err != nil {
			return nil, err
		}

		libs = append(libs, newHTTPLib(client, CallTimeout(e.system)))
	}

	if e.system.DataSource != nil {
		systemDB, kind, err := inv.databases.DBFor(ctx, e.system)
		if err != nil {
			return nil, err
		}

		libs = append(libs, jssql.New(systemDB, kind))
	}

	for _, lib := range libs {
		if err := lib.Install(runtime); err != nil {
			return nil, fmt.Errorf("integration: install lib %s: %w", lib.Name(), err)
		}
	}

	if err := runtime.Set("input", e.input); err != nil {
		return nil, err
	}

	return runtime, runtime.Set("system", systemBinding(e.system))
}

// systemBinding is the read-only view of the system exposed to scripts:
// identity and non-sensitive params, never credentials.
func systemBinding(system *integration.System) map[string]any {
	params := system.Params
	if params == nil {
		params = map[string]string{}
	}

	return map[string]any{
		"code":   system.Code,
		"name":   system.Name,
		"params": params,
	}
}

// runTimeout resolves the script run timeout: invocation option over adapter
// override over the configured default.
func (inv *Invoker) runTimeout(cfg *integration.InvokeConfig, adapter *integration.Adapter) time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}

	if adapter.TimeoutMs > 0 {
		return time.Duration(adapter.TimeoutMs) * time.Millisecond
	}

	return inv.cfg.EffectiveRunTimeout()
}

// validateSchema validates value against the contract schema; an empty
// schema validates everything.
func (inv *Invoker) validateSchema(raw json.RawMessage, value any) error {
	if len(raw) == 0 {
		return nil
	}

	resolved, err := inv.schemas.Get(raw)
	if err != nil {
		return err
	}

	return resolved.Validate(value)
}

// classify maps a script execution error to its failure kind and API error.
// Cancellation is distinguished from timeout: a caller that walked away is
// neither an upstream fault nor an exceeded deadline, and conflating the two
// would distort the health statistics.
func classify(ctx context.Context, err error) (integration.FailureKind, error) {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return integration.FailureCanceled, integration.ErrInvocationCanceled
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return integration.FailureTimeout, integration.ErrInvocationTimeout
	}

	if upstream, ok := errors.AsType[*upstreamError](err); ok {
		return integration.FailureUpstream, integration.ErrUpstreamFailed(upstream.message)
	}

	if _, ok := errors.AsType[*transportError](err); ok {
		return integration.FailureTransport, integration.ErrTransportFailed
	}

	return integration.FailureScript, integration.ErrScriptFailed(err.Error())
}

// outcome is the recorded result of one executed invocation.
type outcome struct {
	system    string
	contract  string
	direction integration.Direction
	kind      integration.FailureKind
	err       error
	duration  time.Duration
	input     any
	output    any
	trace     []integration.HTTPExchange
}

// finish folds one invocation outcome into statistics and the invocation
// log.
func (inv *Invoker) finish(ctx context.Context, o *outcome) {
	message := ""
	if o.err != nil {
		message = o.err.Error()
	}

	inv.stats.Record(o.system, o.contract, o.direction, o.kind, message, o.duration)

	entry := &integration.InvocationLog{
		SystemCode:   o.system,
		ContractCode: o.contract,
		Direction:    o.direction,
		FailureKind:  o.kind,
		DurationMs:   o.duration.Milliseconds(),
		Input:        inv.capturer.captureValue(o.input),
		Output:       inv.capturer.captureValue(o.output),
		HTTPTrace:    o.trace,
		RequestID:    contextx.RequestID(ctx),
	}

	if message != "" {
		entry.Error = &message
	}

	inv.recorder.Record(ctx, entry)
}

// exportOutput converts the script return value into its canonical
// JSON-shaped form.
func exportOutput(value js.Value) (any, error) {
	if value == nil || js.IsUndefined(value) || js.IsNull(value) {
		return nil, nil
	}

	output, err := canonicalize(value.Export())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOutputNotSerializable, err)
	}

	return output, nil
}

// canonicalize round-trips v through JSON so every value the pipeline holds
// (schema validation, cache keys, script bindings) is in canonical
// JSON-shaped form.
func canonicalize(v any) (any, error) {
	if v == nil {
		return nil, nil
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

// loadContract fetches an enabled contract by code.
func (inv *Invoker) loadContract(ctx context.Context, code string) (*integration.Contract, error) {
	contract := new(integration.Contract)

	err := inv.db.NewSelect().
		Model(contract).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("code", code)
		}).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return nil, integration.ErrContractNotFound
		}

		return nil, err
	}

	if !contract.IsEnabled {
		return nil, integration.ErrContractDisabled
	}

	return contract, nil
}

// loadSystem fetches an enabled system by code.
func (inv *Invoker) loadSystem(ctx context.Context, code string) (*integration.System, error) {
	system := new(integration.System)

	err := inv.db.NewSelect().
		Model(system).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("code", code)
		}).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return nil, integration.ErrSystemNotFound
		}

		return nil, err
	}

	if !system.IsEnabled {
		return nil, integration.ErrSystemDisabled
	}

	return system, nil
}

// loadAdapter fetches the enabled adapter binding system to contract in the
// given flow direction.
func (inv *Invoker) loadAdapter(ctx context.Context, systemID, contractID string, direction integration.Direction) (*integration.Adapter, error) {
	adapter := new(integration.Adapter)

	err := inv.db.NewSelect().
		Model(adapter).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("system_id", systemID).
				Equals("contract_id", contractID).
				Equals("direction", direction)
		}).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, result.ErrRecordNotFound) {
			return nil, integration.ErrAdapterNotFound
		}

		return nil, err
	}

	if !adapter.IsEnabled {
		return nil, integration.ErrAdapterDisabled
	}

	return adapter, nil
}
