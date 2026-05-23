package monitor

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for monitor API errors (2100-2199).
const (
	ErrCodeNotReady = 2100
)

// Predefined monitor API errors.
var (
	ErrNotReady = result.Err(
		i18n.T("monitor_not_ready"),
		result.WithCode(ErrCodeNotReady),
	)
)
