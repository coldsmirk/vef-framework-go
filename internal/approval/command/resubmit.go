package command

import (
	"context"
	"fmt"
	"maps"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// ResubmitCmd resubmits a returned instance.
type ResubmitCmd struct {
	cqrs.BaseCommand

	InstanceID string
	Operator   approval.OperatorInfo
	FormData   map[string]any
}

// ResubmitHandler handles the ResubmitCmd command.
type ResubmitHandler struct {
	db            orm.DB
	engine        *engine.FlowEngine
	validationSvc *service.ValidationService
	publisher     *dispatcher.EventPublisher
}

// NewResubmitHandler creates a new ResubmitHandler.
func NewResubmitHandler(
	db orm.DB,
	eng *engine.FlowEngine,
	validationSvc *service.ValidationService,
	publisher *dispatcher.EventPublisher,
) *ResubmitHandler {
	return &ResubmitHandler{db: db, engine: eng, validationSvc: validationSvc, publisher: publisher}
}

func (h *ResubmitHandler) Handle(ctx context.Context, cmd ResubmitCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var instance approval.Instance

	instance.ID = cmd.InstanceID

	if err := db.NewSelect().
		Model(&instance).
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrInstanceNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load instance: %w", err)
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

	mergedFormData := maps.Clone(instance.FormData)
	if mergedFormData == nil {
		mergedFormData = make(map[string]any, len(cmd.FormData))
	}

	if len(cmd.FormData) > 0 {
		maps.Copy(mergedFormData, cmd.FormData)
	}

	if err := h.validationSvc.ValidateFormData(version.FormSchema, mergedFormData); err != nil {
		return cqrs.Unit{}, err
	}

	if len(cmd.FormData) > 0 {
		if instance.FormData == nil {
			instance.FormData = make(map[string]any, len(cmd.FormData))
		}

		maps.Copy(instance.FormData, cmd.FormData)
	}

	instance.Status = approval.InstanceRunning
	instance.FinishedAt = nil

	if err := h.engine.StartProcess(ctx, db, &instance); err != nil {
		return cqrs.Unit{}, fmt.Errorf("start process on resubmit: %w", err)
	}

	if _, err := db.NewUpdate().
		Model(&instance).
		Select("form_data", "status", "current_node_id", "finished_at").
		WherePK().
		Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("update instance: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionResubmit)
	if _, err := db.NewInsert().Model(actionLog).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert action log: %w", err)
	}

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{
		approval.NewInstanceResubmittedEvent(cmd.InstanceID, cmd.Operator.ID),
	}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
