package store

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// PendingDelete is a queued instruction to delete a single object from the
// storage backend. Rows are inserted by the CRUD layer inside the same
// business transaction that dereferenced the file, so the queue inherits
// the atomicity of the business write. The delete worker drains the queue
// asynchronously with retry/backoff.
//
// UploadID is non-empty only for rows scheduled by the claim sweeper for
// expired multipart claims; the worker will best-effort abort the dangling
// multipart session before deleting the object.
//
// Internal type: business code never constructs PendingDelete values
// directly. The higher-level storage.Files facade and the public
// storage.DeleteScheduler interface accept (key, reason) pairs and the
// implementation builds these rows internally. The claim sweeper uses
// Enqueue to retain control over UploadID + Reason on a per-row basis.
type PendingDelete struct {
	orm.BaseModel `json:"-" bun:"table:sys_storage_pending_delete,alias:spd"`

	ID            string               `json:"id"            bun:"id,pk"`
	Key           string               `json:"key"           bun:"object_key"`
	UploadID      string               `json:"uploadId"      bun:"upload_id"`
	Reason        storage.DeleteReason `json:"reason"        bun:"reason"`
	Attempts      int                  `json:"attempts"      bun:"attempts"`
	NextAttemptAt timex.DateTime       `json:"nextAttemptAt" bun:"next_attempt_at"`
	CreatedAt     timex.DateTime       `json:"createdAt"     bun:"created_at,skipupdate"`
}

// IsMultipart reports whether this pending-delete row references a backend
// multipart session that must be aborted before the object can be deleted.
func (p *PendingDelete) IsMultipart() bool {
	return p.UploadID != ""
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
//
// Deployment notes:
//
//   - Multi-instance: the visibility timeout protects against double-
//     processing within a single tick, but multiple worker instances will
//     still race for the same set of due rows on every tick. The default
//     implementation uses SELECT ... FOR UPDATE SKIP LOCKED so each
//     worker leases a disjoint slice without leader election; SQLite's
//     single-writer model makes the locking degenerate harmlessly.
//
//   - S3 incomplete multipart cleanup: the upload init flow occasionally
//     leaves orphan multipart sessions on the backend if the database
//     write following InitMultipart fails. The framework reaps these
//     through the claim sweeper for happy-path failures, but operators
//     should still configure an S3 lifecycle rule that aborts incomplete
//     multipart uploads after N days as a defense-in-depth measure.
//
// Internal type: business code uses the minimal storage.DeleteScheduler
// interface (which DeleteQueue satisfies via embedding). DeleteQueue is
// consumed directly only by the storage worker (Lease/Done/Defer) and
// by the claim sweeper (Enqueue, which retains UploadID for abort).
type DeleteQueue interface {
	// Schedule is the only method business code reaches through the
	// public storage.DeleteScheduler interface. Embedding keeps the two
	// surfaces in lock-step at compile time.
	storage.DeleteScheduler

	// Enqueue INSERTs fully-formed rows inside tx. Used by the claim
	// sweeper to forward UploadID + claim_expired reason on a per-row
	// basis. items may be empty (no-op).
	Enqueue(ctx context.Context, tx orm.DB, items []PendingDelete) error

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
