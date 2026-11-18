package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

func TestInitiationWindowWithoutCalendar(t *testing.T) {
	svc := NewValidationService(nil)
	for _, tc := range []struct {
		name string
		call func(context.Context, *calendar.WindowRule, time.Time) (*calendar.WindowStatus, error)
	}{
		{"Evaluate", svc.EvaluateInitiationWindow},
		{"Check", svc.CheckInitiationWindow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			status, err := tc.call(t.Context(), nil, now)
			require.NoError(t, err, "an unrestricted flow should work without a calendar")
			assert.Nil(t, status, "unrestricted flows should have no window status")

			status, err = tc.call(t.Context(), &calendar.WindowRule{Windows: []calendar.Window{{}}}, now)
			require.ErrorIs(t, err, errors.ErrUnsupported, "a windowed flow should report the missing calendar")
			assert.Nil(t, status, "a missing calendar must not admit a windowed flow")
		})
	}
}

type nextOpeningCalendar struct {
	calendar.Calendar

	next time.Time
}

func (c nextOpeningCalendar) Evaluate(context.Context, calendar.WindowRule, time.Time) (calendar.WindowStatus, error) {
	return calendar.WindowStatus{Next: &calendar.Interval{StartsAt: c.next}}, nil
}

func TestInitiationWindowOpeningMessage(t *testing.T) {
	for _, tc := range []struct {
		name, timezone string
		next           time.Time
		want           string
	}{
		{"LocalMinutes", "", time.Date(2026, 11, 20, 9, 30, 0, 0, time.Local), "2026-11-20 09:30"},
		{"NamedZoneMinutes", "Asia/Shanghai", time.Date(2026, 11, 19, 16, 30, 0, 0, time.UTC), "2026-11-20 00:30 Asia/Shanghai"},
		{"NamedZoneSeconds", "Asia/Shanghai", time.Date(2026, 11, 19, 16, 30, 45, 0, time.UTC), "2026-11-20 00:30:45 Asia/Shanghai"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewValidationService(nil, WithCalendar(nextOpeningCalendar{next: tc.next}))
			rule := calendar.WindowRule{Timezone: tc.timezone, Windows: []calendar.Window{{}}}
			status, err := svc.CheckInitiationWindow(t.Context(), &rule, tc.next.Add(-time.Hour))
			require.ErrorIs(t, err, approval.ErrInitiationWindowClosed, "a future opening should return the closed-window error")
			assert.Nil(t, status, "a closed flow should not return an admitted status")
			assert.Equal(t, i18n.T(shared.ErrMessageInitiationWindowClosed, map[string]any{"next": tc.want}), err.Error(), "the message should show the opening in the rule's wall time")
		})
	}
}
