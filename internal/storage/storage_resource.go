package storage

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const (
	templateDatePath = "2006/01/02"
	defaultExtension = ".bin"
	keyPrefixPublic  = "pub/"
	keyPrefixPrivate = "priv/"
)

// Upload protocol modes returned by init_upload.
const (
	uploadModeDirect    = "direct"
	uploadModeMultipart = "multipart"
	uploadModeProxy     = "proxy"
)

// systemOperator is the principal ID used when an upload is initiated
// without an authenticated user. Aligns with the framework's audit
// convention (created_by = 'system').
const systemOperator = "system"

func NewResource(service storage.Service, claimStore storage.ClaimStore, cfg *config.StorageConfig) api.Resource {
	return &Resource{
		service:    service,
		claimStore: claimStore,
		cfg:        cfg,
		Resource: api.NewRPCResource(
			"sys/storage",
			api.WithOperations(
				api.OperationSpec{Action: "upload"},
				api.OperationSpec{Action: "init_upload"},
				api.OperationSpec{Action: "sign_part"},
				api.OperationSpec{Action: "complete_upload"},
				api.OperationSpec{Action: "abort_upload"},
				api.OperationSpec{Action: "get_presigned_url"},
				api.OperationSpec{Action: "stat"},
				api.OperationSpec{Action: "list"},
			),
		),
	}
}

type Resource struct {
	api.Resource

	service    storage.Service
	claimStore storage.ClaimStore
	cfg        *config.StorageConfig
}

// generateObjectKey returns a date-partitioned key under the visibility
// prefix (pub/ or priv/) for organization and conflict avoidance.
func (*Resource) generateObjectKey(filename string, public bool) string {
	datePath := time.Now().Format(templateDatePath)
	uuid := id.GenerateUUID()

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = defaultExtension
	}

	prefix := keyPrefixPrivate
	if public {
		prefix = keyPrefixPublic
	}

	return prefix + datePath + "/" + uuid + ext
}

// principalID returns the upload's owner ID, defaulting to "system" for
// unauthenticated paths (which shouldn't normally happen given PermToken).
func principalID(p *security.Principal) string {
	if p == nil || p.ID == "" {
		return systemOperator
	}

	return p.ID
}

// ── upload (server-proxied fallback) ─────────────────────────────────────

// UploadParams accepts multipart/form-data. When ClaimID is set, the upload
// completes a claim previously created by init_upload (mode=proxy); when
// empty, a fresh claim is created so the same commit/abort flow applies.
type UploadParams struct {
	api.P

	File *multipart.FileHeader

	ClaimID     string            `json:"claimId"`
	ContentType string            `json:"contentType"`
	Public      bool              `json:"public"`
	Metadata    map[string]string `json:"metadata"`
}

