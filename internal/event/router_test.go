package event

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// StubTransport is a minimal Transport for router unit tests; nothing
// here cares about publish or subscribe behavior.
type StubTransport struct {
	name string
	caps transport.Capabilities
}

func (s *StubTransport) Name() string                         { return s.name }
func (s *StubTransport) Capabilities() transport.Capabilities { return s.caps }
func (*StubTransport) Start(context.Context) error            { return nil }
func (*StubTransport) Stop(context.Context) error             { return nil }
func (*StubTransport) Publish(context.Context, []transport.Frame) error {
	return nil
}

func (*StubTransport) Subscribe(string, string, transport.ConsumeFunc, transport.SubscribeConfig) (transport.Unsubscribe, error) {
	return func() {}, nil
}

func TestRouterFallsBackToDefault(t *testing.T) {
	mem := &StubTransport{name: "memory"}
	registry := map[string]transport.Transport{mem.name: mem}

	r, err := buildRouter(&config.EventConfig{DefaultTransport: "memory"}, registry)
	require.NoError(t, err, "Router should build with a valid default transport")

	got := r.Resolve("unmatched.event")
	require.Len(t, got, 1, "Unmatched events should resolve to exactly one fallback transport")
	require.Equal(t, mem, got[0], "Events without a matching rule should route to the fallback")
}

func TestRouterFirstMatchWins(t *testing.T) {
	mem := &StubTransport{name: "memory"}
	out := &StubTransport{name: "outbox"}
	registry := map[string]transport.Transport{mem.name: mem, out.name: out}

	cfg := &config.EventConfig{
		DefaultTransport: "memory",
		Routing: []config.EventRoutingRule{
			{Pattern: "billing.*", Transports: []string{"outbox"}},
			{Pattern: "*", Transports: []string{"memory"}},
		},
	}

	r, err := buildRouter(cfg, registry)
	require.NoError(t, err, "Router should build with ordered routing rules")

	require.Equal(t, []transport.Transport{out}, r.Resolve("billing.charged"),
		"First matching rule should win")
	require.Equal(t, []transport.Transport{mem}, r.Resolve("auth.login"),
		"Non-billing events should fall through to the catch-all rule")
}

func TestRouterRejectsInvalidPatternAtBuildTime(t *testing.T) {
	mem := &StubTransport{name: "memory"}
	registry := map[string]transport.Transport{mem.name: mem}

	cfg := &config.EventConfig{
		DefaultTransport: "memory",
		Routing: []config.EventRoutingRule{
			{Pattern: "billing.[", Transports: []string{"memory"}}, // unterminated bracket
		},
	}

	_, err := buildRouter(cfg, registry)
	require.Error(t, err, "BuildRouter must reject patterns with malformed metacharacters")
	require.Contains(t, err.Error(), "invalid routing pattern", "Error should describe the invalid routing pattern")
}

func TestRouterUnknownTransportFails(t *testing.T) {
	registry := map[string]transport.Transport{}

	_, err := buildRouter(&config.EventConfig{DefaultTransport: "memory"}, registry)
	require.Error(t, err, "Unknown default transport must surface as a build error")
}
