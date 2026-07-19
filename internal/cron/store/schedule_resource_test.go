package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestPreviewTriggerFires(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 30, 0, 0, time.Local)

	trigger := func(spec cron.TriggerSpec) TriggerParams {
		params := TriggerParams{
			Kind:     spec.Kind,
			Expr:     spec.Expr,
			Timezone: spec.Timezone,
			EveryMs:  spec.EveryMs,
		}

		if spec.At != nil {
			at := timex.DateTime(*spec.At)
			params.At = &at
		}

		return params
	}

	t.Run("CronExpression", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Expr("0 3 * * *", "")),
		}, now)
		require.NoError(t, err, "A valid cron expression should preview")
		require.Len(t, preview.NextFires, nextFiresPreview, "The preview should project the full window")

		for i, fire := range preview.NextFires {
			expected := time.Date(2026, 7, 20+i, 3, 0, 0, 0, time.Local)
			assert.Equal(t, expected, fire.AsLocal(), "Fire %d should land at 03:00 on consecutive days", i)
		}
	})

	t.Run("IntervalAnchoredAtNow", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Every(time.Hour)),
		}, now)
		require.NoError(t, err, "A valid interval should preview")
		require.Len(t, preview.NextFires, nextFiresPreview, "The preview should project the full window")

		for i, fire := range preview.NextFires {
			assert.Equal(t, now.Add(time.Duration(i+1)*time.Hour), fire.AsLocal(),
				"Fire %d should step one hour from the creation anchor", i)
		}
	})

	t.Run("OnceYieldsSingleFire", func(t *testing.T) {
		at := now.Add(time.Hour)

		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Once(at)),
		}, now)
		require.NoError(t, err, "A future one-shot should preview")
		require.Len(t, preview.NextFires, 1, "A one-shot yields exactly one fire")
		assert.Equal(t, at, preview.NextFires[0].AsLocal(), "The fire should be the requested instant")
	})

	t.Run("SpentTriggerPreviewsEmpty", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Once(now.Add(-time.Hour))),
		}, now)
		require.NoError(t, err, "A past one-shot is valid, just spent")
		assert.Empty(t, preview.NextFires, "A spent trigger has no upcoming fires")
		assert.NotNil(t, preview.NextFires, "The empty preview should serialize as [], not null")
	})

	t.Run("WindowBoundsThePreview", func(t *testing.T) {
		starts := timex.DateTime(now.Add(24 * time.Hour))
		ends := timex.DateTime(now.Add(50 * time.Hour))

		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger:  trigger(cron.Expr("0 3 * * *", "")),
			StartsAt: &starts,
			EndsAt:   &ends,
		}, now)
		require.NoError(t, err, "A bounded window should preview")
		require.Len(t, preview.NextFires, 1, "Only the fires inside the window should project")
		assert.Equal(t, time.Date(2026, 7, 21, 3, 0, 0, 0, time.Local), preview.NextFires[0].AsLocal(),
			"The fire should be the first 03:00 after the window opens")
	})

	t.Run("InvalidExpressionRejected", func(t *testing.T) {
		_, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Expr("not an expression", "")),
		}, now)
		require.ErrorIs(t, err, cron.ErrTriggerInvalid(""), "An unparsable expression should fail like a save would")
	})

	t.Run("UnknownKindRejected", func(t *testing.T) {
		_, err := previewTriggerFires(PreviewFiresParams{
			Trigger: TriggerParams{Kind: "hourly"},
		}, now)
		require.ErrorIs(t, err, cron.ErrTriggerInvalid(""), "A kind outside the vocabulary should be rejected")
	})

	t.Run("InvertedWindowRejected", func(t *testing.T) {
		starts := timex.DateTime(now.Add(2 * time.Hour))
		ends := timex.DateTime(now.Add(time.Hour))

		_, err := previewTriggerFires(PreviewFiresParams{
			Trigger:  trigger(cron.Every(time.Hour)),
			StartsAt: &starts,
			EndsAt:   &ends,
		}, now)
		require.ErrorIs(t, err, cron.ErrScheduleInvalid(""), "An inverted window should fail like a save would")
	})

	t.Run("TimezoneEvaluatesExpression", func(t *testing.T) {
		preview, err := previewTriggerFires(PreviewFiresParams{
			Trigger: trigger(cron.Expr("0 3 * * *", "UTC")),
		}, now)
		require.NoError(t, err, "A zoned expression should preview")
		require.NotEmpty(t, preview.NextFires, "The zoned expression should yield fires")

		first := preview.NextFires[0].AsLocal().UTC()
		assert.Equal(t, 3, first.Hour(), "The fire should land at 03:00 in the trigger's zone")
	})
}
