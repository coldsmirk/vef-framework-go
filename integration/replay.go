package integration

import (
	"context"
	"encoding/json"
)

// Replayer re-runs invocations recorded in the invocation log. It is the
// engine behind integration/log.replay, injectable so an application serving
// its own management surface can offer replay under its own resource.
//
// A replay takes the dry-run path — nothing is cached, counted, or logged, and
// disabled definitions are accepted — but the wire calls an outbound replay
// makes are real. An inbound replay skips verification and answers each
// dispatch with what the business handler returned originally, so no business
// code runs twice. The Replayer does not authorize: a host exposing it owns
// that check, as the framework's resource does with integration.log.replay.
type Replayer interface {
	// Replay re-runs log entry logID against the current definitions. A
	// non-empty script runs in place of the saved adapter's, the way a dry
	// run tests unsaved edits. An entry recorded without a replay payload
	// (vef.integration.log.replay off, or the payload over replay_limit)
	// fails with ErrReplayUnavailable, an unknown ID with
	// result.ErrRecordNotFound.
	Replay(ctx context.Context, logID, script string) (*ReplayResult, error)
}

// ReplayResult is the outcome of a replay, shaped like the invocation log entry
// it is compared with. Its captures pass through the same masking and
// truncation, so the two read side by side and a replay never discloses what
// the log masked.
type ReplayResult struct {
	Input       json.RawMessage `json:"input"`
	Output      json.RawMessage `json:"output"`
	HTTPTrace   []HTTPExchange  `json:"httpTrace"`
	FailureKind FailureKind     `json:"failureKind,omitempty"`
	Error       string          `json:"error,omitempty"`
	DurationMs  int64           `json:"durationMs"`
	// DefinitionChanged reports that the contract, system, or adapter was
	// modified after the original invocation, so a different outcome may stem
	// from that edit rather than from the external system.
	DefinitionChanged bool `json:"definitionChanged"`
}
