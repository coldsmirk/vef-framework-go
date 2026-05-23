package event

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// fakeNamedTransport satisfies transport.Transport's Name/Capabilities
// surface used by validateOutboxSinkRoute; the lifecycle methods stay
// no-ops because the validator never invokes them.
type fakeNamedTransport struct {
	name string
	caps transport.Capabilities
}

func (f *fakeNamedTransport) Name() string                         { return f.name }
func (f *fakeNamedTransport) Capabilities() transport.Capabilities { return f.caps }
func (*fakeNamedTransport) Start(_ context.Context) error          { return nil }
func (*fakeNamedTransport) Stop(_ context.Context) error           { return nil }
func (*fakeNamedTransport) Publish(_ context.Context, _ []transport.Frame) error {
	return nil
}

func (*fakeNamedTransport) Subscribe(_, _ string, _ transport.ConsumeFunc, _ transport.SubscribeConfig) (transport.Unsubscribe, error) {
	return func() {}, nil
}

func TestValidateOutboxSinkRoute(t *testing.T) {
	memory := &fakeNamedTransport{name: "memory"}
	redis := &fakeNamedTransport{name: "redis_stream", caps: transport.Capabilities{AtLeastOnce: true}}
	outboxT := &fakeNamedTransport{name: "outbox", caps: transport.Capabilities{PublishOnly: true, Transactional: true}}
	all := []transport.Transport{memory, redis, outboxT}

	t.Run("PassesWhenSinkIsTheOnlySubscribableInRoute", func(t *testing.T) {
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "memory"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "memory", all),
			"sink=memory inside route [outbox, memory] must be accepted")
	})

	t.Run("PassesWhenSinkIsOneOfManySubscribablesInRoute", func(t *testing.T) {
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "approval.*", Transports: []string{"outbox", "memory", "redis_stream"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "redis_stream", all),
			"sink=redis_stream among multiple subscribable transports must be accepted")
	})

	t.Run("PassesWhenRouteHasNoSubscribableTransport", func(t *testing.T) {
		// ["outbox"]-only routes are legal for publish-only flows (e.g.
		// storage events with no internal subscribers); there is no
		// subscribable transport to misalign with the sink.
		cfg := &config.EventConfig{
			Routing: []config.EventRoutingRule{
				{Pattern: "vef.storage.*", Transports: []string{"outbox"}},
			},
		}
		require.NoError(t, validateOutboxSinkRoute(cfg, "memory", all),
			"publish-only-only route must not require sink to appear among its members")
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
		require.True(t, errors.Is(err, ErrOutboxSinkRouteMismatch),
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
