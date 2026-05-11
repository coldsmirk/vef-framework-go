package storage

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// UploadClaim is the in-flight bookkeeping row for an upload that has not
// yet been adopted by a business transaction. Claims are short-lived:
// inserted by InitUpload, deleted by Consume (when business commits) or
// by ScanExpired + DeleteByID (when the TTL elapses).
//
// The row carries enough context to abort multipart sessions and delete
// abandoned objects. It is NOT a long-term audit record.
//
// Field order follows the project's id → audit → business → lifecycle
// convention (see orm.CreationAuditedModel). orm.CreationAuditedModel is
// not embedded directly so we avoid the unused CreatedByName scanonly
// column, which makes no sense for a short-lived claim row.
type UploadClaim struct {
	orm.BaseModel `bun:"table:storage_upload_claims,alias:suc" json:"-"`

	ID          string            `json:"id"          bun:"id,pk"`
	CreatedAt   timex.DateTime    `json:"createdAt"   bun:"created_at,notnull,type:timestamp,default:CURRENT_TIMESTAMP,skipupdate"`
	CreatedBy   string            `json:"createdBy"   bun:"created_by,notnull,skipupdate" mold:"translate=user?"`
	Key         string            `json:"key"         bun:"object_key,notnull,unique"`
	UploadID    string            `json:"uploadId"    bun:"upload_id,notnull"`
	Bucket      string            `json:"bucket"      bun:"bucket,notnull"`
	Size        int64             `json:"size"        bun:"size,notnull"`
	ContentType string            `json:"contentType" bun:"content_type,notnull"`
	Metadata    map[string]string `json:"metadata"    bun:"metadata,nullzero,type:jsonb"`
	ExpiresAt   timex.DateTime    `json:"expiresAt"   bun:"expires_at,notnull,type:timestamp"`
}

// IsMultipart reports whether the claim represents a multipart upload
// session (UploadID is non-empty).
func (c *UploadClaim) IsMultipart() bool {
	return c.UploadID != ""
}

// ClaimStore persists upload claims. Implementations are expected to be
// safe for concurrent use. The interface deliberately splits transactional
// methods (taking an orm.DB tx parameter) from non-transactional ones
// (Create, Get*, ScanExpired, DeleteByID) used by the upload init flow
// and by the claim sweeper worker respectively.
type ClaimStore interface {
	// Create persists a new pending claim. Returns an error if a row with
	// the same Key already exists.
	Create(ctx context.Context, claim *UploadClaim) error

	// Get returns the claim by ID, or ErrClaimNotFound.
	Get(ctx context.Context, id string) (*UploadClaim, error)

	// GetByKey returns the claim by object key, or ErrClaimNotFound.
	GetByKey(ctx context.Context, key string) (*UploadClaim, error)

	// Consume deletes the claim row for key, executed inside the supplied
	// business transaction tx. Returns ErrClaimNotFound when no row exists
	// for key, signalling that the business write references either an
	// uncommitted or already-swept claim and the transaction should roll
	// back. tx must be the same orm.DB instance passed to RunInTX.
	Consume(ctx context.Context, tx orm.DB, key string) error

	// ConsumeMany batches Consume across keys. All-or-nothing: if any key
	// is missing the call returns ErrClaimNotFound and tx will roll back
	// when the caller returns the error.
	ConsumeMany(ctx context.Context, tx orm.DB, keys []string) error

	// ScanExpired returns up to limit claims whose ExpiresAt is before now.
	// Used by the claim sweeper worker to drive cleanup.
	ScanExpired(ctx context.Context, now timex.DateTime, limit int) ([]UploadClaim, error)

	// DeleteByID removes a single claim row, used after the sweeper has
	// finished cleaning up the corresponding storage side-effects.
	DeleteByID(ctx context.Context, id string) error
}
