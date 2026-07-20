package config

import (
	"errors"
	"fmt"
	"time"
)

// Default values for CronStoreConfig, applied by the Effective* accessors.
const (
	DefaultCronStorePollInterval      = 5 * time.Second
	DefaultCronStoreBatchSize         = 32
	DefaultCronStoreMaxConcurrent     = 16
	DefaultCronStoreMisfireThreshold  = time.Minute
	DefaultCronStoreHeartbeatInterval = 10 * time.Second
	DefaultCronStoreAbandonedAfter    = time.Minute
)

// CronConfig defines cron scheduling settings (vef.cron).
type CronConfig struct {
	// Store configures the durable schedule store.
	Store CronStoreConfig `config:"store"`
}

// CronStoreConfig configures the durable schedule store
// (vef.cron.store): persisted schedules on the primary data source,
// cluster-wide single fire per occurrence, misfire handling, the run
// journal, and crash recovery.
type CronStoreConfig struct {
	// Enabled turns the store on. Off by default: with it off, schedules
	// are not loaded, no tables are touched, and the in-memory scheduler
	// is unaffected.
	Enabled bool `config:"enabled"`

	// AutoMigrate runs the cron DDL migration on application start.
	AutoMigrate bool `config:"auto_migrate"`

	// PollInterval bounds how long a node may wait before re-reading the
	// schedule table. The engine sleeps adaptively until the nearest known
	// fire, so this is the visibility latency of schedules created on other
	// nodes, not the fire precision. Default: 5s.
	PollInterval time.Duration `config:"poll_interval"`

	// BatchSize caps the schedules claimed per poll tick. Default: 32.
	BatchSize int `config:"batch_size"`

	// MaxConcurrent caps the runs executing concurrently on one node; the
	// engine claims no more fires than it has free slots. Default: 16.
	MaxConcurrent int `config:"max_concurrent"`

	// MisfireThreshold is how late a fire may start before it counts as
	// misfired and the schedule's misfire policy applies. Default: 1m.
	MisfireThreshold time.Duration `config:"misfire_threshold"`

	// HeartbeatInterval is the executor's liveness cadence on running runs.
	// Default: 10s.
	HeartbeatInterval time.Duration `config:"heartbeat_interval"`

	// AbandonedAfter is how stale a running run's heartbeat may be before
	// the recovery sweep marks it abandoned. Must be at least twice the
	// heartbeat interval. Default: 1m.
	AbandonedAfter time.Duration `config:"abandoned_after"`

	// RunTimeout bounds one run when the schedule sets no own timeout.
	// Zero (the default) leaves runs unbounded — business jobs may
	// legitimately run long; opt in deliberately.
	RunTimeout time.Duration `config:"run_timeout"`

	// RunRetention prunes terminal journal rows older than this window; the
	// sweep runs hourly. Zero (the default) keeps rows forever — deletion
	// of the run journal is strictly opt-in.
	RunRetention time.Duration `config:"run_retention"`
}

// EffectivePollInterval returns PollInterval or its default.
func (c *CronStoreConfig) EffectivePollInterval() time.Duration {
	return coalescePositive(c.PollInterval, DefaultCronStorePollInterval)
}

// EffectiveBatchSize returns BatchSize or its default.
func (c *CronStoreConfig) EffectiveBatchSize() int {
	return coalescePositive(c.BatchSize, DefaultCronStoreBatchSize)
}

// EffectiveMaxConcurrent returns MaxConcurrent or its default.
func (c *CronStoreConfig) EffectiveMaxConcurrent() int {
	return coalescePositive(c.MaxConcurrent, DefaultCronStoreMaxConcurrent)
}

// EffectiveMisfireThreshold returns MisfireThreshold or its default.
func (c *CronStoreConfig) EffectiveMisfireThreshold() time.Duration {
	return coalescePositive(c.MisfireThreshold, DefaultCronStoreMisfireThreshold)
}

// EffectiveHeartbeatInterval returns HeartbeatInterval or its default.
func (c *CronStoreConfig) EffectiveHeartbeatInterval() time.Duration {
	return coalescePositive(c.HeartbeatInterval, DefaultCronStoreHeartbeatInterval)
}

// EffectiveAbandonedAfter returns AbandonedAfter or its default.
func (c *CronStoreConfig) EffectiveAbandonedAfter() time.Duration {
	return coalescePositive(c.AbandonedAfter, DefaultCronStoreAbandonedAfter)
}

// Cron store validation sentinels.
var (
	// ErrInvalidCronStoreDuration indicates a negative duration setting.
	ErrInvalidCronStoreDuration = errors.New("invalid cron store duration")
	// ErrInvalidCronStoreCount indicates a negative batch or concurrency
	// setting.
	ErrInvalidCronStoreCount = errors.New("invalid cron store count")
	// ErrCronStoreAbandonedTooSoon indicates an abandoned window too tight
	// for the heartbeat cadence — healthy executors would be declared dead.
	ErrCronStoreAbandonedTooSoon = errors.New("cron store abandoned_after must be at least twice heartbeat_interval")
)

// Validate rejects self-defeating settings so configuration typos fail at
// startup instead of silently mis-scheduling.
func (c *CronConfig) Validate() error {
	store := &c.Store

	durations := map[string]time.Duration{
		"poll_interval":      store.PollInterval,
		"misfire_threshold":  store.MisfireThreshold,
		"heartbeat_interval": store.HeartbeatInterval,
		"abandoned_after":    store.AbandonedAfter,
		"run_timeout":        store.RunTimeout,
		"run_retention":      store.RunRetention,
	}
	for name, value := range durations {
		if value < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrInvalidCronStoreDuration, name)
		}
	}

	counts := map[string]int{
		"batch_size":     store.BatchSize,
		"max_concurrent": store.MaxConcurrent,
	}
	for name, value := range counts {
		if value < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrInvalidCronStoreCount, name)
		}
	}

	if store.EffectiveHeartbeatInterval() > store.EffectiveAbandonedAfter()/2 {
		return fmt.Errorf("%w: %s < 2 × %s",
			ErrCronStoreAbandonedTooSoon, store.EffectiveAbandonedAfter(), store.EffectiveHeartbeatInterval())
	}

	return nil
}
