package binding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

var logger = logx.Named("approval:binding")

// Worker leases eventual projections from durable state and applies their
// latest desired revision. Lifecycle events are notifications only; worker
// correctness depends solely on the projection table.
type Worker struct {
	db     orm.DB
	bus    event.Bus
	writer *Writer
	config *config.ApprovalConfig
}

// NewWorker constructs the eventual projection worker.
func NewWorker(db orm.DB, bus event.Bus, writer *Writer, cfg *config.ApprovalConfig) *Worker {
	if cfg == nil {
		cfg = new(config.ApprovalConfig)
	}

	return &Worker{db: db, bus: bus, writer: writer, config: cfg}
}

// Run processes one configured batch. Failures are persisted and scheduled for
// retry; they never escape into an approval transaction.
func (w *Worker) Run(ctx context.Context) {
	processed, err := w.ProcessPending(ctx)
	if err != nil {
		logger.Errorf("process pending business projections: %v", err)

		return
	}

	if processed > 0 {
		logger.Infof("Processed %d business projection(s)", processed)
	}
}

// ProcessPending claims and processes at most the configured batch size.
func (w *Worker) ProcessPending(ctx context.Context) (int, error) {
	claimed, err := w.claimBatch(ctx)
	if err != nil {
		return 0, err
	}

	for i := range claimed {
		if err := w.applyClaim(ctx, &claimed[i]); err != nil {
			return i, err
		}
	}

	return len(claimed), nil
}

// Retry applies one locked eventual projection immediately. A business write
// failure is recorded on the projection and is not returned as an
// infrastructure error; the caller can inspect Status and LastError.
func (w *Worker) Retry(ctx context.Context, db orm.DB, projection *approval.BusinessProjection) error {
	if projection.Consistency != config.ApprovalBindingEventual {
		return fmt.Errorf("%w: projection %q is %q", ErrProjectionStateInvalid,
			projection.ID, projection.Consistency)
	}

	return db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		projection.Status = approval.BindingProjectionProcessing
		projection.NextAttemptAt = nil
		projection.LeaseUntil = nil

		if err := w.writeProjection(ctx, tx, projection); err != nil {
			return markProjectionFailed(ctx, tx, projection, err)
		}

		return markProjectionApplied(ctx, tx, projection)
	})
}

func (w *Worker) claimBatch(ctx context.Context) ([]approval.BusinessProjection, error) {
	var claimed []approval.BusinessProjection

	err := w.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		now := timex.Now()
		leaseUntil := now.Add(max(30*time.Second, 3*w.config.BusinessBinding.EffectiveScanInterval()))

		if err := tx.NewSelect().
			Model(&claimed).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("consistency", config.ApprovalBindingEventual).
					GreaterThanColumn("desired_revision", "applied_revision").
					Group(func(cb orm.ConditionBuilder) {
						cb.Group(func(cb orm.ConditionBuilder) {
							cb.In("status", []approval.BindingProjectionStatus{
								approval.BindingProjectionPending,
								approval.BindingProjectionFailed,
							}).
								Group(func(cb orm.ConditionBuilder) {
									cb.IsNull("next_attempt_at").OrLessThanOrEqual("next_attempt_at", now)
								})
						}).OrGroup(func(cb orm.ConditionBuilder) {
							cb.Equals("status", approval.BindingProjectionProcessing).
								Group(func(cb orm.ConditionBuilder) {
									cb.IsNull("lease_until").OrLessThanOrEqual("lease_until", now)
								})
						})
					})
			}).
			OrderByExpr(func(eb orm.ExprBuilder) any {
				return eb.Order(func(ob orm.OrderBuilder) {
					ob.Expr(eb.Coalesce(eb.Column("next_attempt_at"), eb.Column("created_at"))).Asc()
				})
			}).
			OrderBy("created_at", "id").
			Limit(w.config.BusinessBinding.EffectiveBatchSize()).
			ForUpdateSkipLocked().
			Scan(ctx); err != nil {
			return fmt.Errorf("claim business projections: %w", err)
		}

		if len(claimed) == 0 {
			return nil
		}

		ids := make([]string, len(claimed))
		for i := range claimed {
			ids[i] = claimed[i].ID
		}

		if _, err := tx.NewUpdate().
			Model((*approval.BusinessProjection)(nil)).
			Set("status", approval.BindingProjectionProcessing).
			Set("lease_until", leaseUntil).
			Set("updated_at", now).
			Where(func(cb orm.ConditionBuilder) { cb.PKIn(ids) }).
			Exec(ctx); err != nil {
			return fmt.Errorf("lease business projections: %w", err)
		}

		for i := range claimed {
			claimed[i].Status = approval.BindingProjectionProcessing
			lease := leaseUntil
			claimed[i].LeaseUntil = &lease
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

func (w *Worker) applyClaim(ctx context.Context, claimed *approval.BusinessProjection) error {
	var (
		writeErr   error
		instance   approval.Instance
		failed     *approval.BusinessProjection
		ownerFound bool
	)

	err := w.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		current := new(approval.BusinessProjection)

		current.ID = claimed.ID
		if err := tx.NewSelect().Model(current).WherePK().ForUpdate().Scan(ctx); err != nil {
			if result.IsRecordNotFound(err) {
				return nil
			}

			return fmt.Errorf("reload claimed business projection %q: %w", claimed.ID, err)
		}

		if current.Status != approval.BindingProjectionProcessing ||
			current.OwnerInstanceID != claimed.OwnerInstanceID ||
			current.DesiredRevision != claimed.DesiredRevision {
			return nil
		}

		if err := w.writeProjection(ctx, tx, current); err != nil {
			writeErr = err
			if err := markProjectionFailed(ctx, tx, current, err); err != nil {
				return err
			}

			failed = current

			instance.ID = current.OwnerInstanceID
			if err := tx.NewSelect().Model(&instance).WherePK().Scan(ctx); err != nil {
				if !result.IsRecordNotFound(err) {
					return fmt.Errorf("load failed projection owner %q: %w", current.OwnerInstanceID, err)
				}
			} else {
				ownerFound = true
			}

			return nil
		}

		return markProjectionApplied(ctx, tx, current)
	})
	if err != nil {
		return err
	}

	if writeErr != nil {
		logger.Errorf("Business projection %s revision %d failed: %v", claimed.ID, claimed.DesiredRevision, writeErr)

		if failed != nil && ownerFound {
			businessTable := ""
			if failed.Binding != nil {
				businessTable = failed.Binding.TableName
			}

			failureEvent := approval.NewInstanceBindingFailedEvent(
				&instance,
				projectionTrigger(failed),
				failed.DesiredStatus,
				businessTable,
				projectionError(writeErr),
			)
			if err := w.publishFailure(ctx, failureEvent); err != nil {
				logger.Errorf("publish binding failure event for instance %s: %v", instance.ID, err)
			}
		}
	}

	return nil
}

