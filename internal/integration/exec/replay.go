package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// InvocationReplay is the itg_invocation_replay row holding the sealed replay
// payload of one invocation log entry, keyed by the entry's ID. It lives apart
// from the log so browsing the log never loads payloads.
type InvocationReplay struct {
	orm.BaseModel `bun:"table:itg_invocation_replay,alias:ir"`

	ID        string         `bun:"id,pk"`
	Payload   string         `bun:"payload"`
	CreatedAt timex.DateTime `bun:"created_at"`
}

// ReplayPayload is the lossless material a recorded invocation keeps so it can
// be re-run: the unmasked contract input of an outbound call, or the external
// request of an inbound delivery together with what the business handler
// returned, in dispatch order. Credentials never enter it.
type ReplayPayload struct {
	Input      any                         `json:"input,omitempty"`
	Request    *integration.InboundRequest `json:"request,omitempty"`
	Dispatches []DispatchResult            `json:"dispatches,omitempty"`
}

// DispatchResult is what the business handler returned for one inbound
// dispatch: its output, or the message of the error it failed with.
type DispatchResult struct {
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Replayer implements integration.Replayer over the engine's two execution
// flows.
type Replayer struct {
	invoker  *Invoker
	receiver *Receiver
}

// NewReplayer creates the replayer over the engine's two execution flows.
func NewReplayer(invoker *Invoker, receiver *Receiver) integration.Replayer {
	return &Replayer{invoker: invoker, receiver: receiver}
}

// Replay re-runs log entry logID; see integration.Replayer.
func (rp *Replayer) Replay(ctx context.Context, logID, script string) (*integration.ReplayResult, error) {
	db := rp.invoker.db

	entry, err := definition.FindOne[integration.InvocationLog](ctx, db, result.ErrRecordNotFound, byColumn("id", logID))
	if err != nil {
		return nil, err
	}

	if !entry.Replayable {
		return nil, integration.ErrReplayUnavailable
	}

	payload, err := rp.invoker.loadReplayPayload(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	contract, err := definition.FindOne[integration.Contract](ctx, db, integration.ErrContractNotFound, byColumn("code", entry.ContractCode))
	if err != nil {
		return nil, err
	}

	system, err := definition.FindOne[integration.System](ctx, db, integration.ErrSystemNotFound, byColumn("code", entry.SystemCode))
	if err != nil {
		return nil, err
	}

	adapter, err := definition.FindOne[integration.Adapter](ctx, db, integration.ErrAdapterNotFound, func(cb orm.ConditionBuilder) {
		cb.Equals("system_id", system.ID).
			Equals("contract_id", contract.ID).
			Equals("direction", entry.Direction)
	})
	if err != nil {
		return nil, err
	}

	changed := contract.UpdatedAt.After(entry.CreatedAt) ||
		system.UpdatedAt.After(entry.CreatedAt) ||
		adapter.UpdatedAt.After(entry.CreatedAt)

	if script != "" {
		adapter.Script = script
	}

	var replay *integration.ReplayResult
	if entry.Direction == integration.DirectionInbound {
		replay = rp.receiver.replay(ctx, contract, system, adapter, payload)
	} else {
		replay = rp.invoker.replay(ctx, contract, system, adapter, payload.Input)
	}

	replay.DefinitionChanged = changed

	return replay, nil
}

// byColumn matches a single column value.
func byColumn(column, value string) func(orm.ConditionBuilder) {
	return func(cb orm.ConditionBuilder) {
		cb.Equals(column, value)
	}
}

// replay re-runs an outbound invocation's input through the adapter.
func (inv *Invoker) replay(ctx context.Context, contract *integration.Contract, system *integration.System, adapter *integration.Adapter, input any) *integration.ReplayResult {
	start := time.Now()

	output, trace, kind, err := inv.run(ctx, &execution{
		contract:   contract,
		system:     system,
		script:     adapter.Script,
		runTimeout: inv.runTimeout(new(integration.InvokeConfig), adapter),
		input:      input,
	})

	return inv.replayResult(&outcome{kind: kind, err: err, duration: time.Since(start), input: input, output: output, trace: trace})
}

// replay re-runs an inbound delivery's request through the adapter script.
func (r *Receiver) replay(ctx context.Context, contract *integration.Contract, system *integration.System, adapter *integration.Adapter, payload *ReplayPayload) *integration.ReplayResult {
	d := &delivery{contract: contract, system: system, request: payload.Request}
	start := time.Now()

	reply, kind, err := r.runScript(ctx, d, &replayHandler{results: payload.Dispatches}, adapter.Script, r.runTimeout(adapter))

	return r.invoker.replayResult(&outcome{
		kind:     kind,
		err:      err,
		duration: time.Since(start),
		input:    d.dispatchedInput(),
		output:   d.dispatchedOutput(),
		trace:    r.trace(payload.Request, reply, nil),
	})
}

// replayResult shapes a replay outcome like the log entry it is compared with.
func (inv *Invoker) replayResult(o *outcome) *integration.ReplayResult {
	replay := &integration.ReplayResult{
		Input:       inv.capturer.captureValue(o.input),
		Output:      inv.capturer.captureValue(o.output),
		HTTPTrace:   o.trace,
		FailureKind: o.kind,
		DurationMs:  o.duration.Milliseconds(),
	}

	if o.err != nil {
		replay.Error = o.err.Error()
	}

	return replay
}

// sealReplay encodes and seals the outcome's replay payload for storage. It
// returns "" — the entry is recorded without one — when replay is off or the
// payload exceeds vef.integration.log.replay_limit.
func (inv *Invoker) sealReplay(o *outcome) string {
	if !inv.cfg.Log.Replay || o.replay == nil {
		return ""
	}

	data, err := json.Marshal(o.replay)
	if err != nil {
		logger.Warnf("Failed to encode the replay payload of %s/%s: %v", o.system, o.contract, err)

		return ""
	}

	if limit := inv.cfg.Log.EffectiveReplayLimit(); len(data) > limit {
		logger.Warnf("The replay payload of %s/%s (%d bytes) exceeds vef.integration.log.replay_limit (%d bytes); the entry is recorded without it",
			o.system, o.contract, len(data), limit)

		return ""
	}

	sealed, err := inv.codec.EncryptValue(string(data))
	if err != nil {
		logger.Errorf("Failed to seal the replay payload of %s/%s: %v", o.system, o.contract, err)

		return ""
	}

	return sealed
}

// loadReplayPayload opens the sealed replay payload of a log entry. A payload
// that is gone or no longer opens — sealed under a key since changed or
// removed — is reported as unavailable, the cause staying in the server log.
func (inv *Invoker) loadReplayPayload(ctx context.Context, logID string) (*ReplayPayload, error) {
	row, err := definition.FindOne[InvocationReplay](ctx, inv.db, integration.ErrReplayUnavailable, byColumn("id", logID))
	if err != nil {
		return nil, err
	}

	plain, err := inv.codec.DecryptValue(row.Payload)
	if err != nil {
		logger.Warnf("Failed to open the replay payload of invocation log %s: %v", logID, err)

		return nil, integration.ErrReplayUnavailable
	}

	payload := new(ReplayPayload)
	if err := json.Unmarshal([]byte(plain), payload); err != nil {
		logger.Warnf("Failed to decode the replay payload of invocation log %s: %v", logID, err)

		return nil, integration.ErrReplayUnavailable
	}

	return payload, nil
}

// replayRequest copies an inbound request the way replay keeps it. Credentials
// never persist: the always-masked headers are dropped and the verified
// scheme's credential values are scrubbed wherever they appear. Nothing a
// replay needs is lost — it skips verification.
func replayRequest(req *integration.InboundRequest, redact []string) *integration.InboundRequest {
	kept := *req
	kept.Headers = make(map[string]string, len(req.Headers))

	for name, value := range req.Headers {
		if !alwaysMaskedHeaders.Contains(strings.ToLower(name)) {
			kept.Headers[name] = redactSecrets(value, redact)
		}
	}

	kept.Query = redactHeaderSecrets(req.Query, redact)
	kept.Body = []byte(redactSecrets(string(req.Body), redact))

	return &kept
}

// replayHandler stands in for the business handler during an inbound replay,
// answering each dispatch with what the handler returned originally, in order,
// so the script's reply shaping — failure paths included — runs against the
// real results.
type replayHandler struct {
	results []DispatchResult
	next    int
}

// Contract is unused during a replay; the handler serves the replayed contract.
func (*replayHandler) Contract() string {
	return ""
}

// Handle returns the next recorded result.
func (h *replayHandler) Handle(context.Context, any) (any, error) {
	if h.next >= len(h.results) {
		return nil, fmt.Errorf("%w (dispatch %d)", ErrReplayDispatchUnrecorded, h.next+1)
	}

	recorded := h.results[h.next]
	h.next++

	if recorded.Error != "" {
		return nil, recordedHandlerError(recorded.Error)
	}

	return recorded.Output, nil
}

// recordedHandlerError replays the message of a business handler failure the
// original delivery recorded.
type recordedHandlerError string

func (e recordedHandlerError) Error() string {
	return string(e)
}
