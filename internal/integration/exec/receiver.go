package exec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/js"
)

// Registration faults raised while building the inbound handler index; they
// surface as fx start-up errors.
var (
	// ErrBlankHandlerContract rejects an inbound handler with no contract code.
	ErrBlankHandlerContract = errors.New("integration: inbound handler declares no contract code")
	// ErrDuplicateHandlerContract rejects two inbound handlers claiming the
	// same contract.
	ErrDuplicateHandlerContract = errors.New("integration: duplicate inbound handler for contract")
)

// Receiver is the inbound half of the execution engine: the protocol-blind
// pipeline every inbound gateway hands its requests to. It verifies the
// caller against the system's inbound auth, runs the inbound adapter script —
// which translates the wire request, dispatches the standard input to the
// registered business handler, and shapes the vendor-facing reply — and folds
// the delivery into the shared statistics and invocation log. It deliberately
// shares the Invoker's execution substrate (definition loading, compiled
// programs, schema cache, capture policy) instead of duplicating it.
type Receiver struct {
	invoker  *Invoker
	codec    *service.SecretCodec
	schemes  *auth.InboundRegistry
	handlers map[string]integration.InboundHandler
}

// NewReceiver assembles the receiver, indexing the registered inbound
// handlers by contract code and rejecting blank or duplicate registrations at
// boot.
func NewReceiver(invoker *Invoker, codec *service.SecretCodec, schemes *auth.InboundRegistry, handlers []integration.InboundHandler) (*Receiver, error) {
	index := make(map[string]integration.InboundHandler, len(handlers))

	for _, handler := range handlers {
		if handler == nil {
			continue
		}

		contract := handler.Contract()
		if contract == "" {
			return nil, ErrBlankHandlerContract
		}

		if _, ok := index[contract]; ok {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateHandlerContract, contract)
		}

		index[contract] = handler
	}

	return &Receiver{invoker: invoker, codec: codec, schemes: schemes, handlers: index}, nil
}

// Receive processes one vendor-initiated request end to end and returns the
// adapter script's reply for the gateway to render. Verification failures
// return integration.ErrInboundAuthFailed uniformly — the specific reason is
// recorded server-side only.
func (r *Receiver) Receive(ctx context.Context, req *integration.InboundRequest) (any, error) {
	inv := r.invoker

	system, err := inv.loadSystem(ctx, req.SystemCode)
	if err != nil {
		return nil, err
	}

	if err := r.verify(ctx, system, req); err != nil {
		r.recordRejection(req, err)

		return nil, integration.ErrInboundAuthFailed
	}

	contract, err := inv.loadContract(ctx, req.ContractCode)
	if err != nil {
		return nil, err
	}

	adapter, err := inv.loadAdapter(ctx, system.ID, contract.ID, integration.DirectionInbound)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	delivery := &delivery{
		contract: contract,
		system:   system,
		request:  req,
	}

	reply, kind, deliverErr := r.run(ctx, delivery, adapter)
	duration := time.Since(start)

	inv.finish(ctx, &outcome{
		system:    system.Code,
		contract:  contract.Code,
		direction: integration.DirectionInbound,
		kind:      kind,
		err:       deliverErr,
		duration:  duration,
		input:     delivery.dispatchedInput(),
		output:    delivery.dispatchedOutput(),
		trace:     r.trace(req, reply),
	})

	if deliverErr != nil {
		return nil, deliverErr
	}

	return reply, nil
}

// verify authenticates the request against the system's inbound auth
// configuration, fail closed: a system without one refuses inbound delivery.
func (r *Receiver) verify(ctx context.Context, system *integration.System, req *integration.InboundRequest) error {
	scheme, ok := r.schemes.Resolve(system.InboundAuth)
	if !ok {
		return fmt.Errorf("%w: inbound auth scheme", auth.ErrMissingParam)
	}

	decrypted, err := r.codec.DecryptInboundAuth(scheme, system.InboundAuth)
	if err != nil {
		return fmt.Errorf("%w: inbound auth params: %w", auth.ErrMissingParam, err)
	}

	return scheme.Verify(ctx, req, decrypted)
}

// recordRejection folds a verification failure into statistics only. Rejected
// deliveries deliberately stay out of the invocation log: unauthenticated
// traffic must not be able to grow the durable evidence trail.
func (r *Receiver) recordRejection(req *integration.InboundRequest, err error) {
	kind := integration.FailureAuth
	if errors.Is(err, auth.ErrMissingParam) {
		kind = integration.FailureConfig
	}

	logger.Warnf("Inbound delivery to system %q rejected (%s): %v", req.SystemCode, kind, err)

	r.invoker.stats.Record(req.SystemCode, req.ContractCode, integration.DirectionInbound, kind, err.Error(), 0)
}

// delivery is the per-request state of one inbound run: the resolved
// definitions plus what the script dispatched, for classification and the
// invocation log. Batch payloads may dispatch more than once; every dispatch
// is recorded, and the last dispatch failure stays sticky even when the
// script catches it to shape a partial-success reply.
type delivery struct {
	contract *integration.Contract
	system   *integration.System
	request  *integration.InboundRequest

	inputs  []any
	outputs []any

	dispatchKind integration.FailureKind
	dispatchErr  error
}

// dispatchedInput flattens the recorded inputs for the invocation log.
func (d *delivery) dispatchedInput() any {
	return flattenDispatches(d.inputs)
}

