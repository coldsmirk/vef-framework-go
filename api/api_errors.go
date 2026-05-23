package api

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Predefined API request decoding errors (HTTP 200 with bad-request code).
var (
	ErrInvalidRequestParams = result.Err(
		i18n.T("api_request_params_invalid_json"),
		result.WithCode(result.ErrCodeBadRequest),
	)
	ErrInvalidRequestMeta = result.Err(
		i18n.T("api_request_meta_invalid_json"),
		result.WithCode(result.ErrCodeBadRequest),
	)
)
