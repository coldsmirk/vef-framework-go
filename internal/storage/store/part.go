package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// UploadPart records one chunk of an in-flight chunked upload. Rows are
// inserted by storage_resource's upload_part handler after the backend
// returns the part's ETag and are deleted by complete_upload / abort
// (or cascaded when the parent claim is deleted by the sweeper).
//
// Internal type: business code never sees parts. The Files facade and
// public storage.ClaimConsumer interface only deal in finalized claims.
type UploadPart struct {
	orm.BaseModel `json:"-" bun:"table:sys_storage_upload_part,alias:sup"`

	ID         string         `json:"id"         bun:"id,pk"`
	ClaimID    string         `json:"claimId"    bun:"claim_id"`
	PartNumber int            `json:"partNumber" bun:"part_number"`
	ETag       string         `json:"eTag"       bun:"etag"`
	Size       int64          `json:"size"       bun:"size"`
	CreatedAt  timex.DateTime `json:"createdAt"  bun:"created_at,skipupdate"`
}

// UploadPartStore persists per-part bookkeeping for chunked uploads.
//
// All transactional methods (Upsert, DeleteByClaim) take an orm.DB tx
// parameter — they only execute inside the storage_resource handlers,
// which already own the surrounding transaction. ListByClaim is
// non-transactional because complete_upload reads parts before opening
// its commit transaction.
//
// Internal type: not part of the public storage surface.
type UploadPartStore interface {
	// Upsert inserts a new part row or overwrites the ETag / Size of an
	// existing (claim_id, part_number) row. Used by upload_part to make
	// part re-upload safe (the backend returns a fresh ETag and the
	// caller persists it without caring about previous attempts).
	Upsert(ctx context.Context, tx orm.DB, part *UploadPart) error

	// ListByClaim returns every part of the given claim, sorted ascending
	// by PartNumber. complete_upload uses the result to assemble the
	// CompletedPart list it hands to the backend.
	ListByClaim(ctx context.Context, claimID string) ([]UploadPart, error)

	// DeleteByClaim removes every part of the given claim inside tx.
	// Used by complete_upload (after the backend assembles the final
	// object) and abort_upload (cleanup). The schema also cascades on
	// claim deletion; this method is the explicit path for the success
	// case where the claim row remains in 'uploaded' state.
	DeleteByClaim(ctx context.Context, tx orm.DB, claimID string) error
}
