package store

import (
	"encoding/json"
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
	Expr     *string          `json:"expr"`
	Timezone *string          `json:"timezone"`
	EveryMs  *int64           `json:"everyMs"`
	AtUnixMs *int64           `json:"atUnixMs"`
}

// spec converts the wire form into the trigger spec. The kind-field conflict
// rule is deliberately stricter than cron.TriggerSpec.hasFieldConflict, which
// owns the same rule for Go callers: a wire request that names a field of
// another kind is a client mistake even when it carries that kind's zero
// value, so presence is the test here where the spec tests the value. Keep
// the two in lockstep whenever the trigger kinds change.
func (p TriggerParams) spec() (cron.TriggerSpec, error) {
	conflict := false
	switch p.Kind {
	case cron.TriggerCron:
		conflict = p.EveryMs != nil || p.AtUnixMs != nil
	case cron.TriggerInterval:
		conflict = p.Expr != nil || p.Timezone != nil || p.AtUnixMs != nil
	case cron.TriggerOnce:
		conflict = p.Expr != nil || p.Timezone != nil || p.EveryMs != nil
	}

	if conflict {
		return cron.TriggerSpec{}, cron.ErrTriggerInvalid(cron.ErrTriggerFieldsConflict.Error())
	}

	spec := cron.TriggerSpec{Kind: p.Kind, At: unixTimePtr(p.AtUnixMs)}
	if p.Expr != nil {
		spec.Expr = *p.Expr
	}

	if p.Timezone != nil {
		spec.Timezone = *p.Timezone
	}

	if p.EveryMs != nil {
		spec.EveryMs = *p.EveryMs
	}

	return spec, nil
}

// ScheduleParams contains the create/update parameters of a schedule. On
// update, Name addresses the schedule and NewName optionally renames it.
type ScheduleParams struct {
	api.P

	Name              string                 `json:"name" validate:"required"`
	NewName           string                 `json:"newName"`
	JobName           string                 `json:"jobName" validate:"required"`
	Trigger           TriggerParams          `json:"trigger"`
	Params            json.RawMessage        `json:"params"`
	StartsAtUnixMs    *int64                 `json:"startsAtUnixMs"`
	EndsAtUnixMs      *int64                 `json:"endsAtUnixMs"`
	MisfirePolicy     cron.MisfirePolicy     `json:"misfirePolicy"`
	ConcurrencyPolicy cron.ConcurrencyPolicy `json:"concurrencyPolicy"`
	Recover           bool                   `json:"recover"`
	TimeoutMs         int64                  `json:"timeoutMs"`
	Enabled           *bool                  `json:"enabled"`
}

// spec validates duration representation and converts the wire form into the
// schedule spec. Update applies NewName after this conversion.
func (p ScheduleParams) spec() (cron.ScheduleSpec, error) {
	if p.TimeoutMs < 0 {
		return cron.ScheduleSpec{}, cron.ErrScheduleInvalid(ErrScheduleTimeoutNegative.Error())
	}

	if p.TimeoutMs > cron.MaxDurationMilliseconds {
		return cron.ScheduleSpec{}, cron.ErrScheduleInvalid(ErrScheduleTimeoutTooLong.Error())
	}

	trigger, err := p.Trigger.spec()
	if err != nil {
		return cron.ScheduleSpec{}, err
	}

	spec := cron.ScheduleSpec{
		Name:              p.Name,
		JobName:           p.JobName,
		Trigger:           trigger,
		MisfirePolicy:     p.MisfirePolicy,
		ConcurrencyPolicy: p.ConcurrencyPolicy,
		Recover:           p.Recover,
		Timeout:           time.Duration(p.TimeoutMs) * time.Millisecond,
		Enabled:           p.Enabled,
	}

	if p.Params != nil {
		spec.Params = p.Params
	}

	spec.StartsAt = unixTimePtr(p.StartsAtUnixMs)
	spec.EndsAt = unixTimePtr(p.EndsAtUnixMs)

	return spec, nil
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

	Trigger        TriggerParams `json:"trigger"`
	StartsAtUnixMs *int64        `json:"startsAtUnixMs"`
	EndsAtUnixMs   *int64        `json:"endsAtUnixMs"`
}

