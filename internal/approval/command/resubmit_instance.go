package command

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ResubmitInstanceCmd resubmits a returned instance.
type ResubmitInstanceCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.UserInfo
	FormData   map[string]any
	Caller     approval.CallerContext
}

// ResubmitInstanceHandler handles the ResubmitInstanceCmd command.
type ResubmitInstanceHandler struct {
	db            orm.DB
	engine        *engine.FlowEngine
	validationSvc *service.ValidationService
	instanceSvc   *service.InstanceService
	formStorage   *storage.Dispatcher
}

// NewResubmitInstanceHandler creates a new ResubmitInstanceHandler. formStorage refreshes the
// version's physical projection row — replacing the instance's prior row, never
// appending — when its StorageMode is StorageTable; it may be nil in test
// fixtures that do not exercise table-mode storage.
func NewResubmitInstanceHandler(
	db orm.DB,
	eng *engine.FlowEngine,
	validationSvc *service.ValidationService,
	instanceSvc *service.InstanceService,
	formStorage *storage.Dispatcher,
) *ResubmitInstanceHandler {
	return &ResubmitInstanceHandler{db: db, engine: eng, validationSvc: validationSvc, instanceSvc: instanceSvc, formStorage: formStorage}
}

func (h *ResubmitInstanceHandler) Handle(ctx context.Context, cmd ResubmitInstanceCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	instance, err := h.instanceSvc.LoadForUpdate(ctx, db, cmd.InstanceID, cmd.Caller)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if instance.ApplicantID != cmd.Operator.ID {
		return cqrs.Unit{}, approval.ErrNotApplicant
	}

	if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceRunning) {
		return cqrs.Unit{}, approval.ErrResubmitNotAllowed
	}

	var version approval.FlowVersion

	version.ID = instance.FlowVersionID
	if err := db.NewSelect().
		Model(&version).
		Select("form_fields", "storage_mode").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load flow version: %w", err)
	}

	if instance.FormData == nil {
		instance.FormData = make(map[string]any, len(cmd.FormData))
	}

	if len(cmd.FormData) > 0 {
		maps.Copy(instance.FormData, cmd.FormData)
	}

	if err := h.validationSvc.ValidateFormData(version.FormFields, instance.FormData); err != nil {
		return cqrs.Unit{}, err
	}

	// State machine transition: returned|withdrawn -> running. The same
	// UPDATE persists the merged form data and clears finished_at; the
	// engine handles any further status changes during StartProcess
	// (e.g. straight-to-end shortcuts) through ApplyInstanceTransition.
	instance.FinishedAt = nil
	if err := h.instanceSvc.Transition(
		ctx, db, instance, approval.InstanceRunning,
		"form_data", "finished_at",
	); err != nil {
		if errors.Is(err, approval.ErrInvalidInstanceTransition) {
			return cqrs.Unit{}, approval.ErrResubmitNotAllowed
		}

		return cqrs.Unit{}, err
	}

	// Refresh the table-mode projection so the physical table reflects the
	// resubmitted form data. SyncInstanceProjection replaces the instance's
	// existing row (idempotent per instance), so a resubmit refreshes rather
	// than duplicates; the merged form_data is already persisted above, so JSON
	// mode is a no-op.
	if h.formStorage != nil {
		if err := h.formStorage.SyncInstanceProjection(ctx, db, instance); err != nil {
			return cqrs.Unit{}, fmt.Errorf("sync form projection on resubmit: %w", err)
		}
	}

	if err := h.engine.StartProcess(ctx, db, instance); err != nil {
		return cqrs.Unit{}, fmt.Errorf("start process on resubmit: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionResubmit)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewInstanceResubmittedEvent(instance, cmd.Operator),
	)

	return cqrs.Unit{}, nil
}
