package config

import "time"

// MonitorConfig defines monitoring service settings.
type MonitorConfig struct {
	SampleInterval time.Duration `config:"sample_interval"` // Interval between samples (default: 10s)
	SampleDuration time.Duration `config:"sample_duration"` // Sampling window duration (default: 2s)
	// ExcludedMounts lists additional mount-point substrings to exclude from disk
	// statistics, on top of the built-in OS pseudo-filesystem prefixes. Use this for
	// host- or vendor-specific volumes (e.g. developer-tool or virtualization mounts)
	// that should not count toward disk totals.
	ExcludedMounts []string `config:"excluded_mounts"`
}
