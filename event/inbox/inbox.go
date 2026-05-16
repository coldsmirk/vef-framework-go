// Package inbox declares the idempotency record model and repository
// contract used by the consume-side Inbox middleware. The default
// repository persists to sys_event_inbox.
package inbox

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
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
}

// Repository is the persistence boundary for inbox records. Custom
// implementations may target alternative stores; default impl uses
// the framework's orm.DB.
type Repository interface {
	// TryInsert atomically inserts a record. Returns true when the
	// caller acquired the slot (first delivery), false when the
	// (consumerGroup, eventID) pair already exists (duplicate).
	TryInsert(ctx context.Context, consumerGroup, eventID string) (acquired bool, err error)
	// DeleteOlderThan drops records whose created_at is before the
	// cutoff. Returns the deleted row count.
	DeleteOlderThan(ctx context.Context, cutoff timex.DateTime) (int64, error)
}
