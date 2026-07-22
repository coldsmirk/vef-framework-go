package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

const (
	// bodyEncodingBase64 marks a body that is base64 of the raw JSON.
	bodyEncodingBase64 = "base64"
	// bodyEncodingGzipBase64 marks a body that is base64 of a gzip stream of
	// the raw JSON (decode order: base64 then gunzip). Compressing first keeps
	// the wire payload small while the base64 text shape survives middleboxes
	// that reject or inspect a binary Content-Encoding request.
	bodyEncodingGzipBase64 = "gzip+base64"
	// defaultBodyLimit mirrors createFiberApp's fallback so a decoded body may
	// not outgrow the server's configured request-body limit.
	defaultBodyLimit = "32mib"
)

// NewBodyEncodingMiddleware decodes an opt-in X-Body-Encoding request body back
// to raw JSON before the dispatcher parses it, so a client can transport-encode
// code-shaped payloads (integration adapter scripts, envelope/auth scripts,
// dry-run bodies) past middleboxes that false-positive on them. The decode is
// transport-only: storage, content-hash caches and audit all see the raw body.
// A request without the header is untouched, and only the /api surface is
// eligible (other surfaces own their body formats).
//
// The native Content-Encoding path (gzip/br/deflate/zstd) is handled by Fiber
// itself with the same body-limit guard; this middleware covers the encodings
// Fiber does not know — base64 and gzip+base64.
func NewBodyEncodingMiddleware(cfg *config.AppConfig) app.Middleware {
	limit := resolveBodyLimit(cfg.BodyLimit)

	return &SimpleMiddleware{
		handler: func(ctx fiber.Ctx) error {
			if !isAPIPath(ctx.Path()) {
				return ctx.Next()
			}

			encoding := strings.TrimSpace(ctx.Get(api.HeaderXBodyEncoding))
			if encoding == "" {
				return ctx.Next()
			}

			decoded, err := decodeBody(ctx.Request().Body(), encoding, limit)
			if err != nil {
				return err
			}

			ctx.Request().SetBody(decoded)
			// Drop the marker so nothing downstream re-decodes the raw body.
			ctx.Request().Header.Del(api.HeaderXBodyEncoding)

			return ctx.Next()
		},
		name:  "body_encoding",
		order: -750,
	}
}

// decodeBody reverses the client-applied transport encoding, bounding any
// decompression at limit so a gzip bomb cannot outgrow the body limit.
func decodeBody(raw []byte, encoding string, limit int) ([]byte, error) {
	switch encoding {
	case bodyEncodingBase64:
		return decodeBase64(raw)
	case bodyEncodingGzipBase64:
		compressed, err := decodeBase64(raw)
		if err != nil {
			return nil, err
		}

		return gunzipWithLimit(compressed, limit)

	default:
		return nil, api.ErrUnsupportedBodyEncoding
	}
}

// decodeBase64 decodes standard padded base64, tolerating surrounding
// whitespace an intermediary may have added.
func decodeBase64(raw []byte) ([]byte, error) {
	src := bytes.TrimSpace(raw)

	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(src)))

	n, err := base64.StdEncoding.Decode(decoded, src)
	if err != nil {
		return nil, api.ErrBodyDecodeFailed
	}

	return decoded[:n], nil
}

// gunzipWithLimit inflates a gzip stream, reading at most limit+1 bytes so an
// over-limit stream is rejected without inflating the whole payload into memory.
func gunzipWithLimit(compressed []byte, limit int) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, api.ErrBodyDecodeFailed
	}
	defer func() { _ = reader.Close() }()

	decoded, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, api.ErrBodyDecodeFailed
	}

	if len(decoded) > limit {
		return nil, api.ErrBodyTooLarge
	}

	return decoded, nil
}

// resolveBodyLimit parses the configured request-body limit, falling back to the
// framework default when unset or unparseable (createFiberApp fails the boot on
// a genuinely invalid value, so the fallback only guards ordering).
func resolveBodyLimit(raw string) int {
	limit, err := humanize.ParseBytes(lo.CoalesceOrEmpty(strings.TrimSpace(raw), defaultBodyLimit))
	if err != nil {
		limit, _ = humanize.ParseBytes(defaultBodyLimit)
	}

	return int(limit)
}
