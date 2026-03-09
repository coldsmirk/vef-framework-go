package dispatcher

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Relay polls the event outbox table and dispatches pending events.
type Relay struct {
	db         orm.DB
	dispatcher approval.EventDispatcher
	cfg        *config.ApprovalConfig
}

// NewRelay creates a new Relay.
func NewRelay(db orm.DB, dispatcher approval.EventDispatcher, cfg *config.ApprovalConfig) *Relay {
	return &Relay{db: db, dispatcher: dispatcher, cfg: cfg}
}

// RelayPending polls pending and retryable events from the outbox and dispatches them.
func (r *Relay) RelayPending(ctx context.Context) {
	batchSize := r.cfg.OutboxBatchSizeOrDefault()
	maxRetries := r.cfg.OutboxMaxRetriesOrDefault()
	now := timex.Now()

	var claimed []approval.EventOutbox

	if err := r.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var records []approval.EventOutbox

		if err := tx.NewSelect().Model(&records).
			Where(func(cb orm.ConditionBuilder) {
				cb.Group(func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(approval.EventOutboxPending)).
						IsNull("retry_after")
				}).OrGroup(func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(approval.EventOutboxFailed)).
						LessThan("retry_count", maxRetries).
						LessThanOrEqual("retry_after", now)
				}).OrGroup(func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(approval.EventOutboxProcessing)).
						LessThan("retry_count", maxRetries).
						LessThanOrEqual("retry_after", now)
				})
			}).
			OrderBy("created_at").
			Limit(batchSize).
			ForUpdateSkipLocked().
			Scan(ctx); err != nil {
			return fmt.Errorf("poll outbox events: %w", err)
		}

		if len(records) == 0 {
			return nil
		}

		logger.Infof("Relaying %d outbox events", len(records))

		var err error

		claimed, err = r.claimRecords(ctx, tx, records, now, r.claimLeaseDeadline(now))

		return err
	}); err != nil {
		logger.Errorf("Failed to poll and claim outbox events: %v", err)

		return
	}

	if len(claimed) == 0 {
		return
	}

	for i := range claimed {
		if err := r.dispatchOne(ctx, &claimed[i]); err != nil {
			logger.Errorf("Failed to dispatch event %s: %v", claimed[i].EventID, err)
		}
	}
}

func (*Relay) claimRecords(
	ctx context.Context,
	db orm.DB,
	records []approval.EventOutbox,
	now timex.DateTime,
	leaseUntil timex.DateTime,
) ([]approval.EventOutbox, error) {
	claimed := make([]approval.EventOutbox, 0, len(records))

	for _, record := range records {
		isPending := record.Status == approval.EventOutboxPending
		isFailed := record.Status == approval.EventOutboxFailed

		isProcessing := record.Status == approval.EventOutboxProcessing
		if !isPending && !isFailed && !isProcessing {
			continue
		}

		update := db.NewUpdate().
			Model((*approval.EventOutbox)(nil)).
			Set("status", approval.EventOutboxProcessing).
			Set("retry_after", leaseUntil).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(record.ID).
					ApplyIf(isPending, func(cb orm.ConditionBuilder) {
						cb.Equals("status", approval.EventOutboxPending).
							IsNull("retry_after")
					}).
					ApplyIf(isFailed, func(cb orm.ConditionBuilder) {
						cb.Equals("status", approval.EventOutboxFailed).
							LessThanOrEqual("retry_after", now)
					}).
					ApplyIf(isProcessing, func(cb orm.ConditionBuilder) {
						cb.Equals("status", approval.EventOutboxProcessing).
							LessThanOrEqual("retry_after", now)
					})
			})

		result, err := update.Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("claim outbox event %s: %w", record.ID, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("claim outbox event %s affected rows: %w", record.ID, err)
		}

		if affected == 0 {
			continue
		}

		record.Status = approval.EventOutboxProcessing
		record.RetryAfter = &leaseUntil
		claimed = append(claimed, record)
	}

	return claimed, nil
}

func (r *Relay) claimLeaseDeadline(now timex.DateTime) timex.DateTime {
	leaseSeconds := max(r.cfg.OutboxRelayIntervalOrDefault()*4, 15)

	return now.Add(time.Duration(leaseSeconds) * time.Second)
}

// dispatchOne dispatches a single outbox record and updates its status.
func (r *Relay) dispatchOne(ctx context.Context, record *approval.EventOutbox) error {
	now := timex.Now()

	if err := r.dispatcher.Dispatch(ctx, *record); err != nil {
		logger.Errorf("Dispatch failed for event %s: %v", record.EventID, err)

		return r.markFailed(ctx, record, err, now)
	}

	return r.markCompleted(ctx, record, now)
}

// markCompleted marks an outbox record as completed.
func (r *Relay) markCompleted(ctx context.Context, record *approval.EventOutbox, now timex.DateTime) error {
	record.Status = approval.EventOutboxCompleted
	record.ProcessedAt = &now
	record.RetryAfter = nil
	record.LastError = nil

	_, err := r.db.NewUpdate().
		Model(record).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(record.ID).
				Equals("status", approval.EventOutboxProcessing)
		}).
		Select("status", "processed_at", "retry_after", "last_error").
		Exec(ctx)

	return err
}

// markFailed marks an outbox record as failed with exponential backoff retry scheduling.
func (r *Relay) markFailed(ctx context.Context, record *approval.EventOutbox, dispatchErr error, now timex.DateTime) error {
	record.Status = approval.EventOutboxFailed
	record.RetryCount++
	record.LastError = new(dispatchErr.Error())

	maxRetries := r.cfg.OutboxMaxRetriesOrDefault()
	if record.RetryCount < maxRetries {
		backoff := time.Duration(math.Pow(2, float64(record.RetryCount))) * time.Second
		record.RetryAfter = new(now.Add(backoff))
	} else {
		record.RetryAfter = nil
	}

	_, err := r.db.NewUpdate().
		Model(record).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(record.ID).
				Equals("status", approval.EventOutboxProcessing)
		}).
		Select("status", "retry_count", "last_error", "retry_after").
		Exec(ctx)

	return err
}
