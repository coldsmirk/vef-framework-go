package store

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// ClaimStatus enumerates the lifecycle states of an UploadClaim row.
//
//   - StatusPending: the claim's underlying object is not yet finalized
//     (chunked uploads between init_upload and complete_upload). Business
//     code MUST NOT consume a pending claim; ConsumeMany filters them out.
//   - StatusUploaded: the object exists in the backend and the claim is
//     eligible for business consumption (Files.OnCreate).
type ClaimStatus string

const (
	ClaimStatusPending  ClaimStatus = "pending"
	ClaimStatusUploaded ClaimStatus = "uploaded"
)

// UploadClaim is the in-flight bookkeeping row for an upload that has
// not yet been adopted by a business transaction. Claims are
// short-lived: inserted by upload / init_upload, transitioned to
// 'uploaded' by upload / complete_upload, then deleted by Consume (when
// business commits) or by ScanExpired + DeleteByID (when the TTL
// elapses).
//
// The row carries enough context to abort multipart sessions and delete
// abandoned objects. It is NOT a long-term audit record.
//
// Internal type: only the storage worker, upload flow, and the claim
// sweeper construct or consume UploadClaim values. Business code
// interacts with claims indirectly through storage.ClaimConsumer (and
// the higher-level storage.Files facade).
type UploadClaim struct {
	orm.BaseModel `json:"-" bun:"table:sys_storage_upload_claim,alias:suc"`

	ID               string         `json:"id"               bun:"id,pk"`
	CreatedAt        timex.DateTime `json:"createdAt"        bun:"created_at,skipupdate"`
	CreatedBy        string         `json:"createdBy"        bun:"created_by,skipupdate"`
	Key              string         `json:"key"              bun:"object_key"`
	UploadID         string         `json:"uploadId"         bun:"upload_id"`
	Size             int64          `json:"size"             bun:"size"`
	ContentType      string         `json:"contentType"      bun:"content_type"`
	OriginalFilename string         `json:"originalFilename" bun:"original_filename"`
	Status           ClaimStatus    `json:"status"           bun:"status"`
	Public           bool           `json:"public"           bun:"public"`
	PartSize         int64          `json:"partSize"         bun:"part_size"`
	PartCount        int            `json:"partCount"        bun:"part_count"`
	ExpiresAt        timex.DateTime `json:"expiresAt"        bun:"expires_at"`
}

// IsMultipart reports whether the claim represents a multipart upload
// session (UploadID is non-empty).
func (c *UploadClaim) IsMultipart() bool {
	return c.UploadID != ""
}

// IsUploaded reports whether the claim's underlying object has been
// finalized in the backend and is eligible for business consumption.
func (c *UploadClaim) IsUploaded() bool {
	return c.Status == ClaimStatusUploaded
}

// ClaimStore persists upload claims. Implementations are expected to be
// safe for concurrent use. The interface deliberately splits
// transactional methods (taking an orm.DB tx parameter) from
// non-transactional ones (Create, Get*, ScanExpired, DeleteByID) used
// by the upload init flow and by the claim sweeper worker respectively.
//
// Internal type: business code uses the minimal storage.ClaimConsumer
// interface (which ClaimStore satisfies via embedding) and the
// higher-level storage.Files facade. ClaimStore itself is consumed only
// by the init/abort flow (storage_resource) and the claim sweeper.
type ClaimStore interface {
	// ConsumeMany is the only method business code reaches through the
	// public storage.ClaimConsumer interface. Embedding keeps the two
	// surfaces in lock-step at compile time. Only claims with
	// Status='uploaded' are eligible; pending claims behave as if they
	// did not exist and surface ErrClaimNotFound.
	storage.ClaimConsumer

	// Create persists a new pending claim. Returns an error if a row with
	// the same Key already exists.
	Create(ctx context.Context, claim *UploadClaim) error

	// UpdateUploadID sets the upload_id field on an existing claim row.
	// Used by the upload init flow to attach a backend multipart session
	// ID after the claim row has been persisted (INSERT-first ordering).
	// Returns ErrClaimNotFound when no row matches id.
	UpdateUploadID(ctx context.Context, id, uploadID string) error

	// MarkUploaded flips claim.status from 'pending' to 'uploaded' inside
	// tx so the business layer can consume the claim. Used by the
	// complete_upload (and synchronous single-shot upload) paths once
	// the underlying object is finalized. Returns ErrClaimNotFound when
	// no row matches id.
	MarkUploaded(ctx context.Context, tx orm.DB, id string) error

	// Get returns the claim by ID, or ErrClaimNotFound.
	Get(ctx context.Context, id string) (*UploadClaim, error)

	// CountPendingByOwner returns the number of claims with
	// status='pending' owned by the given principal. Used by init_upload
	// to enforce the per-user in-flight session cap.
	CountPendingByOwner(ctx context.Context, owner string) (int, error)

	// GetByKey returns the claim by object key, or ErrClaimNotFound.
	GetByKey(ctx context.Context, key string) (*UploadClaim, error)

	// Consume deletes the claim row for key, executed inside the supplied
	// business transaction tx. Returns ErrClaimNotFound when no row
	// exists for key OR when the row is still pending. tx must be the
	// same orm.DB instance passed to RunInTX.
	Consume(ctx context.Context, tx orm.DB, key string) error

	// ScanExpired returns up to limit claims whose ExpiresAt is before now.
	// Used by the claim sweeper worker to drive cleanup.
	ScanExpired(ctx context.Context, now timex.DateTime, limit int) ([]UploadClaim, error)

	// DeleteByID removes a single claim row, used after the upload abort
	// path has finished cleaning up the corresponding storage side-effects.
	// Non-transactional; safe to call outside a business transaction.
	DeleteByID(ctx context.Context, id string) error

	// DeleteByIDInTx removes a single claim row inside tx. Used by the
	// abort_upload flow so the part-row cascade and the claim-row delete
	// commit together.
	DeleteByIDInTx(ctx context.Context, tx orm.DB, id string) error

	// DeleteByIDs removes multiple claim rows inside the supplied
	// transaction. Used by the claim sweeper to atomically pair the
	// claim-row removal with the corresponding DeleteQueue.Schedule call,
	// guaranteeing the queue and the claim table commit together.
	DeleteByIDs(ctx context.Context, tx orm.DB, ids []string) error
}
