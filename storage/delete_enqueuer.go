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
	// and its associated object (if any) must be cleaned up. Reserved
	// for the framework-internal claim sweeper; business code should
	// not pass this reason to Enqueue.
	DeleteReasonClaimExpired DeleteReason = "claim_expired"
)

// DeleteEnqueuer is the minimal queue-side surface business code needs
// to drop file references into the asynchronous delete pipeline inside
// a CRUD transaction. The framework's Files facade is built on top of
// ClaimConsumer and DeleteEnqueuer; most applications only ever
// interact with Files and never reach for DeleteEnqueuer directly.
//
// The richer set of operations (leasing rows, marking them done,
// deferring with backoff, dead-letter parking) lives on the
// framework-internal store.DeleteQueue type and is not part of the
// stable public surface. Business code that genuinely needs to
// inspect the queue should drop down to a dependency on the storage
// internal package via a custom integration rather than depending on
// this minimal interface to expand.
//
// Implementations MUST be safe for concurrent use.
type DeleteEnqueuer interface {
	// Enqueue INSERTs one pending-delete row per key inside tx, all
	// carrying the supplied reason. tx must be the orm.DB instance
	// passed into RunInTx so that enqueuing shares the business
	// transaction's atomicity guarantees. keys may be empty or nil
	// (no-op).
	//
	// Keys may contain duplicates; implementations are responsible for
	// deduplicating before issuing the underlying INSERT.
	Enqueue(ctx context.Context, tx orm.DB, keys []string, reason DeleteReason) error
}
