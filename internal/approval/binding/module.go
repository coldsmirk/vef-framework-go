package binding

import (
	"go.uber.org/fx"
)

// Module provides the engine-owned write-back Writer (with its default
// no-op BusinessRefProvider and identity BusinessRefResolver) and starts
// the Listener that runs the asynchronous write-back legs when the
// completed / returned / withdrawn / resubmitted instance events fire.
// (The started leg is invoked synchronously by the start_instance
// command inside its transaction.) Hosts replace the provider / resolver
// via vef.SupplyBusinessRefProvider / vef.SupplyBusinessRefResolver; the
// Writer itself is not an extension point.
//
// Reliability note: the Listener is best-effort under the default
// in-memory event transport — a process crash between event publication
// and listener execution drops the write-back. Production deployments
// that rely on business-table consistency MUST route the approval.*
// instance events through the framework's outbox transport so the
// listener retries until acknowledged. The Listener publishes
// InstanceBindingFailedEvent on persistent failure so operators / saga
// workers can compensate; misconfigured flows (ErrBindingMisconfigured)
// ack immediately to avoid an infinite retry loop.
var Module = fx.Module(
	"vef:approval:binding",

	fx.Provide(
		NewNoopRefProvider,
		NewIdentityResolver,
		NewConfigValidator,
		NewWriter,
		NewListener,
	),

	fx.Invoke(func(l *Listener) error { return l.Start() }),
)
