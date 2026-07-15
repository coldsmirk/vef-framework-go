package integration

// FailureKind classifies why an invocation failed. It is the single
// vocabulary shared by invocation logs, statistics, and API errors; an empty
// value means success.
type FailureKind string

const (
	// FailureInputInvalid marks input rejected by the contract's input schema
	// before the adapter script ran.
	FailureInputInvalid FailureKind = "input_invalid"
	// FailureOutputInvalid marks a script return value rejected by the
	// contract's output schema — the adapter mapped the response incorrectly.
	FailureOutputInvalid FailureKind = "output_invalid"
	// FailureUpstream marks a failure reported by the external system, which
	// the adapter script surfaced via errors.upstream(...).
	FailureUpstream FailureKind = "upstream"
	// FailureTransport marks a wire call that never completed: connection
	// refused, TLS failure, or an upstream that stopped responding.
	FailureTransport FailureKind = "transport"
	// FailureTimeout marks an invocation that exceeded its run timeout.
	FailureTimeout FailureKind = "timeout"
	// FailureScript marks a failure of the adapter script itself — an
	// uncaught exception or a compile error; a bug in the adapter, not in
	// the upstream.
	FailureScript FailureKind = "script"
	// FailureConfig marks an invocation the system/adapter configuration
	// prevented from executing: an auth scheme that is no longer registered
	// or a credential that cannot be decrypted.
	FailureConfig FailureKind = "config"
)
