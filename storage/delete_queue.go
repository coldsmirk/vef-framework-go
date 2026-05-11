package storage

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// DeleteReason classifies why an object was scheduled for deletion.
// Used for observability; the worker's behaviour is independent of reason.
type DeleteReason string

const (
	// DeleteReasonReplaced indicates the object was the previous value of a
	// business field that has just been overwritten with a new key.
	DeleteReasonReplaced DeleteReason = "replaced"
	// DeleteReasonDeleted indicates the owning business record was deleted.
	DeleteReasonDeleted DeleteReason = "deleted"
	// DeleteReasonClaimExpired indicates a pending upload claim expired and
	// its associated object (if any) must be cleaned up.
	DeleteReasonClaimExpired DeleteReason = "claim_expired"
)

// PendingDelete is a queued instruction to delete a single object from the
// storage backend. Rows are inserted by the CRUD layer inside the same
// business transaction that dereferenced the file, so the queue inherits
// the atomicity of the business write. The delete worker drains the queue
// asynchronously with retry/backoff.
type PendingDelete struct {
	orm.BaseModel `bun:"table:storage_pending_deletes,alias:spd" json:"-"`

	ID            string         `json:"id"            bun:"id,pk"`
	Key           string         `json:"key"           bun:"object_key,notnull"`
	Reason        DeleteReason   `json:"reason"        bun:"reason,notnull,default:'replaced'"`
	Attempts      int            `json:"attempts"      bun:"attempts,notnull,default:0"`
	NextAttemptAt timex.DateTime `json:"nextAttemptAt" bun:"next_attempt_at,notnull,type:timestamp,default:CURRENT_TIMESTAMP"`
	CreatedAt     timex.DateTime `json:"createdAt"     bun:"created_at,notnull,type:timestamp,default:CURRENT_TIMESTAMP,skipupdate"`
}

// DeleteQueue is the durable queue backing background object deletion.
//
// Lifecycle:
//
//  1. The CRUD layer Schedules items inside the business transaction; the
//     INSERT commits atomically with the business write.
//  2. The delete worker Leases due rows in batches. Lease atomically pushes
//     each leased row's NextAttemptAt into the future (visibility timeout)
//     so concurrent workers (multi-instance deployments, retried jobs)
//     cannot pick the same row.
//  3. On successful object deletion the worker calls Done to remove the
//     row. On transient failure the worker calls Defer with a backoff
//     timestamp; on crash the lease silently expires and the row becomes
//     visible to the next Lease.
type DeleteQueue interface {
	// Schedule INSERTs items inside tx. tx must be the orm.DB instance
	// passed into RunInTX so that scheduling shares the business
	// transaction's atomicity guarantees. items may be empty (no-op).
	Schedule(ctx context.Context, tx orm.DB, items []PendingDelete) error

	// Lease atomically claims up to limit rows whose NextAttemptAt <= now,
	// pushing each claimed row's NextAttemptAt to now+leaseDuration.
	// Returned rows are the worker's responsibility until Done or Defer is
	// called or the lease expires. leaseDuration should comfortably exceed
	// expected per-item processing time (e.g. 5 minutes for object delete).
	Lease(ctx context.Context, now timex.DateTime, limit int, leaseDuration time.Duration) ([]PendingDelete, error)

	// Done removes the rows identified by ids in a single batch (DELETE).
	// ids may be empty (no-op).
	Done(ctx context.Context, ids []string) error

	// Defer atomically increments Attempts and sets NextAttemptAt = nextAt
	// for the row identified by id. The worker uses this on transient
	// failure with an exponential-backoff timestamp.
	Defer(ctx context.Context, id string, nextAt timex.DateTime) error
}
