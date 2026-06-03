package inbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/event/inbox"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// DefaultRepository is the orm.DB-backed Repository used by the inbox
// middleware. Acquire uses the UNIQUE constraint over
// (consumer_group, event_id) as the portable concurrency boundary.
type DefaultRepository struct {
	db orm.DB
}

// NewRepository constructs a DefaultRepository.
func NewRepository(db orm.DB) *DefaultRepository {
	return &DefaultRepository{db: db}
}

// Acquire claims the (consumerGroup, eventID) slot for processing.
func (r *DefaultRepository) Acquire(
	ctx context.Context,
	consumerGroup string,
	eventID string,
	lockUntil timex.DateTime,
) (inbox.AcquireResult, string, error) {
	lockID := id.Generate()
	record := &inbox.Record{
		EventID:       eventID,
		ConsumerGroup: consumerGroup,
		Status:        inbox.StatusProcessing,
		LockID:        lockID,
		LockedUntil:   &lockUntil,
	}
	record.ID = id.Generate()

	_, err := r.db.NewInsert().Model(record).Exec(ctx)
	if err == nil {
		return inbox.AcquireResultAcquired, lockID, nil
	}
	// The framework ORM translates dialect-specific unique-violation
	// codes into result.ErrRecordAlreadyExists, which is what every
	// supported backend funnels into.
	if !errors.Is(err, result.ErrRecordAlreadyExists) {
		return "", "", fmt.Errorf("inbox: acquire (%s, %s): %w", consumerGroup, eventID, err)
	}

	var existing inbox.Record
	if err := r.db.NewSelect().Model(&existing).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("consumer_group", consumerGroup).Equals("event_id", eventID)
		}).
		Scan(ctx); err != nil {
		return "", "", fmt.Errorf("inbox: load existing (%s, %s): %w", consumerGroup, eventID, err)
	}

	if existing.Status == inbox.StatusCompleted {
		return inbox.AcquireResultCompleted, "", nil
	}

	now := timex.Now()

	res, err := r.db.NewUpdate().
		Model((*inbox.Record)(nil)).
		Set("status", inbox.StatusProcessing).
		Set("lock_id", lockID).
		Set("locked_until", lockUntil).
		Set("completed_at", nil).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("consumer_group", consumerGroup).
				Equals("event_id", eventID).
				Equals("status", string(inbox.StatusProcessing)).
				Group(func(cb orm.ConditionBuilder) {
					cb.IsNull("locked_until").OrLessThanOrEqual("locked_until", now)
				})
		}).
		Exec(ctx)
	if err != nil {
		return "", "", fmt.Errorf("inbox: reacquire (%s, %s): %w", consumerGroup, eventID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return "", "", fmt.Errorf("inbox: reacquire rows affected (%s, %s): %w", consumerGroup, eventID, err)
	}

	if affected == 0 {
		return inbox.AcquireResultInProgress, "", nil
	}

	return inbox.AcquireResultAcquired, lockID, nil
}

// MarkCompleted marks the processing claim as completed.
func (r *DefaultRepository) MarkCompleted(
	ctx context.Context,
	consumerGroup string,
	eventID string,
	lockID string,
) error {
	now := timex.Now()

	res, err := r.db.NewUpdate().
		Model((*inbox.Record)(nil)).
		Set("status", inbox.StatusCompleted).
		Set("lock_id", nil).
		Set("locked_until", nil).
		Set("completed_at", now).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("consumer_group", consumerGroup).
				Equals("event_id", eventID).
				Equals("status", string(inbox.StatusProcessing)).
				Equals("lock_id", lockID)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("inbox: mark completed (%s, %s): %w", consumerGroup, eventID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("inbox: mark completed rows affected (%s, %s): %w", consumerGroup, eventID, err)
	}

	if affected == 0 {
		return fmt.Errorf("inbox: mark completed (%s, %s): %w", consumerGroup, eventID, inbox.ErrLockLost)
	}

	return nil
}

// Release removes a failed processing claim.
func (r *DefaultRepository) Release(
	ctx context.Context,
	consumerGroup string,
	eventID string,
	lockID string,
) error {
	_, err := r.db.NewDelete().
		Model((*inbox.Record)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("consumer_group", consumerGroup).
				Equals("event_id", eventID).
				Equals("status", string(inbox.StatusProcessing)).
				Equals("lock_id", lockID)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("inbox: release (%s, %s): %w", consumerGroup, eventID, err)
	}

	return nil
}

// DeleteOlderThan removes completed records strictly older than the cutoff.
func (r *DefaultRepository) DeleteOlderThan(ctx context.Context, cutoff timex.DateTime) (int64, error) {
	res, err := r.db.NewDelete().
		Model((*inbox.Record)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("status", string(inbox.StatusCompleted)).LessThan("completed_at", cutoff)
		}).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("inbox: cleanup: %w", err)
	}

	return res.RowsAffected()
}
