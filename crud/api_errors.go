package crud

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for CRUD API errors (2400-2499).
// 2400 (ErrCodeProcessorInvalidReturn) lives in constants.go.
const (
	ErrCodeFieldNotExistInModel           = 2401
	ErrCodePrimaryKeyRequired             = 2402
	ErrCodeCompositePrimaryKeyRequiresMap = 2403
	ErrCodeUnsupportedExportFormat        = 2404
	ErrCodeImportRequiresMultipart        = 2405
	ErrCodeImportRequiresFile             = 2406
	ErrCodeUnsupportedImportFormat        = 2407
	ErrCodeFileOpenFailed                 = 2408
	ErrCodeImportTypeAssertionFailed      = 2409
	ErrCodeImportValidationFailed         = 2410
)

// Predefined CRUD API errors with fixed messages. These are business errors and
// keep the default HTTP 200 status; the failure is carried by the body code.
var (
	ErrCompositePrimaryKeyRequiresMap = result.Err(
		i18n.T("crud_composite_primary_key_requires_map"),
		result.WithCode(ErrCodeCompositePrimaryKeyRequiresMap),
	)
	ErrUnsupportedExportFormat = result.Err(
		i18n.T("crud_unsupported_export_format"),
		result.WithCode(ErrCodeUnsupportedExportFormat),
	)
	ErrImportRequiresMultipart = result.Err(
		i18n.T("crud_import_requires_multipart"),
		result.WithCode(ErrCodeImportRequiresMultipart),
	)
	ErrImportRequiresFile = result.Err(
		i18n.T("crud_import_requires_file"),
		result.WithCode(ErrCodeImportRequiresFile),
	)
	ErrUnsupportedImportFormat = result.Err(
		i18n.T("crud_unsupported_import_format"),
		result.WithCode(ErrCodeUnsupportedImportFormat),
	)
	ErrFileOpenFailed = result.Err(
		i18n.T("crud_file_open_failed"),
		result.WithCode(ErrCodeFileOpenFailed),
	)
	ErrImportTypeAssertionFailed = result.Err(
		i18n.T("crud_import_type_assertion_failed"),
		result.WithCode(ErrCodeImportTypeAssertionFailed),
	)
)

// ErrPrimaryKeyRequired reports that a required primary-key field was absent from
// the request. field is the name of the missing primary-key column.
func ErrPrimaryKeyRequired(field string) result.Error {
	return result.Err(
		i18n.T("crud_primary_key_required", map[string]any{"field": field}),
		result.WithCode(ErrCodePrimaryKeyRequired),
	)
}

// ErrFieldNotExistInModel reports that a referenced column does not exist on the
// target model. field is the offending column, name is the logical role it was
// supplied as, and model is the model's type name.
func ErrFieldNotExistInModel(field, name, model string) result.Error {
	return result.Err(
		i18n.T("crud_field_not_exist_in_model", map[string]any{
			"field": field,
			"name":  name,
			"model": model,
		}),
		result.WithCode(ErrCodeFieldNotExistInModel),
	)
}
