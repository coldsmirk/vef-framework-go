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
)
