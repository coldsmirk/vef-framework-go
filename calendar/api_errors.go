package calendar

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

const ErrCodeInvalidWindowRule = 2800

const ErrMessageInvalidWindowRule = "calendar_invalid_window_rule"

// ErrInvalidWindowRule identifies a structural error in a recurring window rule.
var ErrInvalidWindowRule = invalidRule("")

func invalidRule(reason string) result.Error {
	return result.Err(i18n.T(ErrMessageInvalidWindowRule, map[string]any{"reason": reason}), result.WithCode(ErrCodeInvalidWindowRule))
}