// writeProjection isolates host-table SQL in a savepoint. PostgreSQL aborts a
// transaction after a statement error; rolling back the savepoint keeps the
// outer transaction usable so the durable failed state can still be recorded.
func (w *Worker) writeProjection(
	ctx context.Context,
	db orm.DB,
	projection *approval.BusinessProjection,
) error {
	return db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		return w.writer.Write(ctx, tx, projection)
	})
}

func markProjectionFailed(ctx context.Context, db orm.DB, projection *approval.BusinessProjection, cause error) error {
	projection.AttemptCount++
	now := timex.Now()
	nextAttempt := now.Add(projectionBackoff(projection.AttemptCount))
	errorMessage := projectionError(cause)
	projection.Status = approval.BindingProjectionFailed
	projection.NextAttemptAt = &nextAttempt
	projection.LeaseUntil = nil
	projection.LastError = &errorMessage
	projection.UpdatedAt = now

	if _, err := db.NewUpdate().
		Model(projection).
		Select("status", "attempt_count", "next_attempt_at", "lease_until", "last_error", "updated_at").
		WherePK().
		Exec(ctx); err != nil {
		return fmt.Errorf("mark business projection %q failed: %w", projection.ID, err)
	}

	return nil
}

func projectionBackoff(attempt int) time.Duration {
	shift := min(max(attempt, 0), 12)

	return min(time.Second<<shift, time.Hour)
}

func projectionError(err error) string {
	const maxErrorBytes = 512

	message := strings.TrimSpace(strings.ToValidUTF8(err.Error(), ""))
	if len(message) > maxErrorBytes {
		message = strings.ToValidUTF8(message[:maxErrorBytes], "")
	}

	return message
}

func projectionTrigger(projection *approval.BusinessProjection) approval.BindingTrigger {
	switch projection.DesiredStatus {
	case approval.InstanceReturned:
		return approval.BindingTriggerReturned
	case approval.InstanceWithdrawn:
		return approval.BindingTriggerWithdrawn
	case approval.InstanceApproved, approval.InstanceRejected, approval.InstanceTerminated:
		return approval.BindingTriggerCompleted
	default:
		if projection.AppliedOwnerInstanceID == nil || *projection.AppliedOwnerInstanceID != projection.OwnerInstanceID {
			return approval.BindingTriggerStarted
		}

		return approval.BindingTriggerResubmitted
	}
}

func (w *Worker) publishFailure(ctx context.Context, failureEvent approval.DomainEvent) error {
	opts := []event.PublishOption{}
	if t := approval.PayloadOccurredAt(failureEvent); !t.IsZero() {
		opts = append(opts, event.WithOccurredAt(t.Unwrap()))
	}

	if w.db == nil {
		return w.bus.Publish(ctx, failureEvent, opts...)
	}

	err := w.db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
		return w.bus.Publish(ctx, failureEvent, append(opts, event.WithTx(tx))...)
	})
	if err == nil {
		return nil
	}

	if errors.Is(err, event.ErrTxRequired) {
		return w.bus.Publish(ctx, failureEvent, opts...)
	}

	return err
}
