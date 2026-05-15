package storage

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
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
	// any entry in keys AND whose created_by matches principal.ID,
	// executed inside the supplied business transaction tx. Returns
	// ErrClaimNotFound (wrapped) when any key has no matching row —
	// covering both "no such claim" and "claim exists but belongs to a
	// different principal" in a single sentinel, so a successful return
	// proves the caller owns every key it referenced.
	//
	// The ownership check is unconditional: business code never needs
	// to remember to authorize claim consumption separately. Passing a
	// nil principal (or one with an empty ID) is always rejected.
	//
	// Keys may contain duplicates; implementations are responsible for
	// deduplicating before issuing the underlying DELETE. An empty or
	// nil keys argument is a no-op and returns nil.
	ConsumeMany(ctx context.Context, tx orm.DB, principal *security.Principal, keys []string) error
}
