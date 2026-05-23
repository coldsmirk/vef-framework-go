package storage

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for storage API errors (2200-2299).
const (
	ErrCodeInvalidFileKey  = 2200
	ErrCodeFileNotFound    = 2201
	ErrCodeFailedToGetFile = 2202
)

// Predefined storage API errors.
var (
	ErrInvalidFileKey = result.Err(
		i18n.T("storage_invalid_file_key"),
		result.WithCode(ErrCodeInvalidFileKey),
	)
	ErrFileNotFound = result.Err(
		i18n.T("storage_file_not_found"),
		result.WithCode(ErrCodeFileNotFound),
	)
	ErrFailedToGetFile = result.Err(
		i18n.T("storage_failed_to_get_file"),
		result.WithCode(ErrCodeFailedToGetFile),
	)
)
