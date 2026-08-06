package event

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// FakeNamedTransport satisfies transport.Transport's Name/Capabilities
// surface used by validateOutboxSinkRoute; the lifecycle methods stay
// no-ops because the validator never invokes them.
type FakeNamedTransport struct {
	name string
	caps transport.Capabilities
}

func (f *FakeNamedTransport) Name() string                         { return f.name }
func (f *FakeNamedTransport) Capabilities() transport.Capabilities { return f.caps }
func (*FakeNamedTransport) Start(context.Context) error            { return nil }
func (*FakeNamedTransport) Stop(context.Context) error             { return nil }
func (*FakeNamedTransport) Publish(context.Context, []transport.Frame) error {
	return nil
}

func (*FakeNamedTransport) Subscribe(string, string, transport.ConsumeFunc, transport.SubscribeConfig) (transport.Unsubscribe, error) {
	return func() {}, nil
}

func TestValidateOutboxSinkRoute(t *testing.T) {
	memory := &FakeNamedTransport{name: "memory"}
	redis := &FakeNamedTransport{name: "redis_stream", caps: transport.Capabilities{AtLeastOnce: true}}
	outboxT := &FakeNamedTransport{name: "outbox", caps: transport.Capabilities{PublishOnly: true, Transactional: true}}
	all := []transport.Transport{memory, redis, outboxT}

	t.Run("PassesWhenSinkIsTheOnlySubscribableInRoute", func(t *testing.T) {
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "memory"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "memory", all),
			"Sink=memory inside route [outbox, memory] must be accepted")
	})

	t.Run("PassesWhenSinkIsOneOfManySubscribablesInRoute", func(t *testing.T) {
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "memory", "redis_stream"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "redis_stream", all),
			"Sink=redis_stream among multiple subscribable transports must be accepted")
	})

	t.Run("PassesWhenRouteHasNoSubscribableTransport", func(t *testing.T) {
		// ["outbox"]-only routes are legal for publish-only flows (e.g.
		// storage events with no internal subscribers); there is no
		// subscribable transport to misalign with the sink. They are reported
		// through unsubscribableOutboxOrigins instead — see
		// TestUnsubscribableOutboxOrigins — because subscribing against them
		// is impossible.
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "vef.storage.*", Transports: []string{"outbox"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "memory", all),
			"Publish-only route must not require sink to appear among its members")
	})

	t.Run("FailsWhenSinkIsMissingFromMultiTransportRoute", func(t *testing.T) {
		// The reviewer's reported scenario: route includes outbox +
		// redis_stream, sink is the default memory. Subscribers attach to
		// redis_stream, relay dispatches to memory, events silently lost.
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "redis_stream"}},
			},
		}
		err := validateOutboxSinkRoute(cfg, "memory", all)
		require.Error(t, err, "Misaligned sink must fail startup")
		require.ErrorIs(t, err, ErrOutboxSinkRouteMismatch,
			"Error must wrap ErrOutboxSinkRouteMismatch so operators can match it")
		require.Contains(t, err.Error(), "approval.*",
			"Error must name the offending pattern")
		require.Contains(t, err.Error(), "memory",
			"Error must name the misaligned sink")
		require.Contains(t, err.Error(), "redis_stream",
			"Error must surface the route's subscribable transports so operators can fix the sink")
	})

	t.Run("SkipsRoutesNotReferencingOutbox", func(t *testing.T) {
		// A route that does not touch outbox is unaffected by sink config.
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "metrics.*", Transports: []string{"memory"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "redis_stream", all),
			"Routes without outbox must not be subject to the sink-route check")
	})
}

// TestUnsubscribableOutboxOrigins pins the detection of the routing shape the
// framework's own fail-fast hints used to prescribe: outbox alone. It publishes
// and relays fine, so nothing fails, yet Bus.Subscribe strips the publish-only
// outbox and has no transport left — every SubscribeInstance / BindCommand
// against it returns ErrNoRouteMatched, which a host that drops the error
// experiences as "the event never fires".
func TestUnsubscribableOutboxOrigins(t *testing.T) {
	memory := &FakeNamedTransport{name: "memory"}
	outboxT := &FakeNamedTransport{name: "outbox", caps: transport.Capabilities{PublishOnly: true, Transactional: true}}
	all := []transport.Transport{memory, outboxT}

	t.Run("ReportsOutboxOnlyRule", func(t *testing.T) {
		cfg := &config.EventConfig{
			DefaultTransport: "memory",
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox"}},
			},
		}
		require.Equal(t, []string{`routing pattern "approval.*"`}, unsubscribableOutboxOrigins(cfg, all),
			"An outbox-only rule must be reported, quoted the way it appears in application.toml")
	})

	t.Run("StaysSilentWhenTheSinkIsListedAlongside", func(t *testing.T) {
		cfg := &config.EventConfig{
			DefaultTransport: "memory",
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "memory"}},
			},
		}
		require.Empty(t, unsubscribableOutboxOrigins(cfg, all),
			"Listing the sink alongside the outbox is the fix, so it must not be reported")
	})

	t.Run("ReportsOutboxAsDefaultTransport", func(t *testing.T) {
		// The fallback route catches every event type no rule matches, so it
		// strands subscribers exactly like a rule does.
		cfg := &config.EventConfig{DefaultTransport: "outbox"}
		require.Equal(t, []string{"vef.event.default_transport"}, unsubscribableOutboxOrigins(cfg, all),
			"default_transport=outbox must be reported as its own origin")
	})

	t.Run("ReportsBothOriginsIndependently", func(t *testing.T) {
		cfg := &config.EventConfig{
			DefaultTransport: "outbox",
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "memory"}},
				{Pattern: "vef.storage.*", Transports: []string{"outbox"}},
			},
		}
		require.Equal(t,
			[]string{"vef.event.default_transport", `routing pattern "vef.storage.*"`},
			unsubscribableOutboxOrigins(cfg, all),
			"Each stranded origin must be reported on its own so operators can fix them one by one")
	})

	t.Run("StaysSilentForRoutesWithoutOutbox", func(t *testing.T) {
		cfg := &config.EventConfig{
			DefaultTransport: "memory",
			Routing: []config.EventRoutingRule{
				{Pattern: "metrics.*", Transports: []string{"memory"}},
			},
		}
		require.Empty(t, unsubscribableOutboxOrigins(cfg, all),
			"A route that never touches the outbox cannot be stranded by it")
	})

	t.Run("TreatsUnregisteredTransportsAsNoTarget", func(t *testing.T) {
		// buildRouter rejects the unknown name at Bus.Start; until then it
		// offers nothing to attach to, so the route is still a dead end.
		cfg := &config.EventConfig{
			DefaultTransport: "memory",
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "typo_stream"}},
			},
		}
		require.Equal(t, []string{`routing pattern "approval.*"`}, unsubscribableOutboxOrigins(cfg, all),
			"An unregistered companion transport must not be mistaken for a subscribable target")
	})
}
