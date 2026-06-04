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

	ErrCodeClaimNotPending            = 2203
	ErrCodeClaimExpired               = 2204
	ErrCodeUploadSizeExceedsLimit     = 2205
	ErrCodeMultipartNotSupported      = 2206
	ErrCodePublicUploadsNotAllowed    = 2207
	ErrCodeUploadTooManyParts         = 2208
	ErrCodeTooManyPendingUploads      = 2209
	ErrCodeUploadRequiresMultipart    = 2210
	ErrCodeUploadRequiresFile         = 2211
	ErrCodeClaimNotMultipart          = 2212
	ErrCodeUploadPartNumberOutOfRange = 2213
	ErrCodeUploadPartTooLarge         = 2214
	ErrCodeUploadPartTooSmall         = 2215
	ErrCodeUploadPartsIncomplete      = 2216
	ErrCodeUploadObjectNotFound       = 2217
	ErrCodeUploadSizeMismatch         = 2218
	ErrCodeAbortFailed                = 2219
)

// Predefined storage API errors. These are business errors and keep the default
// HTTP 200 status; the failure is carried by the body code.
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

	ErrClaimNotPending = result.Err(
		i18n.T("storage_claim_not_pending"),
		result.WithCode(ErrCodeClaimNotPending),
	)
	ErrClaimExpired = result.Err(
		i18n.T("storage_claim_expired"),
		result.WithCode(ErrCodeClaimExpired),
	)
	ErrUploadSizeExceedsLimit = result.Err(
		i18n.T("storage_upload_size_exceeds_limit"),
		result.WithCode(ErrCodeUploadSizeExceedsLimit),
	)
	ErrMultipartNotSupported = result.Err(
		i18n.T("storage_multipart_not_supported"),
		result.WithCode(ErrCodeMultipartNotSupported),
	)
	ErrPublicUploadsNotAllowed = result.Err(
		i18n.T("storage_public_uploads_not_allowed"),
		result.WithCode(ErrCodePublicUploadsNotAllowed),
	)
	ErrUploadTooManyParts = result.Err(
		i18n.T("storage_upload_too_many_parts"),
		result.WithCode(ErrCodeUploadTooManyParts),
	)
	ErrTooManyPendingUploads = result.Err(
		i18n.T("storage_too_many_pending_uploads"),
		result.WithCode(ErrCodeTooManyPendingUploads),
	)
	ErrUploadRequiresMultipart = result.Err(
		i18n.T("storage_upload_requires_multipart"),
		result.WithCode(ErrCodeUploadRequiresMultipart),
	)
	ErrUploadRequiresFile = result.Err(
		i18n.T("storage_upload_requires_file"),
		result.WithCode(ErrCodeUploadRequiresFile),
	)
	ErrClaimNotMultipart = result.Err(
		i18n.T("storage_claim_not_multipart"),
		result.WithCode(ErrCodeClaimNotMultipart),
	)
	ErrUploadPartNumberOutOfRange = result.Err(
		i18n.T("storage_part_number_out_of_range"),
		result.WithCode(ErrCodeUploadPartNumberOutOfRange),
	)
	ErrUploadPartTooLarge = result.Err(
		i18n.T("storage_upload_part_too_large"),
		result.WithCode(ErrCodeUploadPartTooLarge),
	)
	ErrUploadPartTooSmall = result.Err(
		i18n.T("storage_upload_part_too_small"),
		result.WithCode(ErrCodeUploadPartTooSmall),
	)
	ErrUploadPartsIncomplete = result.Err(
		i18n.T("storage_upload_parts_incomplete"),
		result.WithCode(ErrCodeUploadPartsIncomplete),
	)
	ErrUploadObjectNotFound = result.Err(
		i18n.T("storage_object_not_found"),
		result.WithCode(ErrCodeUploadObjectNotFound),
	)
	ErrUploadSizeMismatch = result.Err(
		i18n.T("storage_upload_size_mismatch"),
		result.WithCode(ErrCodeUploadSizeMismatch),
	)
	ErrAbortFailed = result.Err(
		i18n.T("storage_abort_failed"),
		result.WithCode(ErrCodeAbortFailed),
	)
)
