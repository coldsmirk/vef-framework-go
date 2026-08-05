package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// FileStore persists the durable upload registry (sys_storage_file).
// Implementations are expected to be safe for concurrent use.
//
// Every mutation takes a transaction: a registry row must become
// visible exactly when the fact it records does. Record commits with
// the claim's pending → uploaded transition, MarkClaimed with the
// business write that adopts the file, MarkDeleted with the delete
// worker's queue-row removal.
//
// Internal type: business code reads records through the minimal
// storage.FileRegistry interface (which FileStore satisfies via
// embedding) and never writes them.
type FileStore interface {
	// Embedding the public read surface guarantees FileStore always
	// exposes at least it: any addition to storage.FileRegistry
	// propagates here automatically, while the write operations stay
	// framework-internal.
	storage.FileRegistry

	// Record inserts the registry row for a finalized upload inside tx.
	// Every field is projected from the claim, so both paths that
	// finalize an upload produce identical records.
	//
	// Deliberately not idempotent: the two callers each gate this on
	// winning a compare-and-set that flips the claim from pending to
	// uploaded, and a claim never returns to pending, so a duplicate
	// insert means that invariant is broken and must fail loudly rather
	// than be swallowed by a conflict clause.
	Record(ctx context.Context, tx orm.DB, claim UploadClaim) error

	// MarkClaimed transitions the records for keys from uploaded to
	// claimed inside tx. Best-effort by contract: the claim table — not
	// the registry — is the authority on what a caller may adopt, and a
	// key whose record is missing (a file uploaded before the registry
	// existed) must never fail the business write that adopts it.
	MarkClaimed(ctx context.Context, tx orm.DB, keys []string) error

	// MarkDeleted records that the object behind key no longer exists in
	// the backend. First writer wins: a record already marked deleted is
	// left untouched, so the reason that actually removed the object is
	// not rewritten by a later queue row for the same key. Matching no
	// row is normal (objects whose upload never finalized have no
	// record) and is not an error.
	MarkDeleted(ctx context.Context, tx orm.DB, key string, reason storage.DeleteReason) error
}
