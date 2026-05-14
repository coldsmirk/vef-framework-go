package storage

import (
	"context"
	"errors"
	"mime"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/storage/store"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const (
	templateDatePath = "2006/01/02"
	defaultExtension = ".bin"
)

// safeExtPattern allows only alphanumeric extensions to prevent control
// characters or path separators from leaking into object keys.
var safeExtPattern = regexp.MustCompile(`^\.[a-zA-Z0-9]+$`)

// safeContentTypePrefixes and safeContentTypes gate sanitizeContentType.
// Hoisted to package scope so the upload hot path does not re-allocate
// them on every InitUpload call.
var (
	safeContentTypePrefixes = []string{"image/", "audio/", "video/", "font/"}
	safeContentTypes        = collections.NewHashSetFrom(
		"application/pdf",
		"application/zip",
		"application/gzip",
		"application/x-tar",
		"application/octet-stream",
	)
)

// sanitizeContentType returns a safe MIME type for storage. Client-
// supplied values are accepted only when they fall within a known-safe
// set (binary, image, audio, video, font, common archives). Everything
// else is overridden by extension-based detection or falls back to
// application/octet-stream — this prevents stored XSS via text/html or
// application/javascript content types served from the same origin.
func sanitizeContentType(clientCT, filename string) string {
	if isSafeContentType(clientCT) {
		return clientCT
	}

	if detected := mime.TypeByExtension(filepath.Ext(filename)); detected != "" && isSafeContentType(detected) {
		return detected
	}

	return "application/octet-stream"
}

func isSafeContentType(ct string) bool {
	if ct == "" {
		return false
	}

	for _, prefix := range safeContentTypePrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}

	return safeContentTypes.Contains(ct)
}

// ensureOwner asserts that the caller's principal owns the claim.
// All storage RPC actions require authentication; this is defense in
// depth for the per-claim ownership boundary.
func ensureOwner(claim *store.UploadClaim, principal *security.Principal) error {
	if principal == nil || principal.ID == "" || principal.ID != claim.CreatedBy {
		return result.ErrAccessDenied
	}

	return nil
}

// validateUploadInput enforces the server-side cap on object size
// configured in StorageConfig. Called by InitUpload before any backend
// or DB side effect.
//
// Backend user-metadata is intentionally not part of this validation:
// the HTTP API no longer accepts user-supplied metadata at all
// (Metadata is a programmatic concern, set by trusted server-side
// callers through storage.Service directly).
func (r *Resource) validateUploadInput(size int64) error {
	if maxSize := r.cfg.EffectiveMaxUploadSize(); maxSize > 0 && size > maxSize {
		return result.Err(i18n.T("upload_size_exceeds_limit"))
	}

	return nil
}

func NewResource(
	db orm.DB,
	service storage.Service,
	claimStore store.ClaimStore,
	partStore store.UploadPartStore,
	cfg *config.StorageConfig,
) api.Resource {
	r := &Resource{
		db:         db,
		service:    service,
		claimStore: claimStore,
		partStore:  partStore,
		cfg:        cfg,
		Resource: api.NewRPCResource(
			"sys/storage",
			api.WithOperations(
				api.OperationSpec{Action: "init_upload"},
				api.OperationSpec{Action: "upload_part"},
				api.OperationSpec{Action: "complete_upload"},
				api.OperationSpec{Action: "abort_upload"},
			),
		),
	}

	// Optional capability detection: storage.Multipart is an interface
	// backends opt into by implementing. Resource never calls multipart
	// methods through storage.Service — it dispatches through this typed
	// handle, so any code path that touches them is gated on a nil check
	// the compiler can reason about.
	if mp, ok := service.(storage.Multipart); ok {
		r.multipart = mp
	}

	return r
}

type Resource struct {
	api.Resource

	db         orm.DB
	service    storage.Service
	multipart  storage.Multipart // nil when the backend does not implement chunked uploads
	claimStore store.ClaimStore
	partStore  store.UploadPartStore
	cfg        *config.StorageConfig
}

