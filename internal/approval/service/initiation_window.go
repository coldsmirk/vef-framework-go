package service

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/i18n"
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

	return nil, result.Err(
		i18n.T(approval.ErrMessageInitiationWindowClosed, map[string]any{"next": status.Next.StartsAt.Format(time.RFC3339)}),
		result.WithCode(approval.ErrCodeInitiationWindowClosed),
	)
}
