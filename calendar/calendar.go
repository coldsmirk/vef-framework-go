package calendar

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// DayKind classifies a civil date for scheduling.
type DayKind string

const (
	Workday DayKind = "workday"
	RestDay DayKind = "rest_day"
	Holiday DayKind = "holiday"
)

// IsValid reports whether the kind is defined.
func (k DayKind) IsValid() bool { return k == Workday || k == RestDay || k == Holiday }

// Day describes one date and, for an override, its optional label.
type Day struct {
	Date timex.Date `json:"date"`
	Kind DayKind    `json:"kind"`
	Name string     `json:"name,omitempty"`
}

// DayOverride records an exception to the configured weekly baseline.
type DayOverride struct {
	orm.BaseModel `bun:"table:sys_calendar_override,alias:sco"`
	orm.FullAuditedModel

	Date timex.Date `json:"date" bun:"date"`
	Kind DayKind    `json:"kind" bun:"kind"`
	Name string     `json:"name" bun:"name"`
}

// DayLoader supplies the complete calendar. Hosts can replace the built-in loader.
type DayLoader interface {
	// LoadDays returns every date in the inclusive range in ascending order.
	LoadDays(ctx context.Context, startsAt, endsAt timex.Date) ([]Day, error)
}

// Calendar provides cached dates, workday arithmetic and recurring windows.
type Calendar interface {
	// Day returns the classification of one date.
	Day(ctx context.Context, date timex.Date) (Day, error)
	// Days returns every date in the inclusive range in ascending order.
	Days(ctx context.Context, startsAt, endsAt timex.Date) ([]Day, error)
	// AddWorkdays moves n workdays in either direction, excluding the starting date.
	AddWorkdays(ctx context.Context, date timex.Date, n int) (timex.Date, error)
	// Evaluate finds the open interval containing at and the following interval.
	Evaluate(ctx context.Context, rule WindowRule, at time.Time) (WindowStatus, error)
	// Windows returns up to limit merged intervals ending after from.
	Windows(ctx context.Context, rule WindowRule, from time.Time, limit int) ([]Interval, error)
}
