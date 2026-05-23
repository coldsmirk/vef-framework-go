package result

// i18n message keys for cross-cutting API responses. Module-specific
// keys live next to their module (e.g. security.ErrMessageTokenInvalid,
// monitor.ErrMessageMonitorNotReady).
const (
	OkMessage  = "ok"
	ErrMessage = "error"

	// HTTP-level generic errors.
	ErrMessageUnknown              = "unknown_error"
	ErrMessageNotFound             = "not_found"
	ErrMessageTooManyRequests      = "too_many_requests"
	ErrMessageAccessDenied         = "access_denied"
	ErrMessageUnsupportedMediaType = "unsupported_media_type"
	ErrMessageRequestTimeout       = "request_timeout"

	// ORM / persistence errors that any module may surface.
	ErrMessageRecordNotFound      = "record_not_found"
	ErrMessageRecordAlreadyExists = "record_already_exists"
	ErrMessageForeignKeyViolation = "foreign_key_violation"
	ErrMessageDangerousSQL        = "dangerous_sql"
)

// Response codes for cross-cutting API results.
// Code 0 indicates success; codes 1100-1599 are HTTP-level concerns;
// codes 1900+ are unknown / business errors.
const (
	OkCode = 0

	// Authorization errors (1100-1199).
	ErrCodeAccessDenied = 1100

	// Resource errors (1200-1299).
	ErrCodeNotFound = 1200

	// Media type errors (1300-1399).
	ErrCodeUnsupportedMediaType = 1300

	// Request errors (1400-1499).
	ErrCodeBadRequest      = 1400
	ErrCodeTooManyRequests = 1401
	ErrCodeRequestTimeout  = 1402

	// Not implemented (1500-1599).
	ErrCodeNotImplemented = 1500

	// SQL errors (1600-1699).
	ErrCodeDangerousSQL = 1600

	// Unknown errors (1900-1999).
	ErrCodeUnknown = 1900

	// Business errors (2000+).
	ErrCodeDefault             = 2000
	ErrCodeRecordNotFound      = 2001
	ErrCodeRecordAlreadyExists = 2002
	ErrCodeForeignKeyViolation = 2003
)
