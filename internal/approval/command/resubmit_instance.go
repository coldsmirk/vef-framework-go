package command

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ResubmitInstanceCmd resubmits a returned instance.
type ResubmitInstanceCmd struct {
	cqrs.BaseCommand
	approval.ResubmitInstanceInput
}

// ResubmitInstanceHandler handles the ResubmitInstanceCmd command.
type ResubmitInstanceHandler struct {
	business      *business.Registry
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
	formStorage *storage.Dispatcher, registry *business.Registry,
) *ResubmitInstanceHandler {
	return &ResubmitInstanceHandler{business: registry, db: db, engine: eng, validationSvc: validationSvc, instanceSvc: instanceSvc, formStorage: formStorage}
}

func (h *ResubmitInstanceHandler) Handle(ctx context.Context, cmd ResubmitInstanceCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var (
		key   *submissionKey
		fresh bool
	)
	if cmd.IdempotencyKey != "" {
		tenant, err := cmd.Caller.TenantScopeFilter("")
		if err != nil {
			return cqrs.Unit{}, approval.ErrInstanceNotFound
		}

		var submission rpcSubmission

		ctx, submission = takeRPCSubmission(ctx, cmd.FormData)

		key, err = newSubmissionKey(cmd.IdempotencyKey, "resubmit", submissionRequest{ApplicantID: cmd.Operator.ID, InstanceID: cmd.InstanceID, FormData: submission.form})
		if err != nil {
			return cqrs.Unit{}, err
		}

		fresh, err = key.reserve(ctx, db, tenant, cmd.InstanceID)
		if err != nil {
			return cqrs.Unit{}, err
		}
	}

	instance, err := h.instanceSvc.LoadForUpdate(ctx, db, cmd.InstanceID, cmd.Caller)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if key != nil {
		key.TenantID = instance.TenantID
		if !fresh {
			_, err := key.replay(ctx, db)

			return cqrs.Unit{}, err
		}
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
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load flow version: %w", err)
	}

	if instance.BusinessKind != nil {
		flow := &approval.Flow{ID: instance.FlowID}
		if err := db.NewSelect().Model(flow).WherePK().Scan(ctx); err != nil {
			return cqrs.Unit{}, err
		}

		sc := &approval.SubmissionContext{
			InstanceID:  instance.ID,
			Instance:    instance,
			Flow:        flow,
			Version:     &version,
			TenantID:    instance.TenantID,
			Applicant:   instance.Applicant(),
			BusinessRef: instance.BusinessRef,
			FormData:    cmd.FormData,
		}

		submission, err := h.business.Submit(ctx, db, sc)
		if err != nil {
			return cqrs.Unit{}, err
		}

		instance.FormData = submission.FormData
	} else {
		if instance.FormData == nil {
			instance.FormData = make(map[string]any, len(cmd.FormData))
		}

		if len(cmd.FormData) > 0 {
			maps.Copy(instance.FormData, cmd.FormData)
		}
	}

	if err := h.validationSvc.ValidateFormData(ctx, version.FormFields, instance.FormData); err != nil {
		return cqrs.Unit{}, err
	}

	if err := h.instanceSvc.PublishDrafts(ctx, db, instance, nil, cmd.Operator); err != nil {
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

	if err := h.engine.EnterInstance(ctx, db, h.business, instance, &version, true); err != nil {
		return cqrs.Unit{}, fmt.Errorf("start process on resubmit: %w", err)
	}

	actionLog := cmd.Operator.NewActionLog(cmd.InstanceID, approval.ActionResubmit)
	behavior.ActionLogCollectorFromContext(ctx).Add(actionLog)

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewInstanceResubmittedEvent(instance, cmd.Operator),
	)

	if key != nil {
		return cqrs.Unit{}, key.link(ctx, db, instance.ID)
	}

	return cqrs.Unit{}, nil
}
