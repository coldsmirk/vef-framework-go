package middleware

import (
	"io/fs"
)

// SPAConfig is the configuration for the SPA middleware.
type SPAConfig struct {
	// Path is the path to the SPA.
	Path string
	// Fs is the file system for the SPA.
	Fs fs.FS
	// ExcludePaths are paths that should not be handled by the SPA (e.g., "/api", "/ws").
	ExcludePaths []string
}
