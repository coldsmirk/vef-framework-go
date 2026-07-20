package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/cron"
)

func TestScheduleWireUsesExactUnixMilliseconds(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	require.NoError(t, err, "The fallback timezone should load")

	first := time.Date(2012, time.November, 4, 1, 30, 0, 0, newYork)
	second := first.Add(time.Hour)
	require.Equal(t, first.Format(time.DateTime), second.In(newYork).Format(time.DateTime),
		"The fixtures must share one fallback wall-clock label")

	secondUnixMs := second.UnixMilli()
	endUnixMs := second.Add(time.Hour).UnixMilli()
	params := ScheduleParams{
		Trigger: TriggerParams{
			Kind:     cron.TriggerOnce,
			AtUnixMs: &secondUnixMs,
		},
		StartsAtUnixMs: &secondUnixMs,
		EndsAtUnixMs:   &endUnixMs,
	}

	payload, err := json.Marshal(params)
	require.NoError(t, err, "The exact schedule params should encode")
	assert.NotContains(t, string(payload), `"at":`, "The wire contract must not expose a wall-clock trigger field")
	assert.NotContains(t, string(payload), `"startsAt":`, "The wire contract must not expose a wall-clock window field")

	var decoded ScheduleParams
	require.NoError(t, json.Unmarshal(payload, &decoded), "The exact schedule params should decode")

	spec, err := decoded.spec()
	require.NoError(t, err, "The exact schedule params should convert")
	require.NotNil(t, spec.Trigger.At, "The one-shot input should be present")
	assert.True(t, spec.Trigger.At.Equal(second), "The one-shot must retain the second fallback instant")
	require.NotNil(t, spec.StartsAt, "The window start should be present")
	assert.True(t, spec.StartsAt.Equal(second), "The window start must retain the second fallback instant")
	require.NotNil(t, spec.EndsAt, "The window end should be present")
	assert.True(t, spec.EndsAt.Equal(second.Add(time.Hour)), "The window end must retain its exact instant")
}

func TestScheduleParamsRejectUnrepresentableTimeout(t *testing.T) {
	tests := []struct {
		name      string
		timeoutMs int64
	}{
		{name: "Negative", timeoutMs: -1},
		{name: "BeyondDurationRange", timeoutMs: cron.MaxDurationMilliseconds + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (ScheduleParams{TimeoutMs: tt.timeoutMs}).spec()
			require.ErrorIs(t, err, cron.ErrScheduleInvalid(""),
				"An unrepresentable wire timeout must be rejected before duration conversion")
		})
	}
}

func TestScheduleParamsDecodeIsStrictAndExact(t *testing.T) {
	t.Run("PreservesLargeParamsInteger", func(t *testing.T) {
		var raw api.Params
		require.NoError(t, json.Unmarshal([]byte(`{
  "name":"exact-params",
  "jobName":"orders.sync",
  "trigger":{"kind":"interval","everyMs":1000},
  "params":{"businessId":9007199254740993}
}`), &raw), "The exact schedule payload should decode")

		var params ScheduleParams
		require.NoError(t, raw.DecodeStrict(&params), "The strict schedule params should decode")
		spec, err := params.spec()
		require.NoError(t, err, "The exact schedule params should convert")

		persisted, ok := spec.Params.(json.RawMessage)
		require.True(t, ok, "Schedule params should remain a raw JSON payload")
		assert.Contains(t, string(persisted), "9007199254740993",
			"A business integer must retain every digit before persistence")
	})

	unknownCases := []struct {
		name    string
		payload string
	}{
		{
			name: "RetiredWindowField",
			payload: `{"name":"legacy","jobName":"orders.sync",` +
				`"trigger":{"kind":"interval","everyMs":1000},"startsAt":"2026-07-20 10:00:00"}`,
		},
		{
			name: "RetiredTriggerField",
			payload: `{"name":"legacy","jobName":"orders.sync",` +
				`"trigger":{"kind":"once","at":1784532000000,"atUnixMs":1784532000000}}`,
		},
	}

	for _, tt := range unknownCases {
		t.Run(tt.name, func(t *testing.T) {
			var raw api.Params
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &raw), "The legacy payload should parse")

			var params ScheduleParams
			assert.Error(t, raw.DecodeStrict(&params),
				"A strict schedule mutation must reject every unknown or retired field")
		})
	}

	kindConflictCases := []struct {
		name    string
		payload string
	}{
		{
			name: "CronWithZeroInterval",
			payload: `{"name":"conflict","jobName":"orders.sync",` +
				`"trigger":{"kind":"cron","expr":"0 * * * *","everyMs":0}}`,
		},
		{
			name: "IntervalWithEmptyExpression",
			payload: `{"name":"conflict","jobName":"orders.sync",` +
				`"trigger":{"kind":"interval","everyMs":1000,"expr":""}}`,
		},
	}

	for _, tt := range kindConflictCases {
		t.Run(tt.name, func(t *testing.T) {
			var raw api.Params
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &raw), "The conflicting payload should parse")

			var params ScheduleParams
			require.NoError(t, raw.DecodeStrict(&params), "The typed fields should decode before union validation")
			_, err := params.spec()
			require.ErrorIs(t, err, cron.ErrTriggerInvalid(""),
				"A kind-specific zero-value field must still violate the tagged union")
		})
	}
}

