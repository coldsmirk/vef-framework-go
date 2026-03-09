package storage

import (
	"errors"
	"mime"
	"net/url"
	"path/filepath"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/storage"
)

type ProxyMiddleware struct {
	service storage.Service
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
	key, err := url.PathUnescape(ctx.Params("+"))
	if err != nil {
		return result.Err(
			i18n.T(result.ErrMessageInvalidFileKey),
			result.WithCode(result.ErrCodeInvalidFileKey),
		)
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

	ctx.Set(fiber.HeaderCacheControl, "public, max-age=86400, must-revalidate")

	if stat != nil && stat.ETag != "" {
		ctx.Set(fiber.HeaderETag, stat.ETag)
	}

	return ctx.SendStream(reader)
}

func NewProxyMiddleware(service storage.Service) app.Middleware {
	return &ProxyMiddleware{
		service: service,
	}
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
