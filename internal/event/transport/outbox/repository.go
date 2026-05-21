package outbox

import (
	"context"
	"fmt"

	puboutbox "github.com/coldsmirk/vef-framework-go/event/transport/outbox"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// DefaultRepository is the orm.DB-backed Repository used by the outbox
// transport. It claims records under FOR UPDATE SKIP LOCKED so multiple
// relay workers (across processes) can poll the same table safely.
type DefaultRepository struct {
	db orm.DB
}

// NewRepository constructs a DefaultRepository.
func NewRepository(db orm.DB) *DefaultRepository {
	return &DefaultRepository{db: db}
}

// InsertBatch persists pending records on the outer (non-transactional)
// connection. Callers that need the records to share a transaction
// with surrounding business writes must use InsertBatchTx instead.
func (r *DefaultRepository) InsertBatch(ctx context.Context, records []puboutbox.Record) error {
	if len(records) == 0 {
		return nil
	}

	return insertBatchUsing(ctx, r.db, records)
}

// InsertBatchTx persists pending records within the caller's transaction.
func (*DefaultRepository) InsertBatchTx(ctx context.Context, tx orm.DB, records []puboutbox.Record) error {
	if len(records) == 0 {
		return nil
	}

	return insertBatchUsing(ctx, tx, records)
}

func insertBatchUsing(ctx context.Context, db orm.DB, records []puboutbox.Record) error {
	_, err := db.NewInsert().Model(&records).Exec(ctx)
	if err != nil {
		return fmt.Errorf("outbox: insert batch: %w", err)
	}

	return nil
}

// ClaimBatch atomically transitions a batch of pending or retry-eligible
// records to processing under FOR UPDATE SKIP LOCKED, returning the
// claimed rows. The lease deadline is written to retry_after so a stuck
// worker's records can be picked up by another worker after expiry.
func (r *DefaultRepository) ClaimBatch(
	ctx context.Context,
	batchSize int,
	maxRetries int,
	leaseUntil timex.DateTime,
) ([]puboutbox.Record, error) {
	var claimed []puboutbox.Record

	err := r.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		now := timex.Now()

		var records []puboutbox.Record
		if err := tx.NewSelect().Model(&records).
			Where(func(cb orm.ConditionBuilder) {
				cb.Group(func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(puboutbox.StatusPending)).IsNull("retry_after")
				}).OrGroup(func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(puboutbox.StatusFailed)).
						LessThan("retry_count", maxRetries).
						LessThanOrEqual("retry_after", now)
				}).OrGroup(func(cb orm.ConditionBuilder) {
					cb.Equals("status", string(puboutbox.StatusProcessing)).
						LessThan("retry_count", maxRetries).
						LessThanOrEqual("retry_after", now)
				})
			}).
			OrderBy("created_at").
			Limit(batchSize).
			ForUpdateSkipLocked().
			Scan(ctx); err != nil {
			return fmt.Errorf("outbox: poll: %w", err)
		}

		if len(records) == 0 {
			return nil
		}

		claimed = make([]puboutbox.Record, 0, len(records))
		for _, rec := range records {
			res, err := tx.NewUpdate().
				Model((*puboutbox.Record)(nil)).
				Set("status", string(puboutbox.StatusProcessing)).
				Set("retry_after", leaseUntil).
				Where(func(cb orm.ConditionBuilder) {
					cb.PKEquals(rec.ID)

					switch rec.Status {
					case puboutbox.StatusPending:
						cb.Equals("status", string(puboutbox.StatusPending)).IsNull("retry_after")
					case puboutbox.StatusFailed:
						cb.Equals("status", string(puboutbox.StatusFailed)).LessThanOrEqual("retry_after", now)
					case puboutbox.StatusProcessing:
						cb.Equals("status", string(puboutbox.StatusProcessing)).LessThanOrEqual("retry_after", now)
					}
				}).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("outbox: claim %s: %w", rec.ID, err)
			}

			affected, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("outbox: rows affected for %s: %w", rec.ID, err)
			}

			if affected == 0 {
				continue
			}

			rec.Status = puboutbox.StatusProcessing
			leaseCopy := leaseUntil
			rec.RetryAfter = &leaseCopy
			claimed = append(claimed, rec)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

// MarkCompleted transitions a processing record to completed.
func (r *DefaultRepository) MarkCompleted(ctx context.Context, id string) error {
	now := timex.Now()
	record := &puboutbox.Record{
		Status:      puboutbox.StatusCompleted,
		ProcessedAt: &now,
		RetryAfter:  nil,
		LastError:   nil,
	}
	record.ID = id

	_, err := r.db.NewUpdate().
		Model(record).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(id).Equals("status", string(puboutbox.StatusProcessing))
		}).
		Select("status", "processed_at", "retry_after", "last_error").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("outbox: mark completed %s: %w", id, err)
	}

	return nil
}

// MarkFailed transitions a processing record to failed (retry scheduled)
// or dead (retry budget exhausted). retryCount is the *new* counter
// value, i.e. one past whatever the relay observed.
func (r *DefaultRepository) MarkFailed(
	ctx context.Context,
	id string,
	errMsg string,
	retryCount int,
	retryAfter timex.DateTime,
	maxRetries int,
) error {
	errCopy := errMsg
	record := &puboutbox.Record{
		RetryCount: retryCount,
		LastError:  &errCopy,
	}
	record.ID = id

	columns := []string{"status", "retry_count", "last_error", "retry_after"}

	if retryCount >= maxRetries {
		now := timex.Now()
		record.Status = puboutbox.StatusDead
		record.RetryAfter = nil
		record.ProcessedAt = &now

		columns = append(columns, "processed_at")
	} else {
		record.Status = puboutbox.StatusFailed
		ra := retryAfter
		record.RetryAfter = &ra
	}

	_, err := r.db.NewUpdate().
		Model(record).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(id).Equals("status", string(puboutbox.StatusProcessing))
		}).
		Select(columns...).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("outbox: mark failed %s: %w", id, err)
	}

	return nil
}

// DeleteCompletedOlderThan removes completed rows whose processed_at is
// strictly before cutoff. Dead rows are kept for diagnostics regardless.
func (r *DefaultRepository) DeleteCompletedOlderThan(ctx context.Context, cutoff timex.DateTime) (int64, error) {
	res, err := r.db.NewDelete().
		Model((*puboutbox.Record)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("status", string(puboutbox.StatusCompleted)).LessThan("processed_at", cutoff)
		}).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("outbox: cleanup: %w", err)
	}

	return res.RowsAffected()
}