func TestPreviewTriggerFires(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 30, 0, 0, time.UTC)

	trigger := func(spec cron.TriggerSpec) TriggerParams {
		params := TriggerParams{Kind: spec.Kind}

		switch spec.Kind {
		case cron.TriggerCron:
			params.Expr = &spec.Expr
			params.Timezone = &spec.Timezone
		case cron.TriggerInterval:
			params.EveryMs = &spec.EveryMs
		}

		if spec.At != nil {
			params.AtUnixMs = unixMillisPtr(*spec.At)
		}

		return params
	}

	t.Run("CronExpression", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Expr("0 3 * * *", "")),
		}, now)
		require.NoError(t, err, "A valid cron expression should preview")
		require.Len(t, preview.NextFiresUnixMs, nextFiresPreview, "The preview should project the full window")

		for i, fire := range preview.NextFiresUnixMs {
			expected := time.Date(2026, 7, 20+i, 3, 0, 0, 0, time.UTC).UnixMilli()
			assert.Equal(t, expected, fire, "Fire %d should land at 03:00 on consecutive days", i)
		}
	})

	t.Run("IntervalAnchoredAtNow", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Every(time.Hour)),
		}, now)
		require.NoError(t, err, "A valid interval should preview")
		require.Len(t, preview.NextFiresUnixMs, nextFiresPreview, "The preview should project the full window")

		for i, fire := range preview.NextFiresUnixMs {
			assert.Equal(t, now.Add(time.Duration(i+1)*time.Hour).UnixMilli(), fire,
				"Fire %d should step one hour from the creation anchor", i)
		}
	})

	t.Run("OnceYieldsSingleFire", func(t *testing.T) {
		at := now.Add(time.Hour)
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Once(at)),
		}, now)
		require.NoError(t, err, "A future one-shot should preview")
		assert.Equal(t, []int64{at.UnixMilli()}, preview.NextFiresUnixMs,
			"A one-shot should expose its one exact fire")
	})

	t.Run("SpentTriggerPreviewsEmpty", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Once(now.Add(-time.Hour))),
		}, now)
		require.NoError(t, err, "A past one-shot is valid but spent")
		assert.Empty(t, preview.NextFiresUnixMs, "A spent trigger should have no upcoming fires")
		assert.NotNil(t, preview.NextFiresUnixMs, "An empty preview should serialize as an empty array")
	})

	t.Run("WindowBoundsThePreview", func(t *testing.T) {
		starts := now.Add(24 * time.Hour).UnixMilli()
		ends := now.Add(50 * time.Hour).UnixMilli()
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger:        trigger(cron.Expr("0 3 * * *", "")),
			StartsAtUnixMs: &starts,
			EndsAtUnixMs:   &ends,
		}, now)
		require.NoError(t, err, "A bounded window should preview")
		assert.Equal(t,
			[]int64{time.Date(2026, 7, 21, 3, 0, 0, 0, time.UTC).UnixMilli()},
			preview.NextFiresUnixMs,
			"Only fires inside the exact window should project",
		)
	})

	t.Run("InvalidInputsAreRejected", func(t *testing.T) {
		_, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Expr("not an expression", "")),
		}, now)
		require.ErrorIs(t, err, cron.ErrTriggerInvalid(""), "An unparsable expression should be rejected")

		starts := now.Add(2 * time.Hour).UnixMilli()
		ends := now.Add(time.Hour).UnixMilli()
		_, err = previewTriggerFires(PreviewFiresParams{
			Trigger:        trigger(cron.Every(time.Hour)),
			StartsAtUnixMs: &starts,
			EndsAtUnixMs:   &ends,
		}, now)
		require.ErrorIs(t, err, cron.ErrScheduleInvalid(""), "An inverted exact window should be rejected")
	})
}

