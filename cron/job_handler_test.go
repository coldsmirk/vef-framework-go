package cron

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJobHandler(t *testing.T) {
	t.Run("NameAndExecutePassThrough", func(t *testing.T) {
		var seen Execution

		handler := NewJobHandler("report.daily", func(_ context.Context, execution Execution) error {
			seen = execution

			return nil
		})

		assert.Equal(t, "report.daily", handler.Name(), "Handler must expose its job name")

		execution := Execution{RunID: "r1", ScheduleName: "s1", JobName: "report.daily"}
		require.NoError(t, handler.Execute(context.Background(), execution), "Execute must delegate")
		assert.Equal(t, execution, seen, "The execution descriptor must pass through verbatim")
	})

	t.Run("NoDefaultScheduleByDefault", func(t *testing.T) {
		handler := NewJobHandler("plain", func(context.Context, Execution) error { return nil })

		_, ok := handler.(DefaultScheduleProvider)
		assert.False(t, ok, "A handler without the option must not advertise a default schedule")
	})

	t.Run("WithDefaultSchedule", func(t *testing.T) {
		spec := ScheduleSpec{Trigger: Expr("0 2 * * *", "Asia/Shanghai")}
		handler := NewJobHandler("seeded", func(context.Context, Execution) error { return nil },
			WithDefaultSchedule(spec))

		provider, ok := handler.(DefaultScheduleProvider)
		require.True(t, ok, "The option must add the DefaultScheduleProvider capability")
		assert.Equal(t, spec, provider.DefaultSchedule(), "The shipped spec must return verbatim")
		assert.Equal(t, "seeded", handler.Name(), "Decoration must preserve the job name")
	})
}

func TestNewTypedJobHandler(t *testing.T) {
	type ReportParams struct {
		Region string `json:"region"`
		Limit  int    `json:"limit"`
	}

	t.Run("DecodesParams", func(t *testing.T) {
		var seen ReportParams

		handler := NewTypedJobHandler("typed", func(_ context.Context, params ReportParams) error {
			seen = params

			return nil
		})

		execution := Execution{Params: json.RawMessage(`{"region":"east","limit":10}`)}
		require.NoError(t, handler.Execute(context.Background(), execution), "Execute must succeed")
		assert.Equal(t, ReportParams{Region: "east", Limit: 10}, seen, "Params must decode into the typed model")
	})

	t.Run("AbsentParamsYieldZeroValue", func(t *testing.T) {
		var seen ReportParams

		handler := NewTypedJobHandler("typed", func(_ context.Context, params ReportParams) error {
			seen = params

			return nil
		})

		require.NoError(t, handler.Execute(context.Background(), Execution{}), "Execute must succeed")
		assert.Equal(t, ReportParams{}, seen, "Absent params must yield the zero value")
	})

	t.Run("DecodeFailureNamesTheJob", func(t *testing.T) {
		handler := NewTypedJobHandler("typed", func(context.Context, ReportParams) error {
			t.Fatal("The function must not run on a decode failure")

			return nil
		})

		err := handler.Execute(context.Background(), Execution{Params: json.RawMessage(`{"limit":"ten"}`)})
		require.Error(t, err, "A mistyped payload must fail")
		assert.Contains(t, err.Error(), `job "typed"`, "The error must name the job")
	})
}
