package inbox

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Status is the lifecycle state of an inbox record.
type Status string

const (
	// StatusProcessing indicates a consumer has claimed the delivery
	// but has not completed its handler yet.
	StatusProcessing Status = "processing"
	// StatusCompleted indicates the handler completed successfully and
	// future duplicate deliveries should be acknowledged without
	// re-running business code.
	StatusCompleted Status = "completed"
)

// AcquireResult reports how a delivery claim was resolved.
type AcquireResult string

const (
	// AcquireResultAcquired means the caller owns the delivery and
	// should run the handler.
	AcquireResultAcquired AcquireResult = "acquired"
	// AcquireResultCompleted means the delivery was already handled
	// successfully and should be acknowledged.
	AcquireResultCompleted AcquireResult = "completed"
	// AcquireResultInProgress means another consumer still holds a
	// non-expired processing claim; callers should return an error so
	// at-least-once transports retry later.
	AcquireResultInProgress AcquireResult = "in_progress"
)

// Record is the persisted idempotency marker. The unique index on
// (consumer_group, event_id) is the dedupe primitive.
type Record struct {
	orm.BaseModel `bun:"table:sys_event_inbox,alias:sei"`
	orm.Model
	orm.CreationTrackedModel

	// EventID is the framework-generated message ID from Envelope.ID.
	EventID string `json:"eventId" bun:"event_id"`
	// ConsumerGroup identifies the dedupe scope. Two subscriptions
	// in different groups receive the message independently.
	ConsumerGroup string `json:"consumerGroup" bun:"consumer_group"`
	// Status records whether this delivery is in-flight or completed.
	Status Status `json:"status" bun:"status"`
	// LockID identifies the current processing lease owner.
	LockID string `json:"lockId,omitempty" bun:"lock_id,nullzero"`
	// LockedUntil is the processing lease deadline. A later duplicate
	// may re-acquire the delivery once this timestamp has expired.
	LockedUntil *timex.DateTime `json:"lockedUntil,omitempty" bun:"locked_until,nullzero"`
	// CompletedAt records when the handler completed successfully.
	CompletedAt *timex.DateTime `json:"completedAt,omitempty" bun:"completed_at,nullzero"`
}

// Repository is the persistence boundary for inbox records. Custom
// implementations may target alternative stores; default impl uses
// the framework's orm.DB.
type Repository interface {
	// Acquire claims a delivery for processing until lockUntil. It
	// returns Acquired for first or expired deliveries, Completed for
	// already-successful duplicates, and InProgress for active claims.
	// The lockID is populated only when the result is Acquired.
	Acquire(ctx context.Context, consumerGroup, eventID string, lockUntil timex.DateTime) (AcquireResult, string, error)
	// MarkCompleted marks a previously acquired delivery as successful.
	// It returns ErrLockLost when lockID no longer owns the claim.
	MarkCompleted(ctx context.Context, consumerGroup, eventID, lockID string) error
	// Release removes a processing claim after handler failure so the
	// next delivery attempt can run the handler again.
	Release(ctx context.Context, consumerGroup, eventID, lockID string) error
	// DeleteOlderThan drops completed records whose completed_at is
	// before the cutoff. Returns the deleted row count.
	DeleteOlderThan(ctx context.Context, cutoff timex.DateTime) (int64, error)
}
