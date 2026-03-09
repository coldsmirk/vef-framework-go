package monitor

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/config"
)

const (
	// DefaultSampleInterval is the default interval between CPU and process sampling.
	// Sample every 10 seconds provides a good balance between accuracy and overhead.
	DefaultSampleInterval = 10 * time.Second
	// DefaultSampleDuration is the default sampling window duration for CPU and process metrics.
	// A 2-second window smooths short-term fluctuations while providing responsive metrics.
	DefaultSampleDuration = 2 * time.Second
)

// DefaultConfig returns the default monitor configuration.
// This configuration provides reasonable defaults for most use cases:
// - 10 second sampling interval (20% duty cycle with 2s window)
// - 2 second sampling window (smooths CPU spikes).
func DefaultConfig() config.MonitorConfig {
	return config.MonitorConfig{
		SampleInterval: DefaultSampleInterval,
		SampleDuration: DefaultSampleDuration,
	}
}
