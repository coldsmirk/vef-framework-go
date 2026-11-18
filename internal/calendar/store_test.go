package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func TestCalendarOverrides(t *testing.T) {
	testx.ForEachDB(t, func(t *testing.T, env *testx.DBEnv) {
		bus := eventtest.NewFakeBus()
		cfg := &config.CalendarConfig{Workdays: []int{1, 2, 3, 4, 5}}
		s := newStore(env.DB, cfg, bus)
		ctx := t.Context()
		d := timex.DateOf(time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC))
		days, err := s.LoadDays(ctx, d, d.AddDays(2))
		require.NoError(t, err, "disabled store should work without the table")
		assert.Equal(
			t,
			[]calendar.Day{{Date: d, Kind: calendar.RestDay}, {Date: d.AddDays(1), Kind: calendar.RestDay}, {Date: d.AddDays(2), Kind: calendar.Workday}},
			days,
			"baseline should follow ISO weekdays",
		)

		cfg.Store.Enabled = true

		require.NoError(t, migrate(ctx, env.DB, env.DS.Kind), "schema should migrate")
		require.NoError(t, migrate(ctx, env.DB, env.DS.Kind), "schema migration should be idempotent")

		cal, err := NewService(s, bus)
		require.NoError(t, err, "calendar should subscribe")
		_, err = cal.Day(ctx, d)
		require.NoError(t, err, "cache should warm")

		day := calendar.Day{Date: d, Kind: calendar.Workday, Name: "Makeup day"}
		require.NoError(t, s.save(ctx, []calendar.Day{day}), "save should insert an exception")
		day.Name = "Updated"
		require.NoError(t, s.save(ctx, []calendar.Day{day}), "upsert should replace the name")
		got, err := cal.Day(ctx, d)
		require.NoError(t, err, "committed change should invalidate cache")
		assert.Equal(t, day, got, "latest override should win")

		rollback := errors.New("rollback fixture")
		err = env.DB.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
			ctx = contextx.SetDB(ctx, tx)
			require.NoError(t, s.write(ctx, nil, []timex.Date{d}), "nested delete should succeed")
			inside, err := cal.Day(ctx, d)
			require.NoError(t, err, "calendar should read on the open transaction")
			assert.Equal(t, calendar.RestDay, inside.Kind, "uncommitted removal should be visible to its caller")
			assert.Len(t, bus.Captured(), 2, "nested transaction must not publish before outer commit")

			return rollback
		})
		require.ErrorIs(t, err, rollback, "outer transaction should roll back")
		assert.Len(t, bus.Captured(), 2, "rolled back delete should publish no event")

		got, err = cal.Day(ctx, d)
		require.NoError(t, err, "shared cache should still be readable")
		assert.Equal(t, day, got, "rolled-back data must never reach the shared cache")

		day.Kind = calendar.RestDay
		require.NoError(t, s.save(ctx, []calendar.Day{day}), "saving the baseline should delete the override")
		count, err := env.DB.NewSelect().Model((*calendar.DayOverride)(nil)).Count(ctx)
		require.NoError(t, err, "override rows should be countable")
		assert.Zero(t, count, "baseline should consume no storage")

		got, err = cal.Day(ctx, d)
		require.NoError(t, err, "baseline should reload")
		assert.Equal(t, calendar.Day{Date: d, Kind: calendar.RestDay}, got, "baseline should have no override name")
	})
}