func TestPreviewNextFires(t *testing.T) {
	base := time.Date(2026, 7, 19, 10, 0, 0, 0, time.Local)

	t.Run("LeadsWithThePersistedCursor", func(t *testing.T) {
		schedule := &cron.Schedule{
			Kind:             cron.TriggerOnce,
			FireAtUnixMs:     unixMillisPtr(base.Add(-time.Hour)),
			IsEnabled:        true,
			AnchorAtUnixMs:   base.Add(-2 * time.Hour).UnixMilli(),
			NextFireAtUnixMs: unixMillisPtr(base.Add(time.Minute)),
		}

		preview := previewNextFires(schedule, base, nextFiresPreview, time.Minute)
		assert.Equal(t, []int64{base.Add(time.Minute).UnixMilli()}, preview.NextFiresUnixMs,
			"The preview must lead with the persisted cursor")
	})

	t.Run("ContinuesFromThePersistedCursor", func(t *testing.T) {
		schedule := &cron.Schedule{
			Kind:             cron.TriggerInterval,
			EveryMs:          time.Minute.Milliseconds(),
			IsEnabled:        true,
			AnchorAtUnixMs:   base.UnixMilli(),
			NextFireAtUnixMs: unixMillisPtr(base.Add(30 * time.Second)),
		}

		preview := previewNextFires(schedule, base, 3, time.Minute)
		require.Len(t, preview.NextFiresUnixMs, 3, "The preview should fill the requested count")
		assert.Equal(t, base.Add(30*time.Second).UnixMilli(), preview.NextFiresUnixMs[0],
			"The persisted cursor should come first")
		assert.Greater(t, preview.NextFiresUnixMs[1], preview.NextFiresUnixMs[0],
			"The projection should continue past the cursor")
	})

	t.Run("PausedAndSpentSchedulesShowNothing", func(t *testing.T) {
		paused := &cron.Schedule{
			Kind:             cron.TriggerInterval,
			EveryMs:          time.Minute.Milliseconds(),
			AnchorAtUnixMs:   base.UnixMilli(),
			NextFireAtUnixMs: unixMillisPtr(base.Add(time.Minute)),
		}
		assert.Empty(t, previewNextFires(paused, base, 5, time.Minute).NextFiresUnixMs,
			"A paused schedule should preview nothing")

		spent := &cron.Schedule{
			Kind:           cron.TriggerOnce,
			FireAtUnixMs:   unixMillisPtr(base.Add(-time.Hour)),
			IsEnabled:      true,
			AnchorAtUnixMs: base.Add(-2 * time.Hour).UnixMilli(),
		}
		assert.Empty(t, previewNextFires(spent, base, 5, time.Minute).NextFiresUnixMs,
			"A spent one-shot should preview nothing")
	})

	t.Run("FallbackInstantsRemainDistinct", func(t *testing.T) {
		newYork, err := time.LoadLocation("America/New_York")
		require.NoError(t, err, "The fallback timezone should load")

		beforeFold := time.Date(2012, 11, 4, 0, 30, 0, 0, newYork)
		schedule := &cron.Schedule{
			Kind:           cron.TriggerCron,
			Expr:           "0 1 * * *",
			Timezone:       "America/New_York",
			IsEnabled:      true,
			AnchorAtUnixMs: beforeFold.UnixMilli(),
		}

		preview := previewNextFires(schedule, beforeFold, 2, time.Minute)
		require.Len(t, preview.NextFiresUnixMs, 2, "The fallback preview should retain both instants")
		assert.Equal(t, time.Hour.Milliseconds(), preview.NextFiresUnixMs[1]-preview.NextFiresUnixMs[0],
			"The repeated wall label must remain two exact instants")
	})

	t.Run("OverdueFireNowCollapsesTheGap", func(t *testing.T) {
		due := base.Add(-5 * time.Minute)
		schedule := &cron.Schedule{
			Kind:             cron.TriggerInterval,
			EveryMs:          time.Minute.Milliseconds(),
			MisfirePolicy:    cron.MisfireFireNow,
			IsEnabled:        true,
			AnchorAtUnixMs:   base.UnixMilli(),
			NextFireAtUnixMs: unixMillisPtr(due),
		}

		preview := previewNextFires(schedule, base, 3, time.Second)
		assert.Equal(t, []int64{
			due.UnixMilli(),
			base.Add(time.Minute).UnixMilli(),
			base.Add(2 * time.Minute).UnixMilli(),
		}, preview.NextFiresUnixMs, "Fire_now should show one catch-up and then future occurrences")
	})

	t.Run("OverdueSkipOmitsTheMissedGap", func(t *testing.T) {
		schedule := &cron.Schedule{
			Kind:             cron.TriggerInterval,
			EveryMs:          time.Minute.Milliseconds(),
			MisfirePolicy:    cron.MisfireSkip,
			IsEnabled:        true,
			AnchorAtUnixMs:   base.UnixMilli(),
			NextFireAtUnixMs: unixMillisPtr(base.Add(-5 * time.Minute)),
		}

		preview := previewNextFires(schedule, base, 3, time.Second)
		assert.Equal(t, []int64{
			base.Add(time.Minute).UnixMilli(),
			base.Add(2 * time.Minute).UnixMilli(),
			base.Add(3 * time.Minute).UnixMilli(),
		}, preview.NextFiresUnixMs, "Skip should omit every missed occurrence from the preview")
	})

	t.Run("NonPositiveCountReturnsAnEmptyArray", func(t *testing.T) {
		preview := previewNextFires(&cron.Schedule{IsEnabled: true}, base, -1, time.Minute)
		assert.Empty(t, preview.NextFiresUnixMs, "A non-positive count should project no fires")
		assert.NotNil(t, preview.NextFiresUnixMs, "An empty preview should serialize as an empty array")
	})
}
