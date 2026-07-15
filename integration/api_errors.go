package integration

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for integration API errors (2600-2699).
const (
	ErrCodeContractNotFound  = 2600
	ErrCodeContractDisabled  = 2601
	ErrCodeSystemNotFound    = 2602
	ErrCodeSystemDisabled    = 2603
	ErrCodeAdapterNotFound   = 2604
	ErrCodeAdapterDisabled   = 2605
	ErrCodeRouteNotFound     = 2606
	ErrCodeTargetAmbiguous   = 2607
	ErrCodeInputInvalid      = 2608
	ErrCodeOutputInvalid     = 2609
	ErrCodeUpstreamFailed    = 2610
	ErrCodeTransportFailed   = 2611
	ErrCodeInvocationTimeout = 2612
	ErrCodeScriptFailed      = 2613
	ErrCodeUnknownAuthScheme = 2614
	ErrCodeInvalidSchema     = 2615
	ErrCodeInvalidScript     = 2616
	ErrCodeInvalidAuthParams = 2617
	ErrCodeInvalidRouteRef   = 2618
	ErrCodeInvalidBaseURL    = 2619
	ErrCodeInvalidDataSource = 2620
	ErrCodeInvalidDirection  = 2621
)

// Predefined integration API errors. These are business errors and keep the
// default HTTP 200 status; the failure is carried by the body code.
var (
	ErrContractNotFound = result.Err(
		i18n.T("integration_contract_not_found"),
		result.WithCode(ErrCodeContractNotFound),
	)
	ErrContractDisabled = result.Err(
		i18n.T("integration_contract_disabled"),
		result.WithCode(ErrCodeContractDisabled),
	)
	ErrSystemNotFound = result.Err(
		i18n.T("integration_system_not_found"),
		result.WithCode(ErrCodeSystemNotFound),
	)
	ErrSystemDisabled = result.Err(
		i18n.T("integration_system_disabled"),
		result.WithCode(ErrCodeSystemDisabled),
	)
	ErrAdapterNotFound = result.Err(
		i18n.T("integration_adapter_not_found"),
		result.WithCode(ErrCodeAdapterNotFound),
	)
	ErrAdapterDisabled = result.Err(
		i18n.T("integration_adapter_disabled"),
		result.WithCode(ErrCodeAdapterDisabled),
	)
	ErrRouteNotFound = result.Err(
		i18n.T("integration_route_not_found"),
		result.WithCode(ErrCodeRouteNotFound),
	)
	// ErrTargetAmbiguous rejects an invocation passing both WithSystem and
	// WithRoute — the two target selectors are mutually exclusive.
	ErrTargetAmbiguous = result.Err(
		i18n.T("integration_target_ambiguous"),
		result.WithCode(ErrCodeTargetAmbiguous),
	)
	// ErrTransportFailed marks a wire call that never completed. Sentinel
	// (not a factory) because the transport detail belongs in logs, not in
	// the API response.
	ErrTransportFailed = result.Err(
		i18n.T("integration_transport_failed"),
		result.WithCode(ErrCodeTransportFailed),
	)
	ErrInvocationTimeout = result.Err(
		i18n.T("integration_invocation_timeout"),
		result.WithCode(ErrCodeInvocationTimeout),
	)
	// ErrInvalidRouteRef rejects a route referencing a missing contract or
	// system at save time.
	ErrInvalidRouteRef = result.Err(
		i18n.T("integration_invalid_route_ref"),
		result.WithCode(ErrCodeInvalidRouteRef),
	)
	// ErrInvalidBaseURL rejects a system base URL that does not parse as an
	// absolute URL at save time.
	ErrInvalidBaseURL = result.Err(
		i18n.T("integration_invalid_base_url"),
		result.WithCode(ErrCodeInvalidBaseURL),
	)
)

// ErrInputInvalid reports input rejected by the contract's input schema.
func ErrInputInvalid(detail string) result.Error {
	return result.Err(
		i18n.T("integration_input_invalid", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeInputInvalid),
	)
}

// ErrOutputInvalid reports a script return value rejected by the contract's
// output schema.
func ErrOutputInvalid(detail string) result.Error {
	return result.Err(
		i18n.T("integration_output_invalid", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeOutputInvalid),
	)
}

// ErrUpstreamFailed reports a failure the external system itself signaled,
// carrying the message the adapter script surfaced via errors.upstream.
func ErrUpstreamFailed(message string) result.Error {
	return result.Err(
		i18n.T("integration_upstream_failed", map[string]any{"message": message}),
		result.WithCode(ErrCodeUpstreamFailed),
	)
}

// ErrScriptFailed reports an adapter script that threw or failed to compile.
func ErrScriptFailed(detail string) result.Error {
	return result.Err(
		i18n.T("integration_script_failed", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeScriptFailed),
	)
}

// ErrUnknownAuthScheme rejects a system whose auth references a scheme no
// registered AuthScheme reports as its name.
func ErrUnknownAuthScheme(scheme string) result.Error {
	return result.Err(
		i18n.T("integration_unknown_auth_scheme", map[string]any{"scheme": scheme}),
		result.WithCode(ErrCodeUnknownAuthScheme),
	)
}

// ErrInvalidSchema rejects a contract schema that does not parse or resolve
// as a self-contained JSON Schema at save time.
func ErrInvalidSchema(detail string) result.Error {
	return result.Err(
		i18n.T("integration_invalid_schema", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeInvalidSchema),
	)
}

// ErrInvalidScript rejects an adapter script that does not compile at save
// time.
func ErrInvalidScript(detail string) result.Error {
	return result.Err(
		i18n.T("integration_invalid_script", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeInvalidScript),
	)
}

// ErrInvalidAuthParams rejects auth parameters an AuthScheme refused at save
// time.
func ErrInvalidAuthParams(detail string) result.Error {
	return result.Err(
		i18n.T("integration_invalid_auth_params", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeInvalidAuthParams),
	)
}

// ErrInvalidDataSource rejects a system data source configuration that is
// incomplete or whose credential cannot be processed.
func ErrInvalidDataSource(detail string) result.Error {
	return result.Err(
		i18n.T("integration_invalid_data_source", map[string]any{"detail": detail}),
		result.WithCode(ErrCodeInvalidDataSource),
	)
}

// ErrInvalidDirection rejects an adapter direction outside the known flows.
var ErrInvalidDirection = result.Err(
	i18n.T("integration_invalid_direction"),
	result.WithCode(ErrCodeInvalidDirection),
)
