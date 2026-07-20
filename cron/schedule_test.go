package cron

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleTriggerUsesExactFireTime(t *testing.T) {
	epochTime := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC)
	epochMs := epochTime.UnixMilli()
	schedule := &Schedule{
		Kind:         TriggerOnce,
		FireAtUnixMs: &epochMs,
	}

	trigger := schedule.Trigger()
	require.NotNil(t, trigger.At, "A one-shot trigger should expose its fire time")
	assert.True(t, trigger.At.Equal(epochTime), "The trigger should preserve the exact fire instant")
	assert.Same(t, time.UTC, trigger.At.Location(), "The reconstructed fire time should use the canonical UTC location")
}

func TestScheduleJSONUsesExactTimeFieldsOnly(t *testing.T) {
	instant := int64(1_794_727_800_000)
	schedule := &Schedule{
		FireAtUnixMs:     &instant,
		StartsAtUnixMs:   &instant,
		EndsAtUnixMs:     &instant,
		AnchorAtUnixMs:   instant,
		NextFireAtUnixMs: &instant,
		LastFireAtUnixMs: &instant,
	}

	encoded, err := json.Marshal(schedule)
	require.NoError(t, err, "The schedule should marshal to JSON")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload), "The schedule JSON should decode for inspection")

	for _, field := range []string{
		"fireAtUnixMs",
		"startsAtUnixMs",
		"endsAtUnixMs",
		"anchorAtUnixMs",
		"nextFireAtUnixMs",
		"lastFireAtUnixMs",
	} {
		assert.Equal(t, float64(instant), payload[field], "The schedule should expose %s exactly", field)
	}

	for _, field := range []string{"fireAt", "startsAt", "endsAt", "nextFireAt", "lastFireAt"} {
		assert.NotContains(t, payload, field, "The schedule should not expose ambiguous field %s", field)
	}
}

func TestScheduleTimeoutUsesMilliseconds(t *testing.T) {
	schedule := &Schedule{TimeoutMs: 1500}

	assert.Equal(t, 1500*time.Millisecond, schedule.Timeout(), "TimeoutMs must convert to a duration")
}
