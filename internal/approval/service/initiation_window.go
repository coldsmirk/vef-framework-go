package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/result"
)

// WithCalendar supplies the calendar used to evaluate flow initiation windows.
func WithCalendar(cal calendar.Calendar) Option {
	return func(o *options) { o.calendar = cal }
}

// EvaluateInitiationWindow returns nil for an unrestricted flow. The caller supplies
// one instant for the entire request, including every flow on a page.
func (s *ValidationService) EvaluateInitiationWindow(ctx context.Context, rule *calendar.WindowRule, now time.Time) (*calendar.WindowStatus, error) {
	if rule == nil {
		return nil, nil
	}

	if s.calendar == nil {
		return nil, fmt.Errorf("evaluate initiation window without a calendar: %w", errors.ErrUnsupported)
	}

	status, err := s.calendar.Evaluate(ctx, *rule, now)
	if err != nil {
		return nil, err
	}

	return &status, nil
}

// CheckInitiationWindow admits new starts and their form only during an open interval.
func (s *ValidationService) CheckInitiationWindow(ctx context.Context, rule *calendar.WindowRule, now time.Time) (*calendar.WindowStatus, error) {
	status, err := s.EvaluateInitiationWindow(ctx, rule, now)
	if err != nil || status == nil || status.Open {
		return status, err
	}

	if status.Next == nil {
		return nil, approval.ErrInitiationWindowClosed
	}

	loc := time.Local
	if rule.Timezone != "" {
		loc, err = time.LoadLocation(rule.Timezone)
		if err != nil {
			return nil, err
		}
	}

	next := status.Next.StartsAt.In(loc)

	layout := "2006-01-02 15:04"
	if next.Second() != 0 {
		layout = time.DateTime
	}

	opening := next.Format(layout)
	if rule.Timezone != "" {
		opening += " " + rule.Timezone
	}

	return nil, result.Err(
		i18n.T(shared.ErrMessageInitiationWindowClosed, map[string]any{"next": opening}),
		result.WithCode(approval.ErrCodeInitiationWindowClosed),
	)
}
