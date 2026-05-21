package event_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

func TestApplyPublishOptionsDefaultsAreZero(t *testing.T) {
	cfg := event.ApplyPublishOptions(nil)
	require.Zero(t, cfg, "empty option slice should resolve to a zero PublishConfig")
}

func TestApplyPublishOptionsAppliesAllFields(t *testing.T) {
	ts := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	cfg := event.ApplyPublishOptions([]event.PublishOption{
		event.WithSource("auth-service"),
		event.WithOccurredAt(ts),
		event.WithCorrelationID("corr-1"),
		event.WithHeaders(map[string]string{"k1": "v1"}),
	})

	require.Equal(t, "auth-service", cfg.Source)
	require.Equal(t, ts, cfg.OccurredAt)
	require.Equal(t, "corr-1", cfg.CorrelationID)
	require.Equal(t, "v1", cfg.Headers["k1"])
}

func TestWithAsyncSetsAsyncFlag(t *testing.T) {
	cfg := event.ApplyPublishOptions([]event.PublishOption{event.WithAsync()})
	require.True(t, cfg.Async, "WithAsync should toggle the Async flag")
	require.Nil(t, cfg.Tx, "WithAsync should not touch Tx")
}

func TestWithHeadersMergesMultipleCalls(t *testing.T) {
	cfg := event.ApplyPublishOptions([]event.PublishOption{
		event.WithHeaders(map[string]string{"a": "1", "b": "2"}),
		event.WithHeaders(map[string]string{"b": "overwritten", "c": "3"}),
	})

	require.Equal(t, "1", cfg.Headers["a"], "first WithHeaders value should survive")
	require.Equal(t, "overwritten", cfg.Headers["b"], "later WithHeaders should override earlier keys")
	require.Equal(t, "3", cfg.Headers["c"], "merge should accumulate new keys across calls")
	require.Len(t, cfg.Headers, 3, "merge should not duplicate or drop keys")
}

func TestWithHeadersNilInputIsNoop(t *testing.T) {
	cfg := event.ApplyPublishOptions([]event.PublishOption{event.WithHeaders(nil)})
	// A nil map argument should not allocate Headers; merging nil is a no-op.
	require.Empty(t, cfg.Headers, "nil headers input should leave Headers empty")
}

func TestOptionOrderLastWins(t *testing.T) {
	cfg := event.ApplyPublishOptions([]event.PublishOption{
		event.WithSource("first"),
		event.WithSource("second"),
		event.WithSource("third"),
	})
	require.Equal(t, "third", cfg.Source, "later options must override earlier ones for the same field")
}
