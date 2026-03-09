package taskpool

import (
	"fmt"
	"runtime"
	"time"

	"github.com/coldsmirk/vef-framework-go/log"
)

const (
	DefaultMinWorkers          = 1
	DefaultMaxWorkers          = 0 // 0 = runtime.NumCPU() * 2
	DefaultIdleTimeout         = 30 * time.Second
	DefaultTaskQueueSize       = 100
	DefaultTaskTimeout         = 5 * time.Second
	DefaultMaxTaskTimeout      = 30 * time.Second
	DefaultHealthCheckInterval = 1 * time.Minute
)

type Config[TIn, TOut any] struct {
	MinWorkers          int
	MaxWorkers          int
	IdleTimeout         time.Duration
	TaskQueueSize       int
	TaskTimeout         time.Duration
	MaxTaskTimeout      time.Duration
	DelegateFactory     DelegateFactory[TIn, TOut]
	DelegateConfig      any
	HealthCheckInterval time.Duration
	Logger              log.Logger
}

type DelegateFactory[TIn, TOut any] func() WorkerDelegate[TIn, TOut]

// Validate validates the configuration and applies defaults.
func (c *Config[TIn, TOut]) Validate() error {
	c.applyDefaults()

	return c.validateConstraints()
}

func (c *Config[TIn, TOut]) applyDefaults() {
	if c.MinWorkers <= 0 {
		c.MinWorkers = DefaultMinWorkers
	}

	if c.MaxWorkers <= 0 {
		c.MaxWorkers = runtime.NumCPU() * 2
	}

	if c.IdleTimeout == 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}

	if c.TaskQueueSize <= 0 {
		c.TaskQueueSize = DefaultTaskQueueSize
	}

	if c.TaskTimeout == 0 {
		c.TaskTimeout = DefaultTaskTimeout
	}

	if c.MaxTaskTimeout == 0 {
		c.MaxTaskTimeout = DefaultMaxTaskTimeout
	}

	if c.HealthCheckInterval == 0 {
		c.HealthCheckInterval = DefaultHealthCheckInterval
	}
}

func (c *Config[TIn, TOut]) validateConstraints() error {
	if c.MinWorkers > c.MaxWorkers {
		return fmt.Errorf("%w: MinWorkers (%d) > MaxWorkers (%d)",
			ErrInvalidConfig, c.MinWorkers, c.MaxWorkers)
	}

	if c.DelegateFactory == nil {
		return fmt.Errorf("%w: DelegateFactory is required", ErrInvalidConfig)
	}

	if c.Logger == nil {
		return fmt.Errorf("%w: Logger is required", ErrInvalidConfig)
	}

	if c.TaskTimeout > c.MaxTaskTimeout {
		return fmt.Errorf("%w: TaskTimeout (%v) > MaxTaskTimeout (%v)",
			ErrInvalidConfig, c.TaskTimeout, c.MaxTaskTimeout)
	}

	return nil
}
