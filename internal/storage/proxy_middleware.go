package storage

import (
	"errors"
	"mime"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
)

type ProxyMiddleware struct {
	service storage.Service
	acl     storage.FileACL
}

func (*ProxyMiddleware) Name() string {
	return "storage_proxy"
}

func (*ProxyMiddleware) Order() int {
	return 900
}

func (p *ProxyMiddleware) Apply(router fiber.Router) {
	router.Get("/storage/files/+", p.handleFileProxy)
}

func (p *ProxyMiddleware) handleFileProxy(ctx fiber.Ctx) error {
	// fiber v3 Params returns the raw URI path segment without
	// percent-decoding; unescape once to get the actual object key.
	key, err := url.PathUnescape(ctx.Params("+"))
	if err != nil {
		return result.Err(
			i18n.T(result.ErrMessageInvalidFileKey),
			result.WithCode(result.ErrCodeInvalidFileKey),
		)
	}

	// Reject path traversal, absolute paths, and control characters.
	if !isValidObjectKey(key) {
		return result.Err(
			i18n.T(result.ErrMessageInvalidFileKey),
			result.WithCode(result.ErrCodeInvalidFileKey),
		)
	}

	// pub/* is world-readable by design (bucket policy + CDN caching);
	// skip the ACL call entirely for performance and to allow anonymous
	// access without requiring an auth token on the request.
	if !strings.HasPrefix(key, storage.PublicPrefix) {
		principal := contextx.Principal(ctx)

		allowed, aclErr := p.acl.CanRead(ctx.Context(), principal, key)
		if aclErr != nil {
			logger.Errorf("FileACL.CanRead failed for key %s: %v", key, aclErr)

			return result.Err(i18n.T(result.ErrMessageFailedToGetFile))
		}

		if !allowed {
			return result.ErrAccessDenied
		}
	}

	reader, err := p.service.GetObject(ctx.Context(), storage.GetObjectOptions{
		Key: key,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return result.Err(
				i18n.T(result.ErrMessageFileNotFound),
				result.WithCode(result.ErrCodeFileNotFound),
			)
		}

		logger.Errorf("Failed to get object %s: %v", key, err)

		return result.Err(i18n.T(result.ErrMessageFailedToGetFile))
	}

	stat, err := p.service.StatObject(ctx.Context(), storage.StatObjectOptions{
		Key: key,
	})
	if err != nil {
		logger.Warnf("Failed to stat object %s: %v", key, err)
	}

	contentType := detectContentType(stat, key)
	ctx.Set(fiber.HeaderContentType, contentType)
	ctx.Set("X-Content-Type-Options", "nosniff")

	if stat != nil {
		ctx.Set(fiber.HeaderContentLength, strconv.FormatInt(stat.Size, 10))
	}

	// pub/* is safe to cache publicly (CDN, browser); priv/* must never
	// be stored in shared caches — the response is per-principal.
	// Keys contain UUIDs so immutable is safe for CDN hit rate.
	if strings.HasPrefix(key, storage.PublicPrefix) {
		ctx.Set(fiber.HeaderCacheControl, "public, max-age=3600, immutable")

		if stat != nil && stat.ETag != "" {
			ctx.Set(fiber.HeaderETag, stat.ETag)
		}
	} else {
		ctx.Set(fiber.HeaderCacheControl, "private, no-store")
		// Do NOT send ETag for private files — prevents cross-user
		// content fingerprinting via conditional requests.
	}

	return ctx.SendStream(reader)
}

func NewProxyMiddleware(service storage.Service, acl storage.FileACL) app.Middleware {
	return &ProxyMiddleware{
		service: service,
		acl:     acl,
	}
}

// isValidObjectKey rejects keys that could cause path traversal or
// other filesystem-level exploits. A valid key:
//   - is non-empty
//   - does not start with "/" (absolute path)
//   - contains no ".." path segments
//   - equals its path.Clean form (no redundant slashes, no trailing /)
//   - contains no NUL bytes or backslashes
func isValidObjectKey(key string) bool {
	if key == "" {
		return false
	}

	if key[0] == '/' || strings.ContainsAny(key, "\x00\\") {
		return false
	}

	if strings.Contains(key, "..") {
		return false
	}

	if path.Clean(key) != key {
		return false
	}

	return true
}

func detectContentType(stat *storage.ObjectInfo, key string) string {
	if stat != nil && stat.ContentType != "" {
		return stat.ContentType
	}

	if contentType := mime.TypeByExtension(filepath.Ext(key)); contentType != "" {
		return contentType
	}

	return fiber.MIMEOctetStream
}
