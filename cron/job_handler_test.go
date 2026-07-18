package cron

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestNewJobHandler(t *testing.T) {
	t.Run("NameAndExecutePassThrough", func(t *testing.T) {
		var seen Execution

		handler := NewJobHandler("report.daily", func(_ context.Context, execution Execution) error {
			seen = execution

			return nil
		})

		assert.Equal(t, "report.daily", handler.Name(), "handler must expose its job name")

		execution := Execution{RunID: "r1", ScheduleName: "s1", JobName: "report.daily"}
		require.NoError(t, handler.Execute(context.Background(), execution), "execute must delegate")
		assert.Equal(t, execution, seen, "the execution descriptor must pass through verbatim")
	})

	t.Run("NoDefaultScheduleByDefault", func(t *testing.T) {
		handler := NewJobHandler("plain", func(context.Context, Execution) error { return nil })

		_, ok := handler.(DefaultScheduleProvider)
		assert.False(t, ok, "a handler without the option must not advertise a default schedule")
	})

	t.Run("WithDefaultSchedule", func(t *testing.T) {
		spec := ScheduleSpec{Trigger: Expr("0 2 * * *", "Asia/Shanghai")}
		handler := NewJobHandler("seeded", func(context.Context, Execution) error { return nil },
			WithDefaultSchedule(spec))

		provider, ok := handler.(DefaultScheduleProvider)
		require.True(t, ok, "the option must add the DefaultScheduleProvider capability")
		assert.Equal(t, spec, provider.DefaultSchedule(), "the shipped spec must return verbatim")
		assert.Equal(t, "seeded", handler.Name(), "decoration must preserve the job name")
	})
}

func TestNewTypedJobHandler(t *testing.T) {
	type reportParams struct {
		Region string `json:"region"`
		Limit  int    `json:"limit"`
	}

	t.Run("DecodesParams", func(t *testing.T) {
		var seen reportParams

		handler := NewTypedJobHandler("typed", func(_ context.Context, params reportParams) error {
			seen = params

			return nil
		})

		execution := Execution{Params: json.RawMessage(`{"region":"east","limit":10}`)}
		require.NoError(t, handler.Execute(context.Background(), execution), "execute must succeed")
		assert.Equal(t, reportParams{Region: "east", Limit: 10}, seen, "params must decode into the typed model")
	})

	t.Run("AbsentParamsYieldZeroValue", func(t *testing.T) {
		var seen reportParams

		handler := NewTypedJobHandler("typed", func(_ context.Context, params reportParams) error {
			seen = params

			return nil
		})

		require.NoError(t, handler.Execute(context.Background(), Execution{}), "execute must succeed")
		assert.Equal(t, reportParams{}, seen, "absent params must yield the zero value")
	})

	t.Run("DecodeFailureNamesTheJob", func(t *testing.T) {
		handler := NewTypedJobHandler("typed", func(context.Context, reportParams) error {
			t.Fatal("the function must not run on a decode failure")

			return nil
		})

		err := handler.Execute(context.Background(), Execution{Params: json.RawMessage(`{"limit":"ten"}`)})
		require.Error(t, err, "a mistyped payload must fail")
		assert.Contains(t, err.Error(), `job "typed"`, "the error must name the job")
	})
}

func TestExecutionBindParams(t *testing.T) {
	t.Run("AbsentParamsLeaveTargetUntouched", func(t *testing.T) {
		target := map[string]string{"keep": "me"}

		require.NoError(t, Execution{}.BindParams(&target), "binding absent params must succeed")
		assert.Equal(t, map[string]string{"keep": "me"}, target, "the target must stay untouched")
	})

	t.Run("MalformedParamsFail", func(t *testing.T) {
		var target map[string]string

		err := Execution{Params: json.RawMessage(`{broken`)}.BindParams(&target)
		assert.Error(t, err, "malformed params must fail loudly")
	})
}

func TestScheduleTrigger(t *testing.T) {
	t.Run("RoundTrip", func(t *testing.T) {
		// Local wall clock: Trigger() reinterprets the stored fire time via
		// AsLocal, so a local-zone fixture round-trips exactly.
		at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
		schedule := &Schedule{
			Kind:     TriggerOnce,
			Expr:     "",
			Timezone: "",
			EveryMs:  0,
		}
		fireAt := timex.DateTime(at)
		schedule.FireAt = &fireAt

		spec := schedule.Trigger()
		assert.Equal(t, TriggerOnce, spec.Kind, "the kind must round-trip")
		require.NotNil(t, spec.At, "the fire time must round-trip")
		assert.True(t, spec.At.Equal(at), "the fire time value must round-trip")
	})

	t.Run("Timeout", func(t *testing.T) {
		schedule := &Schedule{TimeoutMs: 1500}
		assert.Equal(t, 1500*time.Millisecond, schedule.Timeout(), "TimeoutMs must convert to a duration")
	})
}
