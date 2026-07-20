package cron

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunEventsCarryExactScheduledTime(t *testing.T) {
	run := &Run{ScheduledAtUnixMs: 1_794_727_800_000}

	failed := NewRunFailedEvent(run)
	abandoned := NewRunAbandonedEvent(run)
	assert.Equal(t, run.ScheduledAtUnixMs, failed.ScheduledAtUnixMs,
		"Run-failed events should preserve the scheduled epoch")
	assert.Equal(t, run.ScheduledAtUnixMs, abandoned.ScheduledAtUnixMs,
		"Run-abandoned events should preserve the scheduled epoch")

	for _, event := range []any{failed, abandoned} {
		encoded, err := json.Marshal(event)
		require.NoError(t, err, "The run event should marshal to JSON")

		var payload map[string]any
		require.NoError(t, json.Unmarshal(encoded, &payload), "The run event JSON should decode for inspection")
		assert.Equal(t, float64(run.ScheduledAtUnixMs), payload["scheduledAtUnixMs"],
			"The run event should expose the exact scheduled instant")
		assert.NotContains(t, payload, "scheduledAt", "The run event should not expose an ambiguous wall time")
	}
}
