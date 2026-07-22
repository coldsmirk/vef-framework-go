package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Predefined API request decoding errors. A malformed request (unparseable
// body, wrong param/meta type) is a client error, so it carries HTTP 400 —
// matching the validation path, which also returns 400 for the same
// bad-request code. (Business errors keep HTTP 200 with a code; a malformed
// request is a transport-level fault, not a business outcome.)
var (
	ErrInvalidRequestParams = result.Err(
		i18n.T("api_request_params_invalid_json"),
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusBadRequest),
	)
	ErrInvalidRequestMeta = result.Err(
		i18n.T("api_request_meta_invalid_json"),
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusBadRequest),
	)
	// ErrUnsupportedBodyEncoding rejects an X-Body-Encoding value the framework
	// does not implement.
	ErrUnsupportedBodyEncoding = result.Err(
		i18n.T("api_body_encoding_unsupported"),
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusBadRequest),
	)
	// ErrBodyDecodeFailed reports an X-Body-Encoding body that failed to decode
	// (malformed base64 or a corrupt gzip stream).
	ErrBodyDecodeFailed = result.Err(
		i18n.T("api_body_decode_failed"),
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusBadRequest),
	)
	// ErrBodyTooLarge reports a decoded body that exceeds the configured body
	// limit, mirroring Fiber's native Content-Encoding decompression guard so a
	// decompression bomb cannot outgrow vef.app.body_limit.
	ErrBodyTooLarge = result.Err(
		i18n.T("api_body_too_large"),
		result.WithCode(result.ErrCodeBadRequest),
		result.WithStatus(fiber.StatusRequestEntityTooLarge),
	)
)
