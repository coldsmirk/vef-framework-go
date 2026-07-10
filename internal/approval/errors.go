package approval

import "errors"

// ErrEventRouteNotTransactional indicates the framework's event bus is
// not configured to deliver an approval domain event through a
// transactional transport. Approval publishes every business-side event
// with event.WithTx (via EventPublishBehavior and engine.PublishEventsTx)
// so subscribers see the event iff the originating business transaction
// commits; without a transactional route the first publish would fail at
// runtime with event.ErrTxRequired and roll the business transaction
// back. The wrapped formatted error names the offending event type and
// points operators at the configuration that must be set.
var ErrEventRouteNotTransactional = errors.New("approval: event must route to a transactional transport")
