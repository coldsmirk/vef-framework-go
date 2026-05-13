package storage

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// ClaimConsumer is the minimal claim-side surface business code needs
// when reconciling file references inside a CRUD transaction. The
// framework's Files facade composes ClaimConsumer with DeleteScheduler;
// most applications only ever interact with Files and never reach for
// ClaimConsumer directly.
//
// The richer set of operations (Create, ScanExpired, Get*, etc.) lives
// on the framework-internal store.ClaimStore type and is not part of
// the stable public surface. Business code that genuinely needs to
// inspect or manipulate raw claim rows should drop down to a
// dependency on the storage internal package via a custom integration
// rather than depending on this minimal interface to expand.
//
// Implementations MUST be safe for concurrent use.
type ClaimConsumer interface {
	// ConsumeMany deletes the upload_claim rows whose object_key matches
	// any entry in keys, executed inside the supplied business
	// transaction tx. Returns ErrClaimNotFound (wrapped) when any key has
	// no corresponding row, signaling that the business write
	// references either an uncommitted or already-swept claim and the
	// caller's transaction should roll back. tx must be the same orm.DB
	// instance passed to RunInTX.
	//
	// Keys may contain duplicates; implementations are responsible for
	// deduplicating before issuing the underlying DELETE. An empty or
	// nil keys argument is a no-op and returns nil.
	ConsumeMany(ctx context.Context, tx orm.DB, keys []string) error
}
