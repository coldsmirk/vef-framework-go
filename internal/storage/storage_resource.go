package storage

import (
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
)

const (
	templateDatePath = "2006/01/02"
	defaultExtension = ".bin"
)

func NewResource(service storage.Service) api.Resource {
	return &Resource{
		service: service,
		Resource: api.NewRPCResource(
			"sys/storage",
			api.WithOperations(
				api.OperationSpec{Action: "upload"},
				api.OperationSpec{Action: "get_presigned_url"},
				api.OperationSpec{Action: "delete_temp"},
				api.OperationSpec{Action: "stat"},
				api.OperationSpec{Action: "list"},
			),
		),
	}
}

type Resource struct {
	api.Resource

	service storage.Service
}

type UploadParams struct {
	api.P

	File *multipart.FileHeader

	ContentType string            `json:"contentType"`
	Metadata    map[string]string `json:"metadata"`
}

// Upload generates date-partitioned keys (temp/YYYY/MM/DD/{uuid}{ext}) to organize uploads and avoid conflicts.
func (r *Resource) Upload(ctx fiber.Ctx, params UploadParams) error {
	if httpx.IsJSON(ctx) {
		return result.Err(i18n.T("upload_requires_multipart"))
	}

	if params.File == nil {
		return result.Err(i18n.T("upload_requires_file"))
	}

	key := r.generateObjectKey(params.File.Filename)

	file, err := params.File.Open()
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Errorf("failed to close file: %v", closeErr)
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
		Key:         key,
		Reader:      file,
		Size:        params.File.Size,
		ContentType: contentType,
		Metadata:    metadata,
	})
	if err != nil {
		return err
	}

	return result.Ok(info).Response(ctx)
}

func (*Resource) generateObjectKey(filename string) string {
	datePath := time.Now().Format(templateDatePath)
	uuid := id.GenerateUUID()

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = defaultExtension
	}

	return storage.TempPrefix + datePath + "/" + uuid + ext
}

type GetPresignedURLParams struct {
	api.P

	Key     string `json:"key" validate:"required"`
	Expires int    `json:"expires"`
	Method  string `json:"method"`
}

func (r *Resource) GetPresignedURL(ctx fiber.Ctx, params GetPresignedURLParams) error {
	expires := params.Expires
	if expires <= 0 {
		expires = 3600 // 1 hour default
	}

	method := params.Method
	if method == "" {
		method = http.MethodGet
	}

	url, err := r.service.GetPresignedURL(ctx.Context(), storage.PresignedURLOptions{
		Key:     params.Key,
		Expires: time.Duration(expires) * time.Second,
		Method:  method,
	})
	if err != nil {
		return err
	}

	return result.Ok(fiber.Map{"url": url}).Response(ctx)
}

type DeleteTempParams struct {
	api.P

	Key string `json:"key" validate:"required"`
}

// DeleteTemp restricts deletion to temp/ prefix to prevent accidental removal of permanent files.
func (r *Resource) DeleteTemp(ctx fiber.Ctx, params DeleteTempParams) error {
	if !strings.HasPrefix(params.Key, storage.TempPrefix) {
		return result.Err(i18n.T("invalid_temp_key"))
	}

	if err := r.service.DeleteObject(ctx.Context(), storage.DeleteObjectOptions{
		Key: params.Key,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

type DeleteParams struct {
	api.P

	Key string `json:"key" validate:"required"`
}

func (r *Resource) Delete(ctx fiber.Ctx, params DeleteParams) error {
	if err := r.service.DeleteObject(ctx.Context(), storage.DeleteObjectOptions{
		Key: params.Key,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

type DeleteManyParams struct {
	api.P

	Keys []string `json:"keys" validate:"required,min=1"`
}

func (r *Resource) DeleteMany(ctx fiber.Ctx, params DeleteManyParams) error {
	if err := r.service.DeleteObjects(ctx.Context(), storage.DeleteObjectsOptions{
		Keys: params.Keys,
	}); err != nil {
		return err
	}

	return result.Ok().Response(ctx)
}

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

type CopyParams struct {
	api.P

	SourceKey string `json:"sourceKey" validate:"required"`
	DestKey   string `json:"destKey" validate:"required"`
}

func (r *Resource) Copy(ctx fiber.Ctx, params CopyParams) error {
	info, err := r.service.CopyObject(ctx.Context(), storage.CopyObjectOptions{
		SourceKey: params.SourceKey,
		DestKey:   params.DestKey,
	})
	if err != nil {
		return err
	}

	return result.Ok(info).Response(ctx)
}

type MoveParams struct {
	api.P

	SourceKey string `json:"sourceKey" validate:"required"`
	DestKey   string `json:"destKey" validate:"required"`
}

func (r *Resource) Move(ctx fiber.Ctx, params MoveParams) error {
	info, err := r.service.MoveObject(ctx.Context(), storage.MoveObjectOptions{
		CopyObjectOptions: storage.CopyObjectOptions{
			SourceKey: params.SourceKey,
			DestKey:   params.DestKey,
		},
	})
	if err != nil {
		return err
	}

	return result.Ok(info).Response(ctx)
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
