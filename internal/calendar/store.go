package calendar

import (
	"context"
	"maps"
	"slices"

	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type store struct {
	db  orm.DB
	cfg *config.CalendarConfig
	bus event.Bus
}

func newStore(db orm.DB, cfg *config.CalendarConfig, bus event.Bus) *store {
	return &store{db: db, cfg: cfg, bus: bus}
}

func (s *store) baseline(date timex.Date) calendar.DayKind {
	if slices.Contains(s.cfg.EffectiveWorkdays(), (int(date.Weekday())+6)%7+1) {
		return calendar.Workday
	}

	return calendar.RestDay
}

func (s *store) LoadDays(ctx context.Context, startsAt, endsAt timex.Date) ([]calendar.Day, error) {
	if startsAt.After(endsAt) {
		return nil, errReversedDates
	}

	var overrides []calendar.DayOverride
	if s.cfg.Store.Enabled {
		if err := contextx.DB(ctx, s.db).NewSelect().Model(&overrides).Select("date", "kind", "name").Where(func(cb orm.ConditionBuilder) {
			cb.Between("date", startsAt, endsAt)
		}).OrderBy("date").Scan(ctx); err != nil {
			return nil, err
		}
	}

	var days []calendar.Day
	for d := startsAt; !d.After(endsAt); d = d.AddDays(1) {
		day := calendar.Day{Date: d, Kind: s.baseline(d)}
		if len(overrides) > 0 && overrides[0].Date.Equal(d) {
			day.Kind, day.Name = overrides[0].Kind, overrides[0].Name
			overrides = overrides[1:]
		}

		days = append(days, day)
	}

	return days, nil
}

func (s *store) save(ctx context.Context, days []calendar.Day) error {
	// PostgreSQL cannot update the same conflict target twice in one INSERT.
	// Keep the last submitted value for each civil date in every dialect.
	byDate := make(map[string]calendar.Day, len(days))
	for _, day := range days {
		byDate[day.Date.String()] = day
	}

	var (
		rows    []calendar.DayOverride
		removed []timex.Date
	)
	for _, key := range slices.Sorted(maps.Keys(byDate)) {
		d := byDate[key]
		if d.Kind == s.baseline(d.Date) {
			removed = append(removed, d.Date)
		} else {
			rows = append(rows, calendar.DayOverride{Date: d.Date, Kind: d.Kind, Name: d.Name})
		}
	}

	return s.write(ctx, rows, removed)
}

func (s *store) write(ctx context.Context, rows []calendar.DayOverride, removed []timex.Date) error {
	if len(rows)+len(removed) == 0 {
		return nil
	}

	return contextx.DB(ctx, s.db).RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		years := map[int]struct{}{}
		for _, d := range removed {
			years[d.Year()] = struct{}{}
		}

		for _, r := range rows {
			years[r.Date.Year()] = struct{}{}
		}

		for dates := range slices.Chunk(removed, 400) {
			if _, err := tx.NewDelete().Model((*calendar.DayOverride)(nil)).Where(func(cb orm.ConditionBuilder) { cb.In("date", dates) }).Exec(ctx); err != nil {
				return err
			}
		}

		if len(rows) > 0 {
			if _, err := tx.NewInsert().Model(&rows).OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("date").DoUpdate().Set("kind").Set("name").Set("updated_at").Set("updated_by")
			}).Exec(ctx); err != nil {
				return err
			}
		}

		return orm.OnCommit(ctx, func(ctx context.Context) {
			if err := calendar.PublishChangedEvent(ctx, s.bus, slices.Sorted(maps.Keys(years))...); err != nil {
				logger.Errorf("Publish calendar change: %v", err)
			}
		})
	})
}
