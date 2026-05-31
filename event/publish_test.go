package event_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/event"
)

func TestApplyPublishOptionsDefaultsAreZero(t *testing.T) {
	cfg := event.ApplyPublishOptions(nil)
	require.Zero(t, cfg, "Empty option slice should resolve to a zero PublishConfig")
}

func TestApplyPublishOptionsAppliesAllFields(t *testing.T) {
	ts := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	cfg := event.ApplyPublishOptions([]event.PublishOption{
		event.WithSource("auth-service"),
		event.WithOccurredAt(ts),
		event.WithCorrelationID("corr-1"),
		event.WithHeaders(map[string]string{"k1": "v1"}),
	})

	require.Equal(t, "auth-service", cfg.Source, "WithSource should populate Source")
	require.Equal(t, ts, cfg.OccurredAt, "WithOccurredAt should populate OccurredAt")
	require.Equal(t, "corr-1", cfg.CorrelationID, "WithCorrelationID should populate CorrelationID")
	require.Equal(t, "v1", cfg.Headers["k1"], "WithHeaders should merge header values")
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

	require.Equal(t, "1", cfg.Headers["a"], "First WithHeaders value should survive")
	require.Equal(t, "overwritten", cfg.Headers["b"], "Later WithHeaders should override earlier keys")
	require.Equal(t, "3", cfg.Headers["c"], "Merge should accumulate new keys across calls")
	require.Len(t, cfg.Headers, 3, "Merge should not duplicate or drop keys")
}

func TestWithHeadersNilInputIsNoop(t *testing.T) {
	cfg := event.ApplyPublishOptions([]event.PublishOption{event.WithHeaders(nil)})
	// A nil map argument should not allocate Headers; merging nil is a no-op.
	require.Empty(t, cfg.Headers, "Nil headers input should leave Headers empty")
}

func TestOptionOrderLastWins(t *testing.T) {
	cfg := event.ApplyPublishOptions([]event.PublishOption{
		event.WithSource("first"),
		event.WithSource("second"),
		event.WithSource("third"),
	})
	require.Equal(t, "third", cfg.Source, "Later options must override earlier ones for the same field")
}
