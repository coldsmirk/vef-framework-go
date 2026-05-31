package datasource

import "time"

// RegisterOption tunes a single Update or Unregister call — the two operations
// that close an existing connection. Options compose; later options override
// earlier ones for the same field.
type RegisterOption func(*RegisterOptions)

// RegisterOptions holds the tunables an Update/Unregister call honors. It is
// exported so registry implementations can read it; user code should construct
// it via the RegisterOption helpers.
type RegisterOptions struct {
	// CloseGrace controls how long the registry waits before closing a replaced
	// or unregistered connection. Zero (the default) closes immediately on a
	// background goroutine.
	CloseGrace time.Duration
}

// WithCloseGrace returns a RegisterOption that delays the asynchronous close of
// a replaced (Update) or removed (Unregister) connection by d. Use it to give
// in-flight queries some time to drain before the connection pool tears down.
func WithCloseGrace(d time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		if d > 0 {
			o.CloseGrace = d
		}
	}
}

// ReconcileOption tunes a single Reconcile invocation.
type ReconcileOption func(*ReconcileOptions)

// ReconcileOptions holds the tunables a Reconcile call honors.
type ReconcileOptions struct {
	// DryRun makes Reconcile compute the diff and return it in the report without
	// performing Register/Update/Unregister. Useful for previewing what a
	// refresher job would do.
	DryRun bool
}

// WithReconcileDryRun returns a ReconcileOption that flips Reconcile into
// preview mode: the report still lists Added/Updated/Removed but no connections
// are opened or closed.
func WithReconcileDryRun() ReconcileOption {
	return func(o *ReconcileOptions) {
		o.DryRun = true
	}
}

// ReconcileReport summarizes the result of a Reconcile call. Errors is keyed by
// data source name and is nil when every action succeeded.
type ReconcileReport struct {
	Added   []string
	Updated []string
	Removed []string
	Errors  map[string]error
}
