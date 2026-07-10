package config

import (
	"errors"
	"fmt"
	"time"
)

// ApprovalBindingConsistency selects how business-table projections
// participate in approval transactions.
type ApprovalBindingConsistency string

const (
	// ApprovalBindingSynchronous writes the business row inside the approval
	// transaction. A projection failure rolls the approval action back.
	ApprovalBindingSynchronous ApprovalBindingConsistency = "synchronous"
	// ApprovalBindingEventual commits the desired projection with the approval
	// action and lets the binding worker converge the business row afterwards.
	ApprovalBindingEventual ApprovalBindingConsistency = "eventual"
)

// ErrInvalidApprovalBindingConsistency indicates an unsupported business
// binding consistency mode.
var ErrInvalidApprovalBindingConsistency = errors.New("invalid approval business binding consistency")

// ErrInvalidApprovalBusinessBindingWorkerConfig indicates a negative worker
// interval or batch size. Zero leaves the corresponding setting at its default.
var ErrInvalidApprovalBusinessBindingWorkerConfig = errors.New("invalid approval business binding worker config")

// ApprovalBusinessBindingConfig controls business-state projection behavior.
type ApprovalBusinessBindingConfig struct {
	// Consistency defaults to synchronous so business projection failures abort
	// the approval action unless eventual consistency is explicitly enabled.
	Consistency ApprovalBindingConsistency `config:"consistency"`
	// ScanInterval is the eventual projection worker cadence. Default: 10 seconds.
	ScanInterval time.Duration `config:"scan_interval"`
	// BatchSize bounds the number of pending projections processed per scan.
	// Default: 100.
	BatchSize int `config:"batch_size"`
}

// EffectiveConsistency returns Consistency or the synchronous default.
func (c *ApprovalBusinessBindingConfig) EffectiveConsistency() ApprovalBindingConsistency {
	if c.Consistency == "" {
		return ApprovalBindingSynchronous
	}

	return c.Consistency
}

// EffectiveScanInterval returns ScanInterval or its default.
func (c *ApprovalBusinessBindingConfig) EffectiveScanInterval() time.Duration {
	return coalescePositive(c.ScanInterval, 10*time.Second)
}

// EffectiveBatchSize returns BatchSize or its default.
func (c *ApprovalBusinessBindingConfig) EffectiveBatchSize() int {
	return coalescePositive(c.BatchSize, 100)
}

// Validate rejects unsupported consistency modes so configuration typos fail
// at startup instead of silently selecting different transaction semantics.
func (c *ApprovalBusinessBindingConfig) Validate() error {
	switch c.EffectiveConsistency() {
	case ApprovalBindingSynchronous, ApprovalBindingEventual:
	default:
		return fmt.Errorf("%w %q (want %q or %q)", ErrInvalidApprovalBindingConsistency,
			c.Consistency, ApprovalBindingSynchronous, ApprovalBindingEventual)
	}

	if c.ScanInterval < 0 {
		return fmt.Errorf("%w: scan_interval must be positive when set", ErrInvalidApprovalBusinessBindingWorkerConfig)
	}

	if c.BatchSize < 0 {
		return fmt.Errorf("%w: batch_size must be positive when set", ErrInvalidApprovalBusinessBindingWorkerConfig)
	}

	return nil
}

// ApprovalConfig defines approval workflow engine settings.
//
// Outbox-related fields previously lived here; they have moved to
// EventConfig.Transports.Outbox so the framework-wide outbox transport
// can serve any module, not just approval.
type ApprovalConfig struct {
	// AutoMigrate runs the approval DDL migration on application start.
	AutoMigrate bool `config:"auto_migrate"`

	// TimeoutScanInterval is the polling cadence for the timeout scanner
	// that finds tasks past their deadline. Default: 1 minute.
	TimeoutScanInterval time.Duration `config:"timeout_scan_interval"`

	// PreWarningScanInterval is the polling cadence for the pre-warning
	// scanner that notifies before a task hits its deadline. Default: 5 minutes.
	PreWarningScanInterval time.Duration `config:"pre_warning_scan_interval"`

	// CleanupScanInterval is the cadence of the retention cleanup job
	// that prunes form snapshots, urge records, and CC records past
	// their retention window. Default: 24 hours.
	CleanupScanInterval time.Duration `config:"cleanup_scan_interval"`

	// DelegationMaxDepth caps how deep a delegation chain (A→B→C…) can
	// be resolved before short-circuiting. Default: 10.
	DelegationMaxDepth int `config:"delegation_max_depth"`

	// FormSnapshotRetention is the retention window for apv_form_snapshot
	// rows; older snapshots are deleted by the cleanup job. Default: 90 days.
	FormSnapshotRetention time.Duration `config:"form_snapshot_retention"`

	// UrgeRecordRetention is the retention window for apv_urge_record rows.
	// Default: 30 days.
	UrgeRecordRetention time.Duration `config:"urge_record_retention"`

	// CCRecordRetention is the retention window for apv_cc_record rows
	// (only records that have been read are pruned). Default: 90 days.
	CCRecordRetention time.Duration `config:"cc_record_retention"`

	// BusinessBinding controls approval-to-business-table state projection.
	BusinessBinding ApprovalBusinessBindingConfig `config:"business_binding"`
}

// ApplyDefaults fills zero-valued fields with sensible defaults so callers
// don't have to mirror them in every TOML file.
func (c *ApprovalConfig) ApplyDefaults() {
	if c.TimeoutScanInterval <= 0 {
		c.TimeoutScanInterval = time.Minute
	}

	if c.PreWarningScanInterval <= 0 {
		c.PreWarningScanInterval = 5 * time.Minute
	}

	if c.CleanupScanInterval <= 0 {
		c.CleanupScanInterval = 24 * time.Hour
	}

	if c.DelegationMaxDepth <= 0 {
		c.DelegationMaxDepth = 10
	}

	if c.FormSnapshotRetention <= 0 {
		c.FormSnapshotRetention = 90 * 24 * time.Hour
	}

	if c.UrgeRecordRetention <= 0 {
		c.UrgeRecordRetention = 30 * 24 * time.Hour
	}

	if c.CCRecordRetention <= 0 {
		c.CCRecordRetention = 90 * 24 * time.Hour
	}
}

// Validate checks approval configuration invariants.
func (c *ApprovalConfig) Validate() error {
	return c.BusinessBinding.Validate()
}
