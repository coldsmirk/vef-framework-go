package integration

import "time"

// StatsInspector exposes per-node invocation statistics. The monitor module
// consumes it as an optional dependency (absent when the integration module
// is not enabled), mirroring event.StreamInspector.
type StatsInspector interface {
	// Stats returns a snapshot of invocation statistics, one entry per
	// (system, contract, direction) tuple observed since process start,
	// ordered by system, contract, then direction.
	Stats() []InvocationStats
}

// InvocationStats aggregates the invocations of one (system, contract,
// direction) tuple on this node since process start. Inbound deliveries
// rejected by verification aggregate under an empty Contract — the contract
// code is unvalidated caller input at rejection time.
type InvocationStats struct {
	System        string                `json:"system"`
	Contract      string                `json:"contract"`
	Direction     Direction             `json:"direction"`
	Calls         int64                 `json:"calls"`
	Successes     int64                 `json:"successes"`
	Failures      map[FailureKind]int64 `json:"failures,omitempty"`
	AvgDurationMs int64                 `json:"avgDurationMs"`
	MaxDurationMs int64                 `json:"maxDurationMs"`
	LastError     string                `json:"lastError,omitempty"`
	LastErrorAt   time.Time             `json:"lastErrorAt,omitzero"`
}
