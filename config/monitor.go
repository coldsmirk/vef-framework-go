package config

import "time"

// MonitorConfig defines monitoring service settings.
type MonitorConfig struct {
	SampleInterval time.Duration `config:"sample_interval"` // Interval between samples (default: 10s)
	SampleDuration time.Duration `config:"sample_duration"` // Sampling window duration (default: 2s)
}
