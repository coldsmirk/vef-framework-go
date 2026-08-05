package storage

import (
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// FileStatus enumerates the lifecycle states of a FileRecord.
//
//   - FileStatusUploaded: the object is finalized in the backend but no
//     business transaction has adopted it yet. Rows that stay in this
//     state long after their upload are orphans — the upload succeeded
//     and the business save never happened.
//   - FileStatusClaimed: a business transaction consumed the upload
//     claim, so business data references the object.
//   - FileStatusDeleted: the delete worker removed the object from the
//     backend. The row is kept as the record that the file existed.
type FileStatus string

const (
	FileStatusUploaded FileStatus = "uploaded"
	FileStatusClaimed  FileStatus = "claimed"
	FileStatusDeleted  FileStatus = "deleted"
)

// FileRecord is the durable registry row for one successfully uploaded
// object. It is written when the upload is finalized and outlives the
// short-lived sys_storage_upload_claim row that produced it — which is
// the point: the claim is deleted the moment a business transaction
// adopts the file, taking the original filename with it.
//
// The record is a projection of the claim: every field is copied from
// the claim row, so the two write paths that finalize an upload
// (complete_upload and the claim sweeper's recovery branch) produce
// identical records. ID is the originating claim's ID, which keeps the
// row correlated with the upload-session logs.
//
// Status tracks the object's real lifecycle so the table can be trusted
// as an inventory rather than an append-only log: a row still at
// FileStatusUploaded long after StartedAt is an orphan, and a row at
// FileStatusDeleted records a file that existed and no longer does.
//
// Business code reads records through FileRegistry; writing them is the
// framework's own completeness invariant and is deliberately not part of
// any public surface.
type FileRecord struct {
	orm.BaseModel `json:"-" bun:"table:sys_storage_file,alias:sf"`
	orm.CreationAuditedModel

	// Key is the storage object key — the value business models carry in
	// their `meta:"uploaded_file"` fields, and the natural join key.
	Key string `json:"key" bun:"object_key"`
	// OriginalFilename is the client-supplied name at upload time.
	OriginalFilename string `json:"originalFilename" bun:"original_filename"`
	// ContentType is the sanitized MIME type stored on the object.
	ContentType string `json:"contentType" bun:"content_type"`
	// Size is the object size in bytes.
	Size int64 `json:"size" bun:"size"`
	// Public reports whether the object landed under the public prefix.
	Public bool `json:"public" bun:"public"`
	// Status is the object's lifecycle state.
	Status FileStatus `json:"status" bun:"status"`
	// StartedAt is when the upload session was opened. CreatedAt records
	// when this row was written instead — the completion instant on the
	// normal path, but the recovery instant when the claim sweeper
	// adopted an upload whose complete_upload call never arrived, which
	// can trail the real completion by the whole claim TTL. StartedAt is
	// the same value on both paths and is therefore the honest answer to
	// "when was this uploaded".
	StartedAt timex.DateTime `json:"startedAt" bun:"started_at"`
	// ClaimedAt is when a business transaction adopted the file, or nil
	// while the object is still unreferenced.
	ClaimedAt *timex.DateTime `json:"claimedAt,omitempty" bun:"claimed_at,nullzero"`
	// DeletedAt is when the delete worker removed the object from the
	// backend, or nil while the object exists.
	DeletedAt *timex.DateTime `json:"deletedAt,omitempty" bun:"deleted_at,nullzero"`
	// DeleteReason carries the reason the object was scheduled for
	// deletion; empty until the object is deleted.
	DeleteReason DeleteReason `json:"deleteReason,omitempty" bun:"delete_reason"`
}

// IsDeleted reports whether the object behind this record has been
// removed from the storage backend.
func (r *FileRecord) IsDeleted() bool {
	return r.Status == FileStatusDeleted
}
