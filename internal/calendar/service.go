package calendar

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var logger = logx.Named("calendar")

type service struct {
	loader calendar.DayLoader
	years  *cache.Invalidating[[]calendar.Day]
	// A read-through already loading old data must finish before an invalidation evicts it.
	mu sync.RWMutex
}

// NewService constructs the cached calendar and subscribes it to source changes.
func NewService(loader calendar.DayLoader, bus event.Bus) (calendar.Calendar, error) {
	s := &service{loader: loader}
	s.years = cache.NewInvalidating(func(ctx context.Context, key string) ([]calendar.Day, error) {
		year, err := strconv.Atoi(key)
		if err != nil {
			return nil, err
		}

		return s.loadYear(ctx, year)
	}, logger, cache.WithMemDefaultTTL(time.Hour), cache.WithMemMaxSize(128))

	_, err := event.SubscribeTyped[*calendar.ChangedEvent](bus, func(ctx context.Context, evt *calendar.ChangedEvent, _ event.Envelope) error {
		keys := make([]string, len(evt.Years))
		for i, year := range evt.Years {
			keys[i] = strconv.Itoa(year)
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		return s.years.Invalidate(ctx, keys...)
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe to calendar changes: %w", err)
	}

	return s, nil
}

func (s *service) loadYear(ctx context.Context, year int) ([]calendar.Day, error) {
	start := timex.DateOf(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC))
	end := start.EndOfYear()

	days, err := s.loader.LoadDays(ctx, start, end)
	if err != nil {
		return nil, err
	}

	if len(days) != end.YearDay() {
		return nil, fmt.Errorf("%w: %d dates for %d", errInvalidLoaderResult, len(days), year)
	}

	for i, d := range days {
		if d.Date.Year() != year || d.Date.YearDay() != i+1 || !d.Kind.IsValid() {
			return nil, fmt.Errorf("%w: invalid day at index %d for year %d", errInvalidLoaderResult, i, year)
		}
	}

	return slices.Clone(days), nil
}

type yearReader func(int) ([]calendar.Day, error)

// A transaction must read its own overrides using its existing connection.
// Memoize its years only for this operation, keeping uncommitted data out of
// the shared cache even if the surrounding transaction rolls back.
func (s *service) reader(ctx context.Context) yearReader {
	if db := contextx.DB(ctx); db != nil && db.InTx() {
		years := map[int][]calendar.Day{}

		return func(year int) ([]calendar.Day, error) {
			if days, ok := years[year]; ok {
				return days, nil
			}

			days, err := s.loadYear(ctx, year)
			if err == nil {
				years[year] = days
			}

			return days, err
		}
	}

	return func(year int) ([]calendar.Day, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		return s.years.Get(ctx, strconv.Itoa(year))
	}
}

func (s *service) Day(ctx context.Context, date timex.Date) (calendar.Day, error) {
	days, err := s.reader(ctx)(date.Year())
	if err != nil {
		return calendar.Day{}, err
	}

	return days[date.YearDay()-1], nil
}

func (s *service) Days(ctx context.Context, startsAt, endsAt timex.Date) ([]calendar.Day, error) {
	return s.reader(ctx).days(startsAt, endsAt)
}

func (read yearReader) days(startsAt, endsAt timex.Date) ([]calendar.Day, error) {
	if startsAt.After(endsAt) {
		return nil, errReversedDates
	}

	var days []calendar.Day
	for yr := startsAt.Year(); yr <= endsAt.Year(); yr++ {
		data, err := read(yr)
		if err != nil {
			return nil, err
		}

		lo, hi := 0, len(data)
		if yr == startsAt.Year() {
			lo = startsAt.YearDay() - 1
		}

		if yr == endsAt.Year() {
			hi = endsAt.YearDay()
		}

		days = append(days, data[lo:hi]...)
	}

	return days, nil
}

func (s *service) AddWorkdays(ctx context.Context, date timex.Date, n int) (timex.Date, error) {
	if n == 0 {
		return date, nil
	}

	step := 1
	if n < 0 {
		step = -1
	}

	read := s.reader(ctx)
	// Step through civil dates independently of local midnight transitions.
	cursor := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	var days []calendar.Day
	for range 3660 {
		cursor = cursor.AddDate(0, 0, step)
		if len(days) == 0 || days[0].Date.Year() != cursor.Year() {
			var err error

			days, err = read(cursor.Year())
			if err != nil {
				return timex.Date{}, err
			}
		}

		day := days[cursor.YearDay()-1]
		if day.Kind == calendar.Workday {
			n -= step
			if n == 0 {
				return day.Date, nil
			}
		}
	}

	return timex.Date{}, errWorkdayHorizon
}
