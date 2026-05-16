package config

import "time"

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
