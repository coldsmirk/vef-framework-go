package monitor

import "github.com/coldsmirk/vef-framework-go/integration"

// IntegrationStatsInfo is the monitor payload for integration invocation
// statistics. Enabled reports whether a stats inspector is available (the
// integration module is on); when false, Stats is empty. Numbers are per
// node since process start — the invocation log is the durable record.
type IntegrationStatsInfo struct {
	Enabled bool                          `json:"enabled"`
	Stats   []integration.InvocationStats `json:"stats"`
}
