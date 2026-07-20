package cron

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunJSONUsesExactTimeFieldsOnly(t *testing.T) {
	instant := int64(1_794_727_800_000)
	run := &Run{
		ScheduledAtUnixMs: instant,
		ClaimedAtUnixMs:   instant,
		StartedAtUnixMs:   &instant,
		FinishedAtUnixMs:  &instant,
		HeartbeatAtUnixMs: &instant,
	}

	encoded, err := json.Marshal(run)
	require.NoError(t, err, "The run should marshal to JSON")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload), "The run JSON should decode for inspection")

	for _, field := range []string{
		"scheduledAtUnixMs",
		"claimedAtUnixMs",
		"startedAtUnixMs",
		"finishedAtUnixMs",
		"heartbeatAtUnixMs",
	} {
		assert.Equal(t, float64(instant), payload[field], "The run should expose %s exactly", field)
	}

	for _, field := range []string{"scheduledAt", "startedAt", "finishedAt", "heartbeatAt"} {
		assert.NotContains(t, payload, field, "The run should not expose ambiguous field %s", field)
	}
}
