package push

import (
	"context"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/security"
)

var logger = logx.Named("push")

var Module = fx.Module(
	"vef:push",
	fx.Provide(
		NewHub,
		fx.Annotate(
			NewRelay,
			fx.ParamTags(``, ``, ``, `optional:"true"`),
		),
		newNotifier,
		fx.Annotate(
			NewMiddleware,
			fx.ResultTags(`group:"vef:app:middlewares"`),
		),
		fx.Annotate(
			newRevocationListener,
			fx.ResultTags(`group:"vef:security:session_revocation_listeners"`),
		),
	),
	fx.Invoke(registerLifecycle),
)

// newNotifier exposes the push entry through the public contract: the relay
// when cross-node fan-out is available, the bare hub otherwise. The Notifier
// stays injectable while the endpoint is disabled — pushes then simply have no
// recipients.
func newNotifier(relay *Relay, hub *Hub) push.Notifier {
	if relay != nil {
		return relay
	}

	return hub
}

// registerLifecycle wires the hub shutdown, the relay subscription, and, under
// the opaque token mechanism, the periodic session sweep. A jwt deployment
// gets no sweep: the token is unrevocable by design and there is no session to
// check.
func registerLifecycle(
	lc fx.Lifecycle,
	hub *Hub,
	relay *Relay,
	cfg *config.PushConfig,
	securityCfg *config.SecurityConfig,
	store security.SessionStore,
) {
	if !cfg.Enabled {
		return
	}

	var sweeper *sessionSweeper
	if securityCfg.EffectiveTokenType() == config.TokenTypeOpaque {
		sweeper = newSessionSweeper(hub, store, cfg.EffectiveSessionRecheckInterval())
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			if relay != nil {
				relay.start()
			}

			if sweeper != nil {
				sweeper.start()
			}

			return nil
		},
		OnStop: func(context.Context) error {
			if sweeper != nil {
				sweeper.shutdown()
			}

			if relay != nil {
				relay.stop()
			}

			hub.Shutdown()

			return nil
		},
	})
}
