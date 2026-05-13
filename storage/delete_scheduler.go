package storage

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// DeleteReason classifies why an object was scheduled for deletion. The
// reason is persisted on the queue row and forwarded onto file-deleted
// and dead-letter events for observability; the worker's behavior is
// independent of reason.
type DeleteReason string

const (
	// DeleteReasonReplaced indicates the object was the previous value of
	// a business field that has just been overwritten with a new key.
	DeleteReasonReplaced DeleteReason = "replaced"
	// DeleteReasonDeleted indicates the owning business record was
	// deleted.
	DeleteReasonDeleted DeleteReason = "deleted"
	// DeleteReasonClaimExpired indicates a pending upload claim expired
	// and its associated object (if any) must be cleaned up. Set only by
	// the framework-internal claim sweeper.
	DeleteReasonClaimExpired DeleteReason = "claim_expired"
)

// DeleteScheduler is the minimal queue-side surface business code needs
// to drop file references into the asynchronous delete pipeline inside a
// CRUD transaction. The framework's Files facade composes ClaimConsumer
// with DeleteScheduler; most applications only ever interact with Files
// and never reach for DeleteScheduler directly.
//
// The richer set of operations (Lease, Done, Defer, sweeper-side
// Enqueue) lives on the framework-internal store.DeleteQueue type and
// is not part of the stable public surface. Business code that
// genuinely needs to inspect the queue should drop down to a dependency
// on the storage internal package via a custom integration rather than
// depending on this minimal interface to expand.
//
// Implementations MUST be safe for concurrent use.
type DeleteScheduler interface {
	// Schedule INSERTs one pending-delete row per key inside tx, all
	// carrying the supplied reason. tx must be the orm.DB instance
	// passed into RunInTX so that scheduling shares the business
	// transaction's atomicity guarantees. keys may be empty or nil
	// (no-op).
	//
	// Keys may contain duplicates; implementations are responsible for
	// deduplicating before issuing the underlying INSERT.
	Schedule(ctx context.Context, tx orm.DB, keys []string, reason DeleteReason) error
}
