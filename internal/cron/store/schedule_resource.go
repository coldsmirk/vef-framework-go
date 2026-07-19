package store

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// nextFiresPreview is how many upcoming fire times the detail view projects.
const nextFiresPreview = 5

// TriggerParams is the trigger section of a schedule mutation.
type TriggerParams struct {
	Kind     cron.TriggerKind `json:"kind" validate:"required"`
	Expr     string           `json:"expr"`
	Timezone string           `json:"timezone"`
	EveryMs  int64            `json:"everyMs"`
	At       *timex.DateTime  `json:"at"`
}

// spec converts the wire form into the trigger spec.
func (p TriggerParams) spec() cron.TriggerSpec {
	spec := cron.TriggerSpec{
		Kind:     p.Kind,
		Expr:     p.Expr,
		Timezone: p.Timezone,
		EveryMs:  p.EveryMs,
	}

	if p.At != nil {
		at := p.At.Unwrap()
		spec.At = &at
	}

	return spec
}

// ScheduleParams contains the create/update parameters of a schedule. On
// update, Name addresses the schedule and NewName optionally renames it.
type ScheduleParams struct {
	api.P

	Name              string                 `json:"name" validate:"required"`
	NewName           string                 `json:"newName"`
	JobName           string                 `json:"jobName" validate:"required"`
	Trigger           TriggerParams          `json:"trigger"`
	Params            map[string]any         `json:"params"`
	StartsAt          *timex.DateTime        `json:"startsAt"`
	EndsAt            *timex.DateTime        `json:"endsAt"`
	MisfirePolicy     cron.MisfirePolicy     `json:"misfirePolicy"`
	ConcurrencyPolicy cron.ConcurrencyPolicy `json:"concurrencyPolicy"`
	Recover           bool                   `json:"recover"`
	TimeoutMs         int64                  `json:"timeoutMs"`
	Enabled           *bool                  `json:"enabled"`
}

// spec converts the wire form into the schedule spec; rename indicates
// whether NewName addresses a different name than the target.
func (p ScheduleParams) spec() cron.ScheduleSpec {
	spec := cron.ScheduleSpec{
		Name:              p.Name,
		JobName:           p.JobName,
		Trigger:           p.Trigger.spec(),
		MisfirePolicy:     p.MisfirePolicy,
		ConcurrencyPolicy: p.ConcurrencyPolicy,
		Recover:           p.Recover,
		Timeout:           time.Duration(p.TimeoutMs) * time.Millisecond,
		Enabled:           p.Enabled,
	}

	if p.Params != nil {
		spec.Params = p.Params
	}

	if p.StartsAt != nil {
		starts := p.StartsAt.Unwrap()
		spec.StartsAt = &starts
	}

	if p.EndsAt != nil {
		ends := p.EndsAt.Unwrap()
		spec.EndsAt = &ends
	}

	return spec
}

// ScheduleNameParams addresses one schedule by name.
type ScheduleNameParams struct {
	api.P

	Name string `json:"name" validate:"required"`
}

// PreviewFiresParams carries an unsaved trigger whose upcoming fire times
// the editor wants to preview before persisting.
type PreviewFiresParams struct {
	api.P

	Trigger  TriggerParams   `json:"trigger"`
	StartsAt *timex.DateTime `json:"startsAt"`
	EndsAt   *timex.DateTime `json:"endsAt"`
}

// FiresPreview is the preview_fires response: the trigger's upcoming fire
// times from now; empty when it yields no occurrence inside its window.
type FiresPreview struct {
	NextFires []timex.DateTime `json:"nextFires"`
}

// ScheduleSearch contains the search parameters for schedules.
type ScheduleSearch struct {
	crud.Sortable

	Name      string `json:"name" search:"contains"`
	JobName   string `json:"jobName" search:"eq,column=job_name"`
	Kind      string `json:"kind" search:"eq"`
	IsEnabled *bool  `json:"isEnabled" search:"eq,column=is_enabled"`
}

// ScheduleDetail is the get response: the schedule plus a preview of its
// upcoming fire times.
type ScheduleDetail struct {
	Schedule *cron.Schedule `json:"schedule"`
	// NextFires previews the next few fire times from now; empty when the
	// schedule is paused or spent.
	NextFires []timex.DateTime `json:"nextFires"`
}

// ScheduleResource manages durable schedules: paged browsing plus the
// control operations, all delegated to the ScheduleManager so API mutations
// and programmatic ones share one validation and wake path.
type ScheduleResource struct {
	api.Resource

	crud.FindPage[cron.Schedule, ScheduleSearch]

	manager  cron.ScheduleManager
	registry *Registry
	now      func() time.Time
}

// NewScheduleResource creates the schedule management resource. With the
// store disabled the resource mounts no operations — a feature that is off
// exposes no surface.
func NewScheduleResource(cfg *config.CronConfig, manager cron.ScheduleManager, registry *Registry) api.Resource {
	const name = "sys/cron/schedule"

	if !cfg.Store.Enabled {
		return api.NewRPCResource(name)
	}

	return &ScheduleResource{
		Resource: api.NewRPCResource(
			name,
			api.WithOperations(
				api.OperationSpec{Action: "get", RequiredPermission: "cron.schedule.query"},
				api.OperationSpec{Action: "list_jobs", RequiredPermission: "cron.schedule.query"},
				api.OperationSpec{Action: "preview_fires", RequiredPermission: "cron.schedule.query"},
				api.OperationSpec{Action: "create", RequiredPermission: "cron.schedule.manage", EnableAudit: true},
				api.OperationSpec{Action: "update", RequiredPermission: "cron.schedule.manage", EnableAudit: true},
				api.OperationSpec{Action: "delete", RequiredPermission: "cron.schedule.manage", EnableAudit: true},
				api.OperationSpec{Action: "pause", RequiredPermission: "cron.schedule.manage", EnableAudit: true},
				api.OperationSpec{Action: "resume", RequiredPermission: "cron.schedule.manage", EnableAudit: true},
				api.OperationSpec{Action: "trigger_now", RequiredPermission: "cron.schedule.manage", EnableAudit: true},
			),
		),
		FindPage: crud.NewFindPage[cron.Schedule, ScheduleSearch]().
			RequiredPermission("cron.schedule.query"),
		manager:  manager,
		registry: registry,
		now:      func() time.Time { return timex.Now().Unwrap() },
	}
}

