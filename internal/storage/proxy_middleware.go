package storage

import (
	"context"
	"errors"
	"mime"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

// tokenExtractor mirrors the bearer strategy's chain. The header is the
// primary channel, but a browser rendering a private file in an <img> or
// following a download link cannot set one, so the standard access-token
// query parameter is the practical fallback — the same reasoning that put
// it on the push handshake.
var tokenExtractor = extractors.Chain(
	extractors.FromAuthHeader(security.AuthSchemeBearer),
	extractors.FromQuery(security.QueryKeyAccessToken),
)

type ProxyMiddleware struct {
	service   storage.Service
	acl       storage.FileACL
	registry  storage.FileRegistry
	auth      security.AuthManager
	tokenType string
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
		return storage.ErrInvalidFileKey
	}

	// url.PathUnescape returns its input unchanged when there is nothing to
	// unescape, so a key without escapes is still a view into the pooled
	// request buffer. It is handed to storage.FileACL — a host extension point
	// free to retain it — so copy it once here rather than trusting every
	// implementation to.
	key = strings.Clone(key)

	// Reject path traversal, absolute paths, and control characters.
	if !isValidObjectKey(key) {
		return storage.ErrInvalidFileKey
	}

	// pub/* is world-readable by design (bucket policy + CDN caching);
	// skip the ACL call entirely for performance and to allow anonymous
	// access without requiring an auth token on the request. Identity is
	// resolved inside this branch rather than above it for the same
	// reason: the pub/ path never reaches the ACL, so authenticating it
	// would buy nothing and would let an expired token in a long-lived
	// tab break public image loading.
	if !strings.HasPrefix(key, storage.PublicPrefix) {
		principal, authErr := p.authenticate(ctx)
		if authErr != nil {
			return authErr
		}

		allowed, aclErr := p.acl.CanRead(ctx.Context(), principal, key)
		if aclErr != nil {
			logger.Errorf("FileACL.CanRead failed for key %s: %v", key, aclErr)

			return storage.ErrFailedToGetFile
		}

		if !allowed {
			return result.ErrAccessDenied
		}
	}

	// Resolved BEFORE GetObject: the reader-ownership rule below forbids
	// a failing early return once the body is open, and this lookup can
	// fail. Its result is only needed for a response header.
	filename := p.originalFilename(ctx.Context(), key)

	// reader ownership: from this point on, the io.ReadCloser is handed
	// off to ctx.SendStream below, which is responsible for closing it
	// after the response body is flushed. Do NOT add an early return
	// between here and SendStream without closing reader first, or the
	// descriptor will leak.
	//
	// GetObject returns the object metadata alongside the reader from a
	// single backend fetch (no separate StatObject round-trip on this hot
	// path). stat is best-effort: a nil stat is non-fatal — the response
	// still streams the body, just without Content-Length / ETag headers.
	reader, stat, err := p.service.GetObject(ctx.Context(), storage.GetObjectOptions{
		Key: key,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return storage.ErrFileNotFound
		}

		logger.Errorf("Failed to get object %s: %v", key, err)

		return storage.ErrFailedToGetFile
	}

	contentType := detectContentType(stat, key)
	ctx.Set(fiber.HeaderContentType, contentType)
	ctx.Set("X-Content-Type-Options", "nosniff")

	// Safe to cache alongside the immutable directive below: a registry
	// record's original_filename is written once, when the upload is
	// finalized, and no code path ever updates it.
	if disposition := contentDisposition(contentType, filename); disposition != "" {
		ctx.Set(fiber.HeaderContentDisposition, disposition)
	}

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

// ProxyMiddlewareParams contains the dependencies of the download proxy.
type ProxyMiddlewareParams struct {
	fx.In

	Service  storage.Service
	ACL      storage.FileACL
	Registry storage.FileRegistry
	Auth     security.AuthManager
	Security *config.SecurityConfig
}

func NewProxyMiddleware(params ProxyMiddlewareParams) app.Middleware {
	return &ProxyMiddleware{
		service:   params.Service,
		acl:       params.ACL,
		registry:  params.Registry,
		auth:      params.Auth,
		tokenType: string(params.Security.EffectiveTokenType()),
	}
}

// authenticate resolves the caller's identity for a private key.
//
// This route is registered as an app.Middleware and therefore lives
// outside the /api pipeline, where api/middleware.Auth is the only thing
// in the framework that ever populates the request principal. Nothing
// else fills it in, so the proxy must dispatch the configured token
// mechanism itself — exactly as the push handshake and the MCP handler
// do for their own out-of-pipeline routes.
//
// A request carrying no token authenticates as nobody rather than being
// rejected: FileACL is the authority on private keys, and an
// implementation is free to grant an anonymous read (a share link, a
// tenant-wide asset). A token that is present but invalid IS rejected —
// the caller offered a credential, and downgrading it to anonymous would
// surface as an opaque access-denied on a request that was merely
// carrying an expired token.
func (p *ProxyMiddleware) authenticate(ctx fiber.Ctx) (*security.Principal, error) {
	token, err := tokenExtractor.Extract(ctx)

	// No credential on the request means an anonymous read, which the ACL
	// is free to grant. Any other extraction failure is a broken chain
	// rather than a missing token and must not pass as anonymous.
	if errors.Is(err, extractors.ErrNotFound) || token == "" {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return p.auth.Authenticate(ctx.Context(), security.Authentication{
		Type:      p.tokenType,
		Principal: token,
	})
}

// originalFilename resolves the name the file was uploaded under, or ""
// when the registry has no record for the key (an object written before
// the registry existed, or one put there outside the upload protocol).
//
// Best-effort by design, mirroring the nil-stat rule below: a download
// must never fail because its filename could not be resolved.
func (p *ProxyMiddleware) originalFilename(ctx context.Context, key string) string {
	found, err := p.registry.Lookup(ctx, []string{key})
	if err != nil {
		logger.Warnf("Resolve original filename for %s failed: %v", key, err)

		return ""
	}

	return found[key].OriginalFilename
}

// contentDisposition renders the RFC 6266 header that gives a browser
// the real filename on "save as". mime.FormatMediaType handles the
// RFC 2231/5987 encoding non-ASCII names need, and percent-encodes
// anything that is not an attribute character — so a filename can never
// inject a header, whatever sanitizeFilename let through.
//
// Types a browser renders in place stay inline so in-app previews keep
// working; everything else — archives, and anything sanitizeContentType
// already collapsed to application/octet-stream — is marked as an
// attachment, which costs nothing and puts a second barrier in front of
// content a browser might otherwise try to interpret.
//
// Returns "" when there is no name to advertise or the value cannot be
// encoded, so the caller simply omits the header.
func contentDisposition(contentType, filename string) string {
	if filename == "" {
		return ""
	}

	disposition := "attachment"
	if isInlineRenderable(contentType) {
		disposition = "inline"
	}

	return mime.FormatMediaType(disposition, map[string]string{"filename": filename})
}

// isInlineRenderable reports whether a browser renders contentType in
// place. Deliberately narrower than isSafeContentType, which answers a
// different question — whether the type is safe to serve at all: an
// archive is safe to serve and pointless to render.
func isInlineRenderable(contentType string) bool {
	for _, prefix := range safeContentTypePrefixes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}

	return contentType == "application/pdf"
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

// detectContentType resolves the Content-Type to serve. The result is
// always passed through sanitizeContentType before being written to the
// response header — backends may return whatever they like (e.g.
// filesystem currently re-derives from the file extension, so a
// `.html` upload yields `text/html`), and serving an unsafe type on the
// same origin would open a stored-XSS surface. Sanitize is the final
// barrier; it collapses anything outside the known-safe set to
// `application/octet-stream`.
func detectContentType(stat *storage.ObjectInfo, key string) string {
	var raw string

	switch {
	case stat != nil && stat.ContentType != "":
		raw = stat.ContentType
	default:
		raw = mime.TypeByExtension(filepath.Ext(key))
	}

	return sanitizeContentType(raw, key)
}