// FiresPreview is the preview_fires response: the trigger's upcoming fire
// times from now; empty when it yields no occurrence inside its window.
type FiresPreview struct {
	NextFiresUnixMs []int64 `json:"nextFiresUnixMs"`
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
	// NextFiresUnixMs previews the next few exact fire times from now. An
	// overdue cursor is projected through the schedule's misfire policy; a
	// paused or spent schedule returns an empty list.
	NextFiresUnixMs []int64 `json:"nextFiresUnixMs"`
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
	// misfireThreshold keeps the detail preview on the same decision boundary
	// as the claim path when its persisted cursor is already overdue.
	misfireThreshold time.Duration
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
		manager:          manager,
		registry:         registry,
		now:              func() time.Time { return timex.Now().Unwrap() },
		misfireThreshold: cfg.Store.EffectiveMisfireThreshold(),
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

	preview := previewNextFires(schedule, r.now(), nextFiresPreview, r.misfireThreshold)

	return result.Ok(&ScheduleDetail{
		Schedule:        schedule,
		NextFiresUnixMs: preview.NextFiresUnixMs,
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
	spec, err := params.spec()
	if err != nil {
		return err
	}

	schedule, err := r.manager.Create(ctx.Context(), spec)
	if err != nil {
		return err
	}

	return result.Ok(schedule).Response(ctx)
}

// Update reshapes the named schedule; NewName renames it.
func (r *ScheduleResource) Update(ctx fiber.Ctx, params ScheduleParams) error {
	spec, err := params.spec()
	if err != nil {
		return err
	}

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

// TriggerNow persists one immediate fire without moving the regular cursor.
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
	spec, err := params.Trigger.spec()
	if err != nil {
		return nil, err
	}

	if err := spec.Validate(); err != nil {
		return nil, cron.ErrTriggerInvalid(err.Error())
	}

	starts := unixTimePtr(params.StartsAtUnixMs)

	ends := unixTimePtr(params.EndsAtUnixMs)
	if starts != nil && ends != nil && !ends.After(*starts) {
		return nil, cron.ErrScheduleInvalid(ErrScheduleWindowInverted.Error())
	}

	// The transient anchor is stamped now, exactly what an immediate save
	// would persist. A StartsAt below replaces it as the interval phase.
	transient := &cron.Schedule{
		Kind:           spec.Kind,
		Expr:           spec.Expr,
		Timezone:       spec.Timezone,
		EveryMs:        spec.EveryMs,
		IsEnabled:      true,
		AnchorAtUnixMs: now.UnixMilli(),
	}

	if starts != nil {
		transient.StartsAtUnixMs = unixMsPtr(*starts)
		transient.AnchorAtUnixMs = starts.UnixMilli()
	}

	if ends != nil {
		transient.EndsAtUnixMs = unixMsPtr(*ends)
	}

	if spec.At != nil {
		transient.FireAtUnixMs = unixMsPtr(*spec.At)
	}

	return previewNextFires(transient, now, nextFiresPreview, 0), nil
}

// previewNextFires projects the schedule's next fire times from the given
// instant. A persisted cursor leads the projection: a future cursor is the
// fire the engine will actually claim, while an overdue cursor goes through
// the same misfire decision as the claim path. Recomputing purely from the
// trigger would hide a pending one-shot or overdue regular occurrence.
func previewNextFires(
	schedule *cron.Schedule,
	from time.Time,
	count int,
	misfireThreshold time.Duration,
) *FiresPreview {
	if count <= 0 {
		return &FiresPreview{NextFiresUnixMs: []int64{}}
	}

	preview := &FiresPreview{NextFiresUnixMs: make([]int64, 0, count)}
	if !schedule.IsEnabled {
		return preview
	}

	cursor := from

	if schedule.NextFireAtUnixMs != nil {
		pending := unixTime(*schedule.NextFireAtUnixMs)
		if pending.After(from) {
			appendPreviewFire(preview, pending)
			cursor = pending
		} else {
			decision := decide(schedule, from, misfireThreshold)
			if decision.fire {
				appendPreviewFire(preview, decision.scheduledAt)
			}

			if decision.next == nil || len(preview.NextFiresUnixMs) == count {
				return preview
			}

			appendPreviewFire(preview, *decision.next)
			cursor = *decision.next
		}
	}

	for len(preview.NextFiresUnixMs) < count {
		next, ok := nextFire(schedule, cursor)
		if !ok {
			break
		}

		appendPreviewFire(preview, next)
		cursor = next
	}

	return preview
}

func appendPreviewFire(preview *FiresPreview, fire time.Time) {
	preview.NextFiresUnixMs = append(preview.NextFiresUnixMs, fire.UnixMilli())
}
