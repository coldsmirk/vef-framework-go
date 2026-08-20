package storage

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// FileResource is the read side of the durable upload registry: it turns
// the object keys a business model stores into the metadata needed to
// render them — original filename above all.
//
// Kept apart from the upload protocol resource on purpose: this is a
// query surface over recorded state, not a step in the chunked-upload
// state machine.
type FileResource struct {
	api.Resource

	registry storage.FileRegistry
	acl      storage.FileACL
}

func NewFileResource(registry storage.FileRegistry, acl storage.FileACL) api.Resource {
	return &FileResource{
		registry: registry,
		acl:      acl,
		Resource: api.NewRPCResource(
			"sys/storage/file",
			api.WithOperations(
				api.OperationSpec{Action: "resolve"},
			),
		),
	}
}

// ── resolve ─────────────────────────────────────────────────────────────

type ResolveParams struct {
	api.P

	// A file list long enough to exceed the cap is a paging problem on
	// the client, and an unbounded IN (...) is not something an
	// authenticated caller should be able to ask for.
	Keys []string `json:"keys" validate:"required,min=1,max=200,dive,required"`
}

// ResolvedFile is the client-facing projection of a registry record.
// Deliberately narrower than storage.FileRecord: lifecycle bookkeeping
// (claimed/deleted timestamps, delete reason) is operational data, not
// something a form field needs to render a filename.
type ResolvedFile struct {
	Key              string             `json:"key"`
	OriginalFilename string             `json:"originalFilename"`
	ContentType      string             `json:"contentType"`
	Size             int64              `json:"size"`
	Status           storage.FileStatus `json:"status"`
	UploadedAt       timex.DateTime     `json:"uploadedAt"`
	UploadedBy       string             `json:"uploadedBy"`
}

type ResolveResult struct {
	Files []ResolvedFile `json:"files"`
}

// Resolve returns what the framework recorded about each supplied object
// key. Keys the caller may not read, and keys with no record, are
// omitted from the response rather than reported — the caller is
// rendering a list and must degrade to showing the bare key, and
// distinguishing "not yours" from "never existed" would leak the
// existence of other tenants' files.
//
// Authorization mirrors the download proxy exactly: pub/ keys are open,
// everything else goes through FileACL.CanRead. That is the same barrier
// the bytes themselves sit behind, so this endpoint cannot become the
// weaker door — an application that has not supplied a FileACL cannot
// serve its private files at all, and equally cannot name them here.
func (r *FileResource) Resolve(ctx fiber.Ctx, principal *security.Principal, params ResolveParams) error {
	records, err := r.registry.Lookup(ctx.Context(), params.Keys)
	if err != nil {
		return err
	}

	files := make([]ResolvedFile, 0, len(records))

	// Iterate the request order, not the map, so the response is stable
	// for the same input.
	seen := make(map[string]struct{}, len(records))

	var denied int

	for _, key := range params.Keys {
		record, ok := records[key]
		if !ok {
			continue
		}

		if _, duplicate := seen[key]; duplicate {
			continue
		}

		allowed, aclErr := r.canRead(ctx, principal, key)
		if aclErr != nil {
			return aclErr
		}

		if !allowed {
			denied++

			continue
		}

		seen[key] = struct{}{}

		files = append(files, ResolvedFile{
			Key:              record.Key,
			OriginalFilename: record.OriginalFilename,
			ContentType:      record.ContentType,
			Size:             record.Size,
			Status:           record.Status,
			UploadedAt:       record.StartedAt,
			UploadedBy:       record.CreatedBy,
		})
	}

	// Omitting denied keys silently is the right wire behavior — reporting
	// "not yours" instead of "no record" would leak the existence of other
	// tenants' files — but it also makes an ACL that denies more than its
	// author meant invisible: the client just renders bare object keys,
	// which reads as "the framework lost my filename". Since a pub/ key
	// never reaches the ACL, that failure shows up as "public files keep
	// their name, private ones don't". This line is what tells an operator
	// the ACL said no, rather than the registry having no record.
	if denied > 0 {
		logger.Debugf(
			"FileACL denied %d of %d requested key(s) for principal %q; those files resolve to no original filename",
			denied, len(params.Keys), principal.ID,
		)
	}

	return result.Ok(ResolveResult{Files: files}).Response(ctx)
}

func (r *FileResource) canRead(ctx fiber.Ctx, principal *security.Principal, key string) (bool, error) {
	if strings.HasPrefix(key, storage.PublicPrefix) {
		return true, nil
	}

	allowed, err := r.acl.CanRead(ctx.Context(), principal, key)
	if err != nil {
		logger.Errorf("FileACL.CanRead failed for key %s: %v", key, err)

		return false, storage.ErrFailedToGetFile
	}

	return allowed, nil
}
