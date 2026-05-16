package binding

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// Module provides the default BusinessBindingHook and starts the Listener
// that calls WriteBackStatus asynchronously when InstanceCompletedEvent
// fires. Hosts override the hook with fx.Replace or
// vef.SupplyBusinessBindingHook.
//
// Reliability note: the Listener is best-effort under the default
// in-memory event transport — a process crash between event publication
// and listener execution drops the write-back. Production deployments
// that rely on business-table consistency MUST route
// approval.instance.completed through the framework's outbox transport
// so the listener retries until acknowledged. The Listener publishes
// InstanceBindingFailedEvent on persistent failure so operators / saga
// workers can compensate; misconfigured flows (ErrBindingMisconfigured)
// ack immediately to avoid an infinite retry loop.
var Module = fx.Module(
	"vef:approval:binding",

	fx.Provide(
		fx.Annotate(NewDefaultHook, fx.As(new(approval.BusinessBindingHook))),
		NewListener,
	),

	fx.Invoke(func(l *Listener) error { return l.Start() }),
)
