package config

// ApprovalConfig defines approval workflow engine settings.
type ApprovalConfig struct {
	AutoMigrate         bool `config:"auto_migrate"`
	OutboxRelayInterval int  `config:"outbox_relay_interval"` // Polling interval in seconds (default: 5)
	OutboxMaxRetries    int  `config:"outbox_max_retries"`    // Max retry attempts (default: 10)
	OutboxBatchSize     int  `config:"outbox_batch_size"`     // Max events per poll (default: 100)
}

// OutboxRelayIntervalOrDefault returns the relay interval, defaulting to 5 seconds.
func (c *ApprovalConfig) OutboxRelayIntervalOrDefault() int {
	if c.OutboxRelayInterval <= 0 {
		return 5
	}

	return c.OutboxRelayInterval
}

// OutboxMaxRetriesOrDefault returns the max retries, defaulting to 10.
func (c *ApprovalConfig) OutboxMaxRetriesOrDefault() int {
	if c.OutboxMaxRetries <= 0 {
		return 10
	}

	return c.OutboxMaxRetries
}

// OutboxBatchSizeOrDefault returns the batch size, defaulting to 100.
func (c *ApprovalConfig) OutboxBatchSizeOrDefault() int {
	if c.OutboxBatchSize <= 0 {
		return 100
	}

	return c.OutboxBatchSize
}
