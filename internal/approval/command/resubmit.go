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
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ResubmitCmd resubmits a returned instance.
type ResubmitCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.OperatorInfo
	FormData   map[string]any
	Caller     approval.CallerContext
}

// ResubmitHandler handles the ResubmitCmd command.
type ResubmitHandler struct {
	db            orm.DB
	engine        *engine.FlowEngine
	validationSvc *service.ValidationService
	instanceSvc   *service.InstanceService
}

// NewResubmitHandler creates a new ResubmitHandler.
func NewResubmitHandler(
	db orm.DB,
	eng *engine.FlowEngine,
	validationSvc *service.ValidationService,
	instanceSvc *service.InstanceService,
) *ResubmitHandler {
	return &ResubmitHandler{db: db, engine: eng, validationSvc: validationSvc, instanceSvc: instanceSvc}
}

func (h *ResubmitHandler) Handle(ctx context.Context, cmd ResubmitCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	instance, err := h.instanceSvc.LoadForUpdate(ctx, db, cmd.InstanceID, cmd.Caller)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if instance.ApplicantID != cmd.Operator.ID {
		return cqrs.Unit{}, shared.ErrNotApplicant
	}

	if !engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceRunning) {
		return cqrs.Unit{}, shared.ErrResubmitNotAllowed
	}

	var version approval.FlowVersion

	version.ID = instance.FlowVersionID
	if err := db.NewSelect().
		Model(&version).
		Select("form_schema").
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

	if err := h.validationSvc.ValidateFormData(version.FormSchema, instance.FormData); err != nil {
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
		if errors.Is(err, shared.ErrInvalidInstanceTransition) {
			return cqrs.Unit{}, shared.ErrResubmitNotAllowed
		}

		return cqrs.Unit{}, err
	}

	if err := h.engine.StartProcess(ctx, db, instance); err != nil {
		return cqrs.Unit{}, fmt.Errorf("start process on resubmit: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionResubmit)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewInstanceResubmittedEvent(cmd.InstanceID, instance.TenantID, cmd.Operator.ID),
	)

	return cqrs.Unit{}, nil
}
