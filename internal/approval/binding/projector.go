package binding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Projector owns business-target claims and advances their durable desired
// state. Synchronous projections are applied before the caller's transaction
// can commit; eventual projections are left pending for Worker.
type Projector struct {
	resolver approval.BusinessRefResolver
	writer   *Writer
	config   *config.ApprovalConfig
}

// NewProjector constructs the business-state projector.
func NewProjector(
	resolver approval.BusinessRefResolver,
	writer *Writer,
	cfg *config.ApprovalConfig,
) *Projector {
	if cfg == nil {
		cfg = new(config.ApprovalConfig)
	}

	return &Projector{resolver: resolver, writer: writer, config: cfg}
}

// Bind resolves and claims a version-pinned business target for a newly
// inserted instance. A non-final existing owner blocks the claim; a final owner
// can be superseded, with the applied owner retained as the business-table CAS
// fence until the new desired state is written.
func (p *Projector) Bind(
	ctx context.Context,
	db orm.DB,
	flow *approval.Flow,
	version *approval.FlowVersion,
	instance *approval.Instance,
) error {
	if version == nil || version.BusinessBinding == nil {
		return nil
	}

	binding, err := NormalizeConfig(approval.BindingBusiness, version.BusinessBinding)
	if err != nil {
		return fmt.Errorf("%w: flow version %q: %w", ErrBindingMisconfigured, version.ID, err)
	}

	if instance.BusinessRef == nil || strings.TrimSpace(*instance.BusinessRef) == "" {
		return approval.ErrBusinessRefRequired
	}

	resolvedFlow := *flow
	resolvedFlow.BindingMode = approval.BindingBusiness
	resolvedFlow.BusinessBinding = binding

	recordKey, err := p.resolver.ResolveRecordKey(ctx, &resolvedFlow, *instance.BusinessRef)
	if err != nil {
		if errors.Is(err, ErrInvalidBusinessRef) {
			return approval.ErrInvalidBusinessRef
		}

		return fmt.Errorf("resolve business ref for flow version %q: %w", version.ID, err)
	}

	encodedKey, err := encodeRecordKey(binding, recordKey)
	if err != nil {
		if errors.Is(err, ErrInvalidBusinessRef) {
			return approval.ErrInvalidBusinessRef
		}

		return err
	}

	targetHash := hashx.SHA256(binding.TableName + "\x00" + string(encodedKey))
	consistency := p.config.BusinessBinding.EffectiveConsistency()

	projection := &approval.BusinessProjection{
		TenantID:        instance.TenantID,
		FlowID:          instance.FlowID,
		FlowVersionID:   instance.FlowVersionID,
		OwnerInstanceID: instance.ID,
		TargetHash:      targetHash,
		Consistency:     consistency,
		Binding:         binding,
		RecordKey:       encodedKey,
		DesiredRevision: 1,
		Status:          approval.BindingProjectionPending,
	}
	if err := setDesiredState(projection, instance); err != nil {
		return err
	}

	if _, err := db.NewInsert().
		Model(projection).
		OnConflict(func(cb orm.ConflictBuilder) {
			cb.Columns("target_hash").DoUpdate().Set("target_hash")
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("insert business projection: %w", err)
	}

	loaded := new(approval.BusinessProjection)
	if err := db.NewSelect().
		Model(loaded).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("target_hash", targetHash) }).
		ForUpdate().
		Scan(ctx); err != nil {
		return fmt.Errorf("load business projection target: %w", err)
	}

	sameRecordKey, err := recordKeysEqual(loaded.RecordKey, encodedKey)
	if err != nil {
		return fmt.Errorf("%w: target hash %q has an invalid stored record key: %w",
			ErrProjectionStateInvalid, targetHash, err)
	}

	if loaded.Binding == nil || loaded.Binding.TableName != binding.TableName || !sameRecordKey {
		return fmt.Errorf("%w: target hash %q resolved to different target data", ErrProjectionStateInvalid, targetHash)
	}

	if loaded.OwnerInstanceID != instance.ID {
		if err := ensureOwnerCanBeSuperseded(ctx, db, loaded.OwnerInstanceID); err != nil {
			return err
		}

		loaded.TenantID = instance.TenantID
		loaded.FlowID = instance.FlowID
		loaded.FlowVersionID = instance.FlowVersionID
		loaded.OwnerInstanceID = instance.ID
		loaded.Consistency = consistency
		loaded.Binding = binding
		loaded.RecordKey = encodedKey

		loaded.DesiredRevision++
		if err := setDesiredState(loaded, instance); err != nil {
			return err
		}
	} else if loaded.DesiredRevision == 0 {
		loaded.DesiredRevision = 1
	}

	resetPendingState(loaded)

	if err := persistDesiredState(ctx, db, loaded); err != nil {
		return err
	}

	projectionID := loaded.ID

	instance.BusinessProjectionID = &projectionID
	if _, err := db.NewUpdate().
		Model(instance).
		Select("business_projection_id").
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("link instance to business projection: %w", err)
	}

	return p.applySynchronouslyIfConfigured(ctx, db, loaded)
}

