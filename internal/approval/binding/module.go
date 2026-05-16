package binding

import (
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// Module provides the default BusinessBindingHook and starts the listener
// that runs OnInstanceCompleted asynchronously when InstanceCompletedEvent
// fires. Hosts override the hook with fx.Replace or
// vef.SupplyBusinessBindingHook.
var Module = fx.Module(
	"vef:approval:binding",

	fx.Provide(
		fx.Annotate(NewDefaultHook, fx.As(new(approval.BusinessBindingHook))),
		NewListener,
	),

	fx.Invoke(func(l *Listener) error { return l.Start() }),
)
