package vef

import (
	ilog "github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/log"
)

// NamedLogger creates a named logger instance for the specified component.
// This is a convenience function that wraps the internal logger factory.
func NamedLogger(name string) log.Logger {
	return ilog.Named(name)
}