// Project advances the desired state for an already-bound instance. It is the
// engine transition hook used for running, paused, and final statuses alike.
func (p *Projector) Project(ctx context.Context, db orm.DB, instance *approval.Instance) error {
	if instance.BusinessProjectionID == nil || *instance.BusinessProjectionID == "" {
		return nil
	}

	projection := new(approval.BusinessProjection)

	projection.ID = *instance.BusinessProjectionID
	if err := db.NewSelect().Model(projection).WherePK().ForUpdate().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return fmt.Errorf("%w: projection %q not found", ErrProjectionStateInvalid, projection.ID)
		}

		return fmt.Errorf("load business projection %q: %w", projection.ID, err)
	}

	if projection.OwnerInstanceID != instance.ID {
		return nil
	}

	projection.DesiredRevision++
	if err := setDesiredState(projection, instance); err != nil {
		return err
	}

	resetPendingState(projection)

	if err := persistDesiredState(ctx, db, projection); err != nil {
		return err
	}

	return p.applySynchronouslyIfConfigured(ctx, db, projection)
}

func (p *Projector) applySynchronouslyIfConfigured(ctx context.Context, db orm.DB, projection *approval.BusinessProjection) error {
	switch projection.Consistency {
	case config.ApprovalBindingEventual:
		return nil
	case config.ApprovalBindingSynchronous:
		if err := p.writer.Write(ctx, db, projection); err != nil {
			return fmt.Errorf("apply synchronous business projection: %w", err)
		}

		return markProjectionApplied(ctx, db, projection)

	default:
		return fmt.Errorf("%w: projection %q has consistency %q", ErrProjectionStateInvalid,
			projection.ID, projection.Consistency)
	}
}

func ensureOwnerCanBeSuperseded(ctx context.Context, db orm.DB, ownerInstanceID string) error {
	if ownerInstanceID == "" {
		return nil
	}

	var owner approval.Instance

	owner.ID = ownerInstanceID
	if err := db.NewSelect().Model(&owner).Select("status").WherePK().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil
		}

		return fmt.Errorf("load business projection owner %q: %w", ownerInstanceID, err)
	}

	if !owner.Status.IsFinal() {
		return approval.ErrBindingTargetBusy
	}

	return nil
}

func setDesiredState(projection *approval.BusinessProjection, instance *approval.Instance) error {
	projection.DesiredStatus = instance.Status

	projection.DesiredStartedAt = instance.CreatedAt
	if projection.DesiredStartedAt.IsZero() {
		projection.DesiredStartedAt = timex.Now()
	}

	projection.DesiredFinishedAt = nil
	if instance.Status.IsFinal() {
		if instance.FinishedAt == nil {
			return fmt.Errorf("%w: final instance %q has no finish time", ErrProjectionStateInvalid, instance.ID)
		}

		finishedAt := *instance.FinishedAt
		projection.DesiredFinishedAt = &finishedAt
	}

	return nil
}

func resetPendingState(projection *approval.BusinessProjection) {
	nextAttempt := timex.Now()

	projection.Status = approval.BindingProjectionPending
	projection.AttemptCount = 0
	projection.NextAttemptAt = &nextAttempt
	projection.LeaseUntil = nil
	projection.LastError = nil
}

func persistDesiredState(ctx context.Context, db orm.DB, projection *approval.BusinessProjection) error {
	projection.UpdatedAt = timex.Now()

	if _, err := db.NewUpdate().
		Model(projection).
		Select(
			"tenant_id", "flow_id", "flow_version_id", "owner_instance_id",
			"consistency", "binding", "record_key", "desired_status",
			"desired_started_at", "desired_finished_at", "desired_revision",
			"status", "attempt_count", "next_attempt_at", "lease_until", "last_error", "updated_at",
		).
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("persist desired business projection %q: %w", projection.ID, err)
	}

	return nil
}

func markProjectionApplied(ctx context.Context, db orm.DB, projection *approval.BusinessProjection) error {
	now := timex.Now()
	owner := projection.OwnerInstanceID
	projection.AppliedOwnerInstanceID = &owner
	projection.AppliedRevision = projection.DesiredRevision
	projection.Status = approval.BindingProjectionApplied
	projection.AttemptCount = 0
	projection.NextAttemptAt = nil
	projection.LeaseUntil = nil
	projection.LastError = nil
	projection.AppliedAt = &now
	projection.UpdatedAt = now

	if _, err := db.NewUpdate().
		Model(projection).
		Select(
			"applied_owner_instance_id", "applied_revision", "status",
			"attempt_count", "next_attempt_at", "lease_until", "last_error", "applied_at", "updated_at",
		).
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("mark business projection %q applied: %w", projection.ID, err)
	}

	return nil
}