// generateObjectKey returns a date-partitioned key under the visibility
// prefix (pub/ or priv/) for organization and conflict avoidance.
func (*Resource) generateObjectKey(filename string, public bool) string {
	datePath := time.Now().Format(templateDatePath)
	uuid := id.GenerateUUID()

	ext := filepath.Ext(filename)
	if ext == "" || !safeExtPattern.MatchString(ext) {
		ext = defaultExtension
	}

	prefix := storage.PrivatePrefix
	if public {
		prefix = storage.PublicPrefix
	}

	return prefix + datePath + "/" + uuid + ext
}

// ── init_upload ─────────────────────────────────────────────────────────

// InitUploadParams declares an upload intent. Every upload goes through
// the chunked protocol — small files simply end up with PartCount=1.
// Size is required so the framework can compute the part plan and
// validate against the configured upload cap before opening any backend
// session. ContentType is persisted onto the final object; Public
// controls the key prefix.
type InitUploadParams struct {
	api.P

	Filename    string `json:"filename"    validate:"required,max=255"`
	Size        int64  `json:"size"        validate:"required,min=1"`
	ContentType string `json:"contentType" validate:"max=127"`
	Public      bool   `json:"public"`
}

// InitUploadResult tells the client how to deliver the parts. The client
// uploads each part via the upload_part action (multipart/form-data
// proxied through the framework) and finalizes with complete_upload.
// OriginalFilename is the client-supplied filename echoed back; the
// framework persists it on the claim row, not in backend user-metadata,
// so callers can rely on it independent of the storage backend.
type InitUploadResult struct {
	Key              string    `json:"key"`
	ClaimID          string    `json:"claimId"`
	UploadID         string    `json:"uploadId"`
	OriginalFilename string    `json:"originalFilename"`
	PartSize         int64     `json:"partSize"`
	PartCount        int       `json:"partCount"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

// InitUpload opens an upload session. Every upload — including small
// files that end up with a single part — flows through the same
// init → upload_part → complete protocol; this keeps the client and
// server logic uniform regardless of file size.
//
// The flow:
//
//  1. validate the declared size against the configured cap;
//  2. compute the part plan from the backend's authoritative PartSize;
//  3. INSERT the claim (status='pending') first so a backend failure
//     leaves no orphan multipart session;
//  4. open the backend multipart session and bind its UploadID to the
//     claim row (best-effort cleanup on failure).
//
// If the backend does not implement Multipart, init_upload is rejected
// outright — there is no fallback path in the unified protocol.
func (r *Resource) InitUpload(ctx fiber.Ctx, principal *security.Principal, params InitUploadParams) error {
	if err := r.validateUploadInput(params.Size); err != nil {
		return err
	}

	if r.multipart == nil {
		// Defensive: unreachable with current backends (all implement Multipart).
		return result.Err(i18n.T("multipart_not_supported"))
	}

	if params.Public && !r.cfg.AllowPublicUploads {
		return result.Err(i18n.T("public_uploads_not_allowed"))
	}

	partSize := r.multipart.PartSize()
	partCount := int((params.Size + partSize - 1) / partSize)

	if maxParts := r.multipart.MaxPartCount(); maxParts > 0 && partCount > maxParts {
		return result.Err(i18n.T("upload_too_many_parts"))
	}

	contentType := sanitizeContentType(params.ContentType, params.Filename)

	// Enforce per-principal in-flight session cap.
	owner := principal.ID

	pendingCount, err := r.claimStore.CountPendingByOwner(ctx.Context(), owner)
	if err != nil {
		return err
	}

	if pendingCount >= r.cfg.EffectiveMaxPendingClaims() {
		return result.Err(i18n.T("too_many_pending_uploads"))
	}

	key := r.generateObjectKey(params.Filename, params.Public)

	claim := &store.UploadClaim{
		ID:               id.GenerateUUID(),
		Key:              key,
		Size:             params.Size,
		ContentType:      contentType,
		OriginalFilename: params.Filename,
		Public:           params.Public,
		Status:           store.ClaimStatusPending,
		PartSize:         partSize,
		PartCount:        partCount,
		CreatedBy:        owner,
		ExpiresAt:        timex.DateTime(time.Now().Add(r.cfg.EffectiveClaimTTL())),
		CreatedAt:        timex.Now(),
	}
	if err := r.claimStore.Create(ctx.Context(), claim); err != nil {
		return err
	}

	session, err := r.multipart.InitMultipart(ctx.Context(), storage.InitMultipartOptions{
		Key:         key,
		ContentType: contentType,
	})
	if err != nil {
		// Best-effort cleanup so the sweeper does not see a stale row;
		// if this delete also fails the sweeper will still handle it on
		// TTL expiry.
		if delErr := r.claimStore.DeleteByID(ctx.Context(), claim.ID); delErr != nil {
			logger.Warnf("Delete claim %s after multipart init failure: %v", claim.ID, delErr)
		}

		return err
	}

	if updateErr := r.claimStore.UpdateUploadID(ctx.Context(), claim.ID, session.UploadID); updateErr != nil {
		// The claim row UPDATE failed after the backend session opened.
		// Abort the orphan session before surfacing the error so the
		// backend does not retain dangling parts. AbortMultipart is
		// idempotent so it is safe to call even if the session never
		// actually accepted parts.
		if abortErr := r.multipart.AbortMultipart(ctx.Context(), storage.AbortMultipartOptions{
			Key:      claim.Key,
			UploadID: session.UploadID,
		}); abortErr != nil {
			logger.Warnf("Abort orphan multipart session %s after claim UPDATE failure: %v", session.UploadID, abortErr)
		}

		return updateErr
	}

	claim.UploadID = session.UploadID

	return result.Ok(InitUploadResult{
		Key:              claim.Key,
		ClaimID:          claim.ID,
		UploadID:         claim.UploadID,
		OriginalFilename: claim.OriginalFilename,
		PartSize:         partSize,
		PartCount:        partCount,
		ExpiresAt:        time.Time(claim.ExpiresAt),
	}).Response(ctx)
}

// ── upload_part ─────────────────────────────────────────────────────────

// UploadPartParams accepts multipart/form-data carrying a single part of
// an in-progress chunked upload. ClaimID and PartNumber identify which
// slot of which session the bytes belong to; the file payload is
// streamed through to the backend's PutPart.
type UploadPartParams struct {
	api.P

	File *multipart.FileHeader

	ClaimID    string `json:"claimId"    validate:"required"`
	PartNumber int    `json:"partNumber" validate:"required,min=1"`
}

// UploadPartResult echoes the part position and recorded byte count. The
// backend ETag is intentionally NOT returned to the client: it is
// persisted server-side on the upload_part row so complete_upload can
// reconstruct the parts list without trusting client state.
type UploadPartResult struct {
	PartNumber int   `json:"partNumber"`
	Size       int64 `json:"size"`
}

// UploadPart proxies a single multipart part through the framework to
// the backend. The handler validates ownership, the claim's pending
// status, and the part-number range before opening the backend stream;
// a successful PutPart is then mirrored to the upload_part table so
// complete_upload can drive the assemble step from the database
// (clients never round-trip ETags themselves).
func (r *Resource) UploadPart(ctx fiber.Ctx, principal *security.Principal, params UploadPartParams) error {
	if httpx.IsJSON(ctx) {
		return result.Err(i18n.T("upload_requires_multipart"))
	}

	if params.File == nil {
		return result.Err(i18n.T("upload_requires_file"))
	}

	if r.multipart == nil {
		return result.Err(i18n.T("multipart_not_supported"))
	}

	claim, err := r.claimStore.Get(ctx.Context(), params.ClaimID)
	if err != nil {
		return err
	}

	if err := ensureOwner(claim, principal); err != nil {
		return err
	}

	if claim.Status != store.ClaimStatusPending {
		return result.Err(i18n.T("claim_not_pending"))
	}

	if !claim.IsMultipart() {
		return result.Err(i18n.T("claim_not_multipart"))
	}

	if params.PartNumber < 1 || params.PartNumber > claim.PartCount {
		return result.Err(i18n.T("part_number_out_of_range"))
	}

	file, err := params.File.Open()
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Errorf("Close uploaded part file failed: %v", closeErr)
		}
	}()

	if params.File.Size > claim.PartSize {
		return result.Err(i18n.T("upload_part_too_large"))
	}

	if params.PartNumber < claim.PartCount && params.File.Size < claim.PartSize {
		return result.Err(i18n.T("upload_part_too_small"))
	}

	partInfo, err := r.multipart.PutPart(ctx.Context(), storage.PutPartOptions{
		Key:        claim.Key,
		UploadID:   claim.UploadID,
		PartNumber: params.PartNumber,
		Reader:     file,
		Size:       params.File.Size,
	})
	if err != nil {
		return err
	}

	// Persist the (claim_id, part_number) → ETag mapping so
	// complete_upload can assemble the parts list from the database
	// without trusting the client. Upsert handles re-uploads of the
	// same part number — the latest ETag wins, matching the backend's
	// last-writer semantics.
	if err := r.db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
		return r.partStore.Upsert(txCtx, tx, &store.UploadPart{
			ID:         id.GenerateUUID(),
			ClaimID:    claim.ID,
			PartNumber: partInfo.PartNumber,
			ETag:       partInfo.ETag,
			Size:       partInfo.Size,
			CreatedAt:  timex.Now(),
		})
	}); err != nil {
		return err
	}

	return result.Ok(UploadPartResult{
		PartNumber: partInfo.PartNumber,
		Size:       partInfo.Size,
	}).Response(ctx)
}

// ── complete_upload ─────────────────────────────────────────────────────

type CompleteUploadParams struct {
	api.P

	ClaimID string `json:"claimId" validate:"required"`
}

// CompleteUploadResult bundles the backend ObjectInfo with the
// framework-tracked OriginalFilename. Returning a wrapper rather than
// the bare ObjectInfo keeps backend abstractions clean of framework
// concepts while still giving callers a single response shape that
// covers everything they need to render the upload.
type CompleteUploadResult struct {
	storage.ObjectInfo

	OriginalFilename string `json:"originalFilename"`
}

// CompleteUpload finalizes a chunked upload. The handler reads the
// recorded parts from the database (clients never replay ETags
// themselves), verifies the part count matches the original plan,
// instructs the backend to assemble the object, then atomically marks
// the claim 'uploaded' and clears its part rows.
//
// Idempotency: a retried complete_upload that arrives after the
// backend session is already closed surfaces ErrUploadSessionNotFound;
// the handler then re-stats the object to confirm it exists and
// commits the same MarkUploaded + DeleteByClaim transaction so the
// claim still ends in a consumable state.
func (r *Resource) CompleteUpload(ctx fiber.Ctx, principal *security.Principal, params CompleteUploadParams) error {
	claim, err := r.claimStore.Get(ctx.Context(), params.ClaimID)
	if err != nil {
		return err
	}

	if err := ensureOwner(claim, principal); err != nil {
		return err
	}

	// True idempotency — if already uploaded, return the result
	// without re-assembling. This covers client retries after a
	// successful CompleteMultipart whose response was lost in transit.
	if claim.Status == store.ClaimStatusUploaded {
		info, statErr := r.service.StatObject(ctx.Context(), storage.StatObjectOptions{Key: claim.Key})
		if statErr != nil {
			return statErr
		}

		return result.Ok(CompleteUploadResult{
			ObjectInfo:       *info,
			OriginalFilename: claim.OriginalFilename,
		}).Response(ctx)
	}

	if claim.Status != store.ClaimStatusPending {
		return result.Err(i18n.T("claim_not_pending"))
	}

	// Reject expired claims to prevent race with ClaimSweeper.
	if time.Time(claim.ExpiresAt).Before(time.Now()) {
		return result.Err(i18n.T("claim_expired"))
	}

	if !claim.IsMultipart() {
		return result.Err(i18n.T("claim_not_multipart"))
	}

	if r.multipart == nil {
		// Defensive: unreachable with current backends (all implement Multipart).
		return result.Err(i18n.T("multipart_not_supported"))
	}

	parts, err := r.partStore.ListByClaim(ctx.Context(), claim.ID)
	if err != nil {
		return err
	}

	if len(parts) != claim.PartCount {
		return result.Err(i18n.T("upload_parts_incomplete"))
	}

	completed := make([]storage.CompletedPart, len(parts))
	for i := range parts {
		completed[i] = storage.CompletedPart{
			PartNumber: parts[i].PartNumber,
			ETag:       parts[i].ETag,
		}
	}

	info, err := r.multipart.CompleteMultipart(ctx.Context(), storage.CompleteMultipartOptions{
		Key:      claim.Key,
		UploadID: claim.UploadID,
		Parts:    completed,
	})
	if err != nil {
		// Idempotent retry: a previous complete_upload may have
		// finalized the object and closed the session. Confirm via
		// StatObject and proceed with the bookkeeping commit so the
		// claim ends in 'uploaded' state for business consumption.
		if !errors.Is(err, storage.ErrUploadSessionNotFound) {
			return err
		}

		stat, statErr := r.service.StatObject(ctx.Context(), storage.StatObjectOptions{Key: claim.Key})
		if statErr != nil {
			if errors.Is(statErr, storage.ErrObjectNotFound) {
				return result.Err(i18n.T("object_not_found"))
			}

			return statErr
		}

		info = stat
	}

	if claim.Size > 0 && info.Size != claim.Size {
		// Object already assembled but size doesn't match declaration;
		// clean up immediately rather than waiting for sweeper TTL.
		if delErr := r.service.DeleteObject(ctx.Context(), storage.DeleteObjectOptions{Key: claim.Key}); delErr != nil && !errors.Is(delErr, storage.ErrObjectNotFound) {
			logger.Warnf("Delete object %s after size mismatch failed: %v (relying on sweeper for cleanup)", claim.Key, delErr)
		}

		return result.Err(i18n.T("upload_size_mismatch"))
	}

	if err := r.db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
		if err := r.claimStore.MarkUploaded(txCtx, tx, claim.ID); err != nil {
			return err
		}

		return r.partStore.DeleteByClaim(txCtx, tx, claim.ID)
	}); err != nil {
		return err
	}

	return result.Ok(CompleteUploadResult{
		ObjectInfo:       *info,
		OriginalFilename: claim.OriginalFilename,
	}).Response(ctx)
}

// ── abort_upload ────────────────────────────────────────────────────────

type AbortUploadParams struct {
	api.P

	ClaimID string `json:"claimId" validate:"required"`
}

// AbortUpload cancels an in-flight upload. The handler aborts the
// backend multipart session, deletes any object bytes the backend may
// have published, then in a single transaction drops the part rows
// and the claim row. AbortMultipart and DeleteObject are both treated
// idempotently — retrying abort_upload on a partially-cleaned state
// still ends with the claim row removed.
func (r *Resource) AbortUpload(ctx fiber.Ctx, principal *security.Principal, params AbortUploadParams) error {
	claim, err := r.claimStore.Get(ctx.Context(), params.ClaimID)
	if err != nil {
		if errors.Is(err, storage.ErrClaimNotFound) {
			return result.Ok().Response(ctx)
		}

		return err
	}

	if err := ensureOwner(claim, principal); err != nil {
		return err
	}

	if claim.IsMultipart() && r.multipart != nil {
		// AbortMultipart is idempotent — calling it on an unknown or
		// already-closed session is a no-op. When the backend has been
		// swapped (r.multipart == nil) the multipart session is
		// unreachable through this service anyway; fall through to the
		// object delete + claim cleanup and rely on the operator-
		// configured S3 lifecycle policy to reap any orphan sessions.
		if abortErr := r.multipart.AbortMultipart(ctx.Context(), storage.AbortMultipartOptions{
			Key:      claim.Key,
			UploadID: claim.UploadID,
		}); abortErr != nil {
			logger.Errorf("AbortMultipart failed for claim %s: %v", claim.ID, abortErr)

			return result.Err(i18n.T("abort_failed"))
		}
	}

	if delErr := r.service.DeleteObject(ctx.Context(), storage.DeleteObjectOptions{Key: claim.Key}); delErr != nil && !errors.Is(delErr, storage.ErrObjectNotFound) {
		return delErr
	}

	if err := r.db.RunInTX(ctx.Context(), func(txCtx context.Context, tx orm.DB) error {
		if err := r.partStore.DeleteByClaim(txCtx, tx, claim.ID); err != nil {
			return err
		}

		return r.claimStore.DeleteByIDInTx(txCtx, tx, claim.ID)
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}
