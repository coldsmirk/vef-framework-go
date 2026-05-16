package config

// ApprovalConfig defines approval workflow engine settings.
//
// Outbox-related fields previously lived here; they have moved to
// EventConfig.Transports.Outbox so the framework-wide outbox transport
// can serve any module, not just approval.
type ApprovalConfig struct {
	AutoMigrate bool `config:"auto_migrate"`
}
