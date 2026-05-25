package storage

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// ClaimConsumer is the minimal claim-side surface business code needs
// when reconciling file references inside a CRUD transaction. The
// framework's Files facade is built on top of ClaimConsumer and
// DeleteEnqueuer; most applications only ever interact with Files and
// never reach for ClaimConsumer directly.
//
// The richer set of operations (Create, ListExpired, Get, etc.) lives
// on the framework-internal store.ClaimStore type and is not part of
// the stable public surface. Business code that genuinely needs to
// inspect or manipulate raw claim rows should drop down to a
// dependency on the storage internal package via a custom integration
// rather than depending on this minimal interface to expand.
//
// Implementations MUST be safe for concurrent use.
type ClaimConsumer interface {
	// Consume deletes the upload_claim rows whose object_key matches
	// any entry in keys AND whose created_by matches principal.ID,
	// executed inside the supplied business transaction tx. Returns
	// ErrClaimNotFound (wrapped) when any key has no matching row —
	// covering both "no such claim" and "claim exists but belongs to a
	// different principal" in a single sentinel, so a successful return
	// proves the caller owns every key it referenced.
	//
	// Returns ErrAccessDenied (wrapped) for principals the framework
	// classifies as anonymous (nil, empty ID, or the shared anonymous
	// sentinel) — claims are per-principal, so an anonymous scope can
	// never own them. Background jobs operating on behalf of "the
	// system" must pass a synthetic system principal.
	//
	// Keys may contain duplicates; implementations are responsible for
	// deduplicating before issuing the underlying DELETE. An empty or
	// nil keys argument is a no-op and returns nil.
	Consume(ctx context.Context, tx orm.DB, principal *security.Principal, keys []string) error
}