// ListJobs returns the job names registered on this node — the vocabulary
// the schedule editor's job picker offers. Heterogeneous deployments may
// register different sets per node; the answering node's view is returned.
func (r *ScheduleResource) ListJobs(ctx fiber.Ctx) error {
	return result.Ok(r.registry.Names()).Response(ctx)
}

// Get returns one schedule with its upcoming-fire preview.
func (r *ScheduleResource) Get(ctx fiber.Ctx, params ScheduleNameParams) error {
	schedule, err := r.manager.Get(ctx.Context(), params.Name)
	if err != nil {
		return err
	}

	return result.Ok(&ScheduleDetail{
		Schedule:  schedule,
		NextFires: previewNextFires(schedule, r.now(), nextFiresPreview),
	}).Response(ctx)
}

// PreviewFires projects the upcoming fire times of an unsaved trigger, so
// the editor validates an expression against the real parser before saving.
func (r *ScheduleResource) PreviewFires(ctx fiber.Ctx, params PreviewFiresParams) error {
	preview, err := previewTriggerFires(params, r.now())
	if err != nil {
		return err
	}

	return result.Ok(preview).Response(ctx)
}

// Create persists a new schedule.
func (r *ScheduleResource) Create(ctx fiber.Ctx, params ScheduleParams) error {
	schedule, err := r.manager.Create(ctx.Context(), params.spec())
	if err != nil {
		return err
	}

	return result.Ok(schedule).Response(ctx)
}

// Update reshapes the named schedule; NewName renames it.
func (r *ScheduleResource) Update(ctx fiber.Ctx, params ScheduleParams) error {
	spec := params.spec()
	if params.NewName != "" {
		spec.Name = params.NewName
	}

	schedule, err := r.manager.Update(ctx.Context(), params.Name, spec)
	if err != nil {
		return err
	}

	return result.Ok(schedule).Response(ctx)
}

// Delete removes the schedule; its journaled runs are kept.
func (r *ScheduleResource) Delete(ctx fiber.Ctx, params ScheduleNameParams) error {
	if err := r.manager.Delete(ctx.Context(), params.Name); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// Pause stops fire claiming until resume.
func (r *ScheduleResource) Pause(ctx fiber.Ctx, params ScheduleNameParams) error {
	if err := r.manager.Pause(ctx.Context(), params.Name); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// Resume re-enables the schedule under its misfire policy.
func (r *ScheduleResource) Resume(ctx fiber.Ctx, params ScheduleNameParams) error {
	if err := r.manager.Resume(ctx.Context(), params.Name); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// TriggerNow requests one immediate fire through the regular claim path.
func (r *ScheduleResource) TriggerNow(ctx fiber.Ctx, params ScheduleNameParams) error {
	if err := r.manager.TriggerNow(ctx.Context(), params.Name); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// previewTriggerFires validates the unsaved trigger and projects its fire
// times from now — the editor-time counterpart of the detail preview. The
// trigger and window checks mirror the manager's save-time validation so the
// preview rejects exactly what a save would.
func previewTriggerFires(params PreviewFiresParams, now time.Time) (*FiresPreview, error) {
	spec := params.Trigger.spec()
	if err := spec.Validate(); err != nil {
		return nil, cron.ErrTriggerInvalid(err.Error())
	}

	if params.StartsAt != nil && params.EndsAt != nil && !params.EndsAt.Unwrap().After(params.StartsAt.Unwrap()) {
		return nil, cron.ErrScheduleInvalid(ErrScheduleWindowInverted.Error())
	}

	// CreatedAt doubles as the interval anchor — stamped now, exactly what an
	// immediate save would persist.
	transient := &cron.Schedule{
		Kind:      spec.Kind,
		Expr:      spec.Expr,
		Timezone:  spec.Timezone,
		EveryMs:   spec.EveryMs,
		StartsAt:  params.StartsAt,
		EndsAt:    params.EndsAt,
		IsEnabled: true,
	}
	transient.CreatedAt = timex.DateTime(now)

	if spec.At != nil {
		at := timex.DateTime(*spec.At)
		transient.FireAt = &at
	}

	return &FiresPreview{NextFires: previewNextFires(transient, now, nextFiresPreview)}, nil
}

// previewNextFires projects the schedule's next fire times from the given
// instant.
func previewNextFires(schedule *cron.Schedule, from time.Time, count int) []timex.DateTime {
	fires := make([]timex.DateTime, 0, count)

	if !schedule.IsEnabled {
		return fires
	}

	cursor := from
	for range count {
		next, ok := nextFire(schedule, cursor)
		if !ok {
			break
		}

		fires = append(fires, timex.DateTime(next))
		cursor = next
	}

	return fires
}
