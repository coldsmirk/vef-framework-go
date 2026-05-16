package inbox

import (
	"context"
	"errors"
	"fmt"

	pubinbox "github.com/coldsmirk/vef-framework-go/event/inbox"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// DefaultRepository is the orm.DB-backed Repository used by the inbox
// middleware. TryInsert relies on the UNIQUE constraint over
// (consumer_group, event_id) to detect duplicates atomically.
type DefaultRepository struct {
	db orm.DB
}

// NewRepository constructs a DefaultRepository.
func NewRepository(db orm.DB) *DefaultRepository {
	return &DefaultRepository{db: db}
}

// TryInsert atomically claims the (consumerGroup, eventID) slot.
// Returns (true, nil) on first delivery and (false, nil) on duplicate.
func (r *DefaultRepository) TryInsert(ctx context.Context, consumerGroup, eventID string) (bool, error) {
	record := &pubinbox.Record{
		EventID:       eventID,
		ConsumerGroup: consumerGroup,
	}
	record.ID = id.Generate()

	_, err := r.db.NewInsert().Model(record).Exec(ctx)
	if err == nil {
		return true, nil
	}
	// The framework ORM translates dialect-specific unique-violation
	// codes into result.ErrRecordAlreadyExists, which is what every
	// supported backend funnels into.
	if errors.Is(err, result.ErrRecordAlreadyExists) {
		return false, nil
	}

	return false, fmt.Errorf("inbox: try insert (%s, %s): %w", consumerGroup, eventID, err)
}

// DeleteOlderThan removes records strictly older than the cutoff.
func (r *DefaultRepository) DeleteOlderThan(ctx context.Context, cutoff timex.DateTime) (int64, error) {
	res, err := r.db.NewDelete().
		Model((*pubinbox.Record)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.LessThan("created_at", cutoff)
		}).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("inbox: cleanup: %w", err)
	}

	return res.RowsAffected()
}

// ErrDuplicate is exported for callers that want to bypass TryInsert
// and detect duplicates from a raw insert error.
var ErrDuplicate = errors.New("inbox: duplicate (consumer_group, event_id)")