// dispatchedOutput flattens the recorded outputs for the invocation log.
func (d *delivery) dispatchedOutput() any {
	return flattenDispatches(d.outputs)
}

// flattenDispatches keeps the single-dispatch case readable (one value, not a
// one-element array) while preserving every dispatch of a batch run.
func flattenDispatches(values []any) any {
	switch len(values) {
	case 0:
		return nil
	case 1:
		return values[0]
	default:
		return values
	}
}

// run executes the inbound adapter script and returns the vendor-facing
// reply. A script that completes owns the reply even when a dispatch inside
// it failed — the failure is still classified for the record; a script that
// throws yields no reply, and an uncaught dispatch error keeps its own
// classification instead of counting as a script bug.
func (r *Receiver) run(ctx context.Context, d *delivery, adapter *integration.Adapter) (any, integration.FailureKind, error) {
	handler, ok := r.handlers[d.contract.Code]
	if !ok {
		return nil, integration.FailureConfig, integration.ErrInboundHandlerMissing
	}

	inv := r.invoker

	program, err := inv.programs.Get(adapter.Script)
	if err != nil {
		return nil, integration.FailureScript, integration.ErrScriptFailed(err.Error())
	}

	runtime, err := inv.engine.NewRuntime(js.WithRunTimeout(r.runTimeout(adapter)))
	if err != nil {
		return nil, integration.FailureScript, err
	}

	if err := r.bind(ctx, runtime, d, handler); err != nil {
		return nil, integration.FailureScript, err
	}

	value, err := runtime.RunProgram(ctx, program)
	if err != nil {
		if d.dispatchErr != nil && errors.Is(err, d.dispatchErr) {
			return nil, d.dispatchKind, d.dispatchErr
		}

		if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			return nil, integration.FailureTimeout, integration.ErrInvocationTimeout
		}

		return nil, integration.FailureScript, integration.ErrScriptFailed(err.Error())
	}

	reply, err := exportOutput(value)
	if err != nil {
		return nil, integration.FailureScript, integration.ErrScriptFailed(err.Error())
	}

	return reply, d.dispatchKind, nil
}

// bind installs the per-delivery bindings: the wire request, the system view,
// and the dispatch function bridging into the business handler.
func (r *Receiver) bind(ctx context.Context, runtime *js.Runtime, d *delivery, handler integration.InboundHandler) error {
	if err := runtime.Set("request", auth.InboundRequestBinding(d.request)); err != nil {
		return err
	}

	if err := runtime.Set("system", systemBinding(d.system)); err != nil {
		return err
	}

	return runtime.Set("dispatch", func(input any) (any, error) {
		return r.dispatch(ctx, d, handler, input)
	})
}

// dispatch validates the standard input, runs the business handler, and
// validates the standard output — the contract enforcement point of the
// inbound flow. Failures are recorded on the delivery before they surface
// into the script as catchable errors.
func (r *Receiver) dispatch(ctx context.Context, d *delivery, handler integration.InboundHandler, input any) (any, error) {
	inv := r.invoker

	input, err := canonicalize(input)
	if err != nil {
		return nil, d.fail(integration.FailureInputInvalid, integration.ErrInputInvalid(err.Error()))
	}

	if err := inv.validateSchema(d.contract.InputSchema, input); err != nil {
		return nil, d.fail(integration.FailureInputInvalid, integration.ErrInputInvalid(err.Error()))
	}

	d.inputs = append(d.inputs, input)

	output, err := handler.Handle(ctx, input)
	if err != nil {
		return nil, d.fail(integration.FailureHandler, err)
	}

	if output, err = canonicalize(output); err != nil {
		return nil, d.fail(integration.FailureOutputInvalid, integration.ErrOutputInvalid(err.Error()))
	}

	if err := inv.validateSchema(d.contract.OutputSchema, output); err != nil {
		return nil, d.fail(integration.FailureOutputInvalid, integration.ErrOutputInvalid(err.Error()))
	}

	d.outputs = append(d.outputs, output)

	return output, nil
}

// fail records a dispatch failure on the delivery and returns the error for
// the script to observe.
func (d *delivery) fail(kind integration.FailureKind, err error) error {
	d.dispatchKind = kind
	d.dispatchErr = err

	return err
}

// runTimeout resolves the inbound script run timeout: adapter override over
// the configured default (inbound deliveries have no per-call option).
func (r *Receiver) runTimeout(adapter *integration.Adapter) time.Duration {
	if adapter.TimeoutMs > 0 {
		return time.Duration(adapter.TimeoutMs) * time.Millisecond
	}

	return r.invoker.cfg.EffectiveRunTimeout()
}

// trace renders the vendor-side view of the delivery as one wire exchange,
// masked and truncated by the shared capture policy. Status stays zero — the
// pipeline is protocol-blind and never interprets the reply.
func (r *Receiver) trace(req *integration.InboundRequest, reply any) []integration.HTTPExchange {
	capturer := r.invoker.capturer

	exchange := integration.HTTPExchange{
		Method:         req.Method,
		URL:            capturer.maskURL(req.Path),
		RequestHeaders: capturer.maskHeaderMap(req.Headers),
		RequestBody:    capturer.captureBody(string(req.Body)),
	}

	if reply != nil {
		exchange.ResponseBody = capturer.captureBody(string(capturer.captureValue(reply)))
	}

	return []integration.HTTPExchange{exchange}
}