// UploadResult bundles the storage ObjectInfo with the claim handle the
// business layer needs to commit the upload.
type UploadResult struct {
	*storage.ObjectInfo

	ClaimID   string    `json:"claimId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Upload accepts a file via multipart/form-data and stores it server-side
// (bytes flow through the application). For browser clients that can do
// CORS-enabled direct upload, prefer init_upload + presigned PUT.
func (r *Resource) Upload(ctx fiber.Ctx, principal *security.Principal, params UploadParams) error {
	if httpx.IsJSON(ctx) {
		return result.Err(i18n.T("upload_requires_multipart"))
	}

	if params.File == nil {
		return result.Err(i18n.T("upload_requires_file"))
	}

	claim, err := r.resolveUploadClaim(ctx.Context(), principal, &params)
	if err != nil {
		return err
	}

	file, err := params.File.Open()
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Errorf("Close uploaded file failed: %v", closeErr)
		}
	}()

	contentType := params.ContentType
	if contentType == "" {
		contentType = params.File.Header.Get(fiber.HeaderContentType)
	}

	metadata := params.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}

	metadata[storage.MetadataKeyOriginalFilename] = params.File.Filename

	info, err := r.service.PutObject(ctx.Context(), storage.PutObjectOptions{
		Key:         claim.Key,
		Reader:      file,
		Size:        params.File.Size,
		ContentType: contentType,
		Metadata:    metadata,
	})
	if err != nil {
		return err
	}

	return result.Ok(UploadResult{
		ObjectInfo: info,
		ClaimID:    claim.ID,
		ExpiresAt:  time.Time(claim.ExpiresAt),
	}).Response(ctx)
}

// resolveUploadClaim either fetches an existing claim by ID (init_upload
// proxy path) or creates a fresh one for a standalone server-proxied upload.
func (r *Resource) resolveUploadClaim(ctx context.Context, principal *security.Principal, params *UploadParams) (*storage.UploadClaim, error) {
	if params.ClaimID != "" {
		return r.claimStore.Get(ctx, params.ClaimID)
	}

	key := r.generateObjectKey(params.File.Filename, params.Public)

	contentType := params.ContentType
	if contentType == "" {
		contentType = params.File.Header.Get(fiber.HeaderContentType)
	}

	claim := &storage.UploadClaim{
		ID:          id.GenerateUUID(),
		Key:         key,
		Size:        params.File.Size,
		ContentType: contentType,
		Metadata:    params.Metadata,
		CreatedBy:   principalID(principal),
		ExpiresAt:   timex.DateTime(time.Now().Add(r.cfg.EffectiveClaimTTL())),
		CreatedAt:   timex.Now(),
	}
	if err := r.claimStore.Create(ctx, claim); err != nil {
		return nil, err
	}

	return claim, nil
}

// ── init_upload ─────────────────────────────────────────────────────────

// InitUploadParams declares an upload intent. Size hints the mode dispatch
// (multipart vs. direct); ContentType and Metadata are persisted onto the
// final object. Public controls the key prefix.
type InitUploadParams struct {
	api.P

	Filename    string            `json:"filename" validate:"required"`
	Size        int64             `json:"size"`
	ContentType string            `json:"contentType"`
	Public      bool              `json:"public"`
	Metadata    map[string]string `json:"metadata"`
}

// InitUploadResult tells the client how to deliver the bytes. Only the
// fields relevant to the chosen Mode are populated.
type InitUploadResult struct {
	Mode      string    `json:"mode"`
	Key       string    `json:"key"`
	ClaimID   string    `json:"claimId"`
	ExpiresAt time.Time `json:"expiresAt"`

	// Direct mode: single presigned PUT URL.
	UploadURL       string            `json:"uploadUrl,omitempty"`
	RequiredHeaders map[string]string `json:"requiredHeaders,omitempty"`

	// Multipart mode: opaque session token plus the chunking plan.
	UploadID  string `json:"uploadId,omitempty"`
	PartSize  int64  `json:"partSize,omitempty"`
	PartCount int    `json:"partCount,omitempty"`
}

// InitUpload picks an upload protocol based on the declared size and the
// backend's reported Capabilities, optionally opens a multipart session on
// the backend, and persists an UploadClaim so the worker can clean up if
// the client never completes the upload.
func (r *Resource) InitUpload(ctx fiber.Ctx, principal *security.Principal, params InitUploadParams) error {
	mode, partSize, partCount := r.selectUploadMode(params.Size)

	key := r.generateObjectKey(params.Filename, params.Public)

	contentType := params.ContentType
	if contentType == "" {
		contentType = http.DetectContentType(nil) // empty body → application/octet-stream
	}

	// Open a multipart session up front so the claim carries the upload_id.
	var uploadID string

	if mode == uploadModeMultipart {
		session, err := r.service.InitMultipart(ctx.Context(), storage.InitMultipartOptions{
			Key:         key,
			ContentType: contentType,
			Metadata:    params.Metadata,
		})
		if err != nil {
			if errors.Is(err, storage.ErrCapabilityNotSupported) {
				// Capabilities() lied or changed; degrade gracefully.
				mode, partSize, partCount = r.degradeFromMultipart(params.Size)
			} else {
				return err
			}
		} else {
			uploadID = session.UploadID
		}
	}

	claim := &storage.UploadClaim{
		ID:          id.GenerateUUID(),
		Key:         key,
		UploadID:    uploadID,
		Size:        params.Size,
		ContentType: contentType,
		Metadata:    params.Metadata,
		CreatedBy:   principalID(principal),
		ExpiresAt:   timex.DateTime(time.Now().Add(r.cfg.EffectiveClaimTTL())),
		CreatedAt:   timex.Now(),
	}
	if err := r.claimStore.Create(ctx.Context(), claim); err != nil {
		// Best-effort: abandon any multipart session we just opened.
		if uploadID != "" {
			_ = r.service.AbortMultipart(ctx.Context(), storage.AbortMultipartOptions{
				Key:      key,
				UploadID: uploadID,
			})
		}

		return err
	}

	out := InitUploadResult{
		Mode:      mode,
		Key:       key,
		ClaimID:   claim.ID,
		ExpiresAt: time.Time(claim.ExpiresAt),
		UploadID:  uploadID,
		PartSize:  partSize,
		PartCount: partCount,
	}

	if mode == uploadModeDirect {
		presigned, err := r.service.PresignPutObject(ctx.Context(), storage.PresignPutOptions{
			Key:         key,
			ContentType: contentType,
			Expires:     r.cfg.EffectivePresignedTTL(),
		})
		if err != nil {
			if errors.Is(err, storage.ErrCapabilityNotSupported) {
				out.Mode = uploadModeProxy
			} else {
				return err
			}
		} else {
			out.UploadURL = presigned.URL
			out.RequiredHeaders = presigned.Headers
		}
	}

	return result.Ok(out).Response(ctx)
}

// selectUploadMode dispatches based on caps + declared size.
func (r *Resource) selectUploadMode(size int64) (string, int64, int) {
	caps := r.service.Capabilities()

	threshold := r.cfg.EffectiveMultipartThreshold()

	if size > threshold && caps.Multipart && caps.PresignedPart {
		partSize := r.cfg.EffectivePartSize()
		if caps.MinPartSize > partSize {
			partSize = caps.MinPartSize
		}

		partCount := 0
		if size > 0 {
			partCount = int((size + partSize - 1) / partSize)
		}

		return uploadModeMultipart, partSize, partCount
	}

	if caps.PresignedPut {
		return uploadModeDirect, 0, 0
	}

	return uploadModeProxy, 0, 0
}

// degradeFromMultipart picks a fallback when the backend refuses multipart
// at runtime even though Capabilities advertised it.
func (r *Resource) degradeFromMultipart(_ int64) (string, int64, int) {
	caps := r.service.Capabilities()

	if caps.PresignedPut {
		return uploadModeDirect, 0, 0
	}

	return uploadModeProxy, 0, 0
}

// ── sign_part ───────────────────────────────────────────────────────────

type SignPartParams struct {
	api.P

	ClaimID    string `json:"claimId"    validate:"required"`
	PartNumber int    `json:"partNumber" validate:"required,min=1"`
}

// SignPart returns a fresh presigned PUT URL for a single multipart part.
// The client calls this for each part; the server does not pre-sign all
// parts at init time to avoid token expiry on long uploads.
func (r *Resource) SignPart(ctx fiber.Ctx, params SignPartParams) error {
	claim, err := r.claimStore.Get(ctx.Context(), params.ClaimID)
	if err != nil {
		return err
	}

	if !claim.IsMultipart() {
		return result.Err(i18n.T("claim_not_multipart"))
	}

	presigned, err := r.service.PresignPart(ctx.Context(), storage.PresignPartOptions{
		Key:        claim.Key,
		UploadID:   claim.UploadID,
		PartNumber: params.PartNumber,
		Expires:    r.cfg.EffectivePresignedTTL(),
	})
	if err != nil {
		return err
	}

	return result.Ok(presigned).Response(ctx)
}

// ── complete_upload ─────────────────────────────────────────────────────

type CompleteUploadParams struct {
	api.P

	ClaimID string                  `json:"claimId"        validate:"required"`
	Parts   []storage.CompletedPart `json:"parts,omitempty"`
}

// CompleteUpload finalizes an upload. For multipart claims it instructs the
// backend to assemble parts; for direct/proxy claims it verifies the object
// exists. In both cases the claim row remains until the business layer
// consumes it (or until TTL expiry).
func (r *Resource) CompleteUpload(ctx fiber.Ctx, params CompleteUploadParams) error {
	claim, err := r.claimStore.Get(ctx.Context(), params.ClaimID)
	if err != nil {
		return err
	}

	var info *storage.ObjectInfo

	if claim.IsMultipart() {
		info, err = r.service.CompleteMultipart(ctx.Context(), storage.CompleteMultipartOptions{
			Key:      claim.Key,
			UploadID: claim.UploadID,
			Parts:    params.Parts,
		})
	} else {
		info, err = r.service.StatObject(ctx.Context(), storage.StatObjectOptions{Key: claim.Key})
	}

	if err != nil {
		return err
	}

	return result.Ok(info).Response(ctx)
}

// ── abort_upload ────────────────────────────────────────────────────────

type AbortUploadParams struct {
	api.P

	ClaimID string `json:"claimId" validate:"required"`
}

// AbortUpload cancels an in-flight upload, releasing any partial parts on
// the backend, deleting the (possibly partial) object, and removing the
// claim row. Idempotent on missing claims.
func (r *Resource) AbortUpload(ctx fiber.Ctx, params AbortUploadParams) error {
	claim, err := r.claimStore.Get(ctx.Context(), params.ClaimID)
	if err != nil {
		if errors.Is(err, storage.ErrClaimNotFound) {
			return result.Ok().Response(ctx)
		}

		return err
	}

	if claim.IsMultipart() {
		if abortErr := r.service.AbortMultipart(ctx.Context(), storage.AbortMultipartOptions{
			Key:      claim.Key,
			UploadID: claim.UploadID,
		}); abortErr != nil && !errors.Is(abortErr, storage.ErrCapabilityNotSupported) {
			// Best-effort: log and continue; the claim sweeper will retry
			// after the row's TTL if we leave it in place.
			logger.Warnf("Abort multipart for claim %s failed: %v", claim.ID, abortErr)
		}
	}

	if delErr := r.service.DeleteObject(ctx.Context(), storage.DeleteObjectOptions{Key: claim.Key}); delErr != nil && !errors.Is(delErr, storage.ErrObjectNotFound) {
		return delErr
	}

	if err := r.claimStore.DeleteByID(ctx.Context(), claim.ID); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

// ── get_presigned_url (read) ────────────────────────────────────────────

type GetPresignedURLParams struct {
	api.P

	Key     string `json:"key"     validate:"required"`
	Expires int    `json:"expires"`
	Method  string `json:"method"`
}

func (r *Resource) GetPresignedURL(ctx fiber.Ctx, params GetPresignedURLParams) error {
	expires := time.Duration(params.Expires) * time.Second
	if expires <= 0 {
		expires = r.cfg.EffectivePresignedReadTTL()
	}

	method := params.Method
	if method == "" {
		method = http.MethodGet
	}

	url, err := r.service.GetPresignedURL(ctx.Context(), storage.PresignedURLOptions{
		Key:     params.Key,
		Expires: expires,
		Method:  method,
	})
	if err != nil {
		return err
	}

	return result.Ok(fiber.Map{"url": url}).Response(ctx)
}

// ── stat / list ─────────────────────────────────────────────────────────

type ListParams struct {
	api.P

	Prefix    string `json:"prefix"`
	Recursive bool   `json:"recursive"`
	MaxKeys   int    `json:"maxKeys"`
}

func (r *Resource) List(ctx fiber.Ctx, params ListParams) error {
	objects, err := r.service.ListObjects(ctx.Context(), storage.ListObjectsOptions{
		Prefix:    params.Prefix,
		Recursive: params.Recursive,
		MaxKeys:   params.MaxKeys,
	})
	if err != nil {
		return err
	}

	return result.Ok(objects).Response(ctx)
}

type StatParams struct {
	api.P

	Key string `json:"key" validate:"required"`
}

func (r *Resource) Stat(ctx fiber.Ctx, params StatParams) error {
	info, err := r.service.StatObject(ctx.Context(), storage.StatObjectOptions{
		Key: params.Key,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return result.Err(i18n.T("object_not_found"))
		}

		return err
	}

	return result.Ok(info).Response(ctx)
}
