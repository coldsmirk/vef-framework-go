package event_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

func TestApplySubscribeOptionsDefaultIsZero(t *testing.T) {
	cfg := event.ApplySubscribeOptions(nil)
	require.Equal(t, "", cfg.Group, "default Group is empty")
	require.Equal(t, 0, cfg.Concurrency, "default Concurrency is zero (caller decides fallback)")
}

func TestWithGroupSetsGroup(t *testing.T) {
	cfg := event.ApplySubscribeOptions([]event.SubscribeOption{event.WithGroup("payments")})
	require.Equal(t, "payments", cfg.Group)
}

func TestWithConcurrencyRejectsZeroOrNegative(t *testing.T) {
	cfg := event.ApplySubscribeOptions([]event.SubscribeOption{
		event.WithConcurrency(0),
		event.WithConcurrency(-3),
	})
	require.Equal(t, 0, cfg.Concurrency, "zero/negative concurrency must be ignored to keep the default semantics")
}

func TestWithConcurrencySetsPositive(t *testing.T) {
	cfg := event.ApplySubscribeOptions([]event.SubscribeOption{event.WithConcurrency(8)})
	require.Equal(t, 8, cfg.Concurrency)
}

func TestSubscribeOptionLastWins(t *testing.T) {
	cfg := event.ApplySubscribeOptions([]event.SubscribeOption{
		event.WithGroup("a"),
		event.WithGroup("b"),
	})
	require.Equal(t, "b", cfg.Group, "later WithGroup must override")
}
