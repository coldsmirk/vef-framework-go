package cron

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for cron API errors (2700-2799).
const (
	ErrCodeScheduleNotFound = 2700
	ErrCodeScheduleExists   = 2701
	ErrCodeScheduleDisabled = 2702
	ErrCodeTriggerInvalid   = 2703
	ErrCodeJobNotRegistered = 2704
	ErrCodeStoreDisabled    = 2705
	ErrCodeScheduleInvalid  = 2706
)

// Predefined cron API errors. These are business errors and keep the default
// HTTP 200 status; the failure is carried by the body code.
var (
	ErrScheduleNotFound = result.Err(
		i18n.T("cron_schedule_not_found"),
		result.WithCode(ErrCodeScheduleNotFound),
	)
	ErrScheduleExists = result.Err(
		i18n.T("cron_schedule_exists"),
		result.WithCode(ErrCodeScheduleExists),
	)
	ErrScheduleDisabled = result.Err(
		i18n.T("cron_schedule_disabled"),
		result.WithCode(ErrCodeScheduleDisabled),
	)
	ErrJobNotRegistered = result.Err(
		i18n.T("cron_job_not_registered"),
		result.WithCode(ErrCodeJobNotRegistered),
	)
	ErrStoreDisabled = result.Err(
		i18n.T("cron_store_disabled"),
		result.WithCode(ErrCodeStoreDisabled),
	)
)

// ErrTriggerInvalid builds the trigger-validation error carrying the concrete
// reason. errors.Is matches the sentinel semantics through the shared code.
func ErrTriggerInvalid(reason string) result.Error {
	return result.Err(
		i18n.T("cron_trigger_invalid", map[string]any{"reason": reason}),
		result.WithCode(ErrCodeTriggerInvalid),
	)
}

// ErrScheduleInvalid builds the spec-validation error for non-trigger faults
// (name, window, timeout, params, policy vocabulary), carrying the concrete
// reason. errors.Is matches through the shared code.
func ErrScheduleInvalid(reason string) result.Error {
	return result.Err(
		i18n.T("cron_schedule_invalid", map[string]any{"reason": reason}),
		result.WithCode(ErrCodeScheduleInvalid),
	)
}
