package monitor

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

// Response codes for monitor API errors (2100-2199).
const (
	ErrCodeNotReady         = 2100
	ErrCodeCollectionFailed = 2101
)

// Predefined monitor API errors.
var (
	// ErrNotReady indicates a background-sampled metric (CPU, process) has not
	// produced its first sample yet.
	ErrNotReady = result.Err(
		i18n.T("monitor_not_ready"),
		result.WithCode(ErrCodeNotReady),
	)
	// ErrCollectionFailed indicates a live metric collector failed to read a
	// system metric from the host.
	ErrCollectionFailed = result.Err(
		i18n.T("monitor_collection_failed"),
		result.WithCode(ErrCodeCollectionFailed),
	)
)
