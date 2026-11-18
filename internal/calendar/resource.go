package calendar

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/calendar"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Resource exposes the work calendar and recurring window preview.
type Resource struct {
	api.Resource

	calendar calendar.Calendar
	store    *store
}

func newResource(cal calendar.Calendar, s *store) api.Resource {
	ops := []api.OperationSpec{{Action: "find_days"}, {Action: "preview_windows"}}
	if s.cfg.Store.Enabled {
		ops = append(
			ops,
			api.OperationSpec{Action: "save_overrides", RequiredPermission: "sys.calendar.manage", EnableAudit: true},
			api.OperationSpec{Action: "delete_overrides", RequiredPermission: "sys.calendar.manage", EnableAudit: true},
		)
	}

	return &Resource{Resource: api.NewRPCResource("sys/calendar", api.WithOperations(ops...)), calendar: cal, store: s}
}

type FindDaysParams struct {
	api.P

	StartsAt timex.Date `json:"startsAt" validate:"required"`
	EndsAt   timex.Date `json:"endsAt" validate:"required"`
}

func (r *Resource) FindDays(ctx fiber.Ctx, p FindDaysParams) error {
	if p.StartsAt.IsZero() || p.EndsAt.IsZero() || p.StartsAt.After(p.EndsAt) || p.EndsAt.After(p.StartsAt.AddDays(365)) {
		return errInvalidDates
	}

	days, err := r.calendar.Days(ctx.Context(), p.StartsAt, p.EndsAt)
	if err != nil {
		return err
	}

	return result.Ok(days).Response(ctx)
}

type PreviewWindowsParams struct {
	api.P

	Rule  calendar.WindowRule `json:"rule"`
	From  *int64              `json:"from"`
	Limit *int                `json:"limit,omitempty" validate:"omitempty,min=1,max=50"`
}

type preview struct {
	Status  calendar.WindowStatus `json:"status"`
	Windows []calendar.Interval   `json:"windows"`
}

func (r *Resource) PreviewWindows(ctx fiber.Ctx, p PreviewWindowsParams) error {
	from := time.Now()
	if p.From != nil {
		from = time.UnixMilli(*p.From)
	}

	limit := 10
	if p.Limit != nil {
		limit = *p.Limit
	}

	windows, err := r.calendar.Windows(ctx.Context(), p.Rule, from, max(limit, 2))
	if err != nil {
		return err
	}

	return result.Ok(preview{Status: windowStatus(windows, from), Windows: windows[:min(limit, len(windows))]}).Response(ctx)
}

type SaveOverridesParams struct {
	api.P

	Overrides []calendar.Day `json:"overrides" validate:"required,max=400,dive"`
}

func (r *Resource) SaveOverrides(ctx fiber.Ctx, p SaveOverridesParams) error {
	if err := r.store.save(ctx.Context(), p.Overrides); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

type DeleteOverridesParams struct {
	api.P

	Dates []timex.Date `json:"dates" validate:"required,dive,required"`
}

func (r *Resource) DeleteOverrides(ctx fiber.Ctx, p DeleteOverridesParams) error {
	if err := r.store.write(ctx.Context(), nil, p.Dates); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}
