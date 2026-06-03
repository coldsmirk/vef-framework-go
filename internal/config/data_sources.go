package config

import (
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
)

// ErrPrimaryDataSourceMissing is returned by newDataSourcesConfig when the
// TOML configuration omits vef.data_sources.primary.
var ErrPrimaryDataSourceMissing = errors.New("config: vef.data_sources.primary is required")

// newDataSourcesConfig reads vef.data_sources into a map keyed by data source
// name and validates that the primary entry is present. Additional entries
// (analytics, audit, ...) are passed through unchanged and registered later
// by the database module's seedStaticDataSources invoke.
func newDataSourcesConfig(cfg config.Config) (*config.DataSourcesConfig, error) {
	sources := map[string]config.DataSourceConfig{}
	if err := cfg.Unmarshal("vef.data_sources", &sources); err != nil {
		return nil, fmt.Errorf("unmarshal vef.data_sources: %w", err)
	}

	if _, ok := sources[config.PrimaryDataSourceName]; !ok {
		return nil, ErrPrimaryDataSourceMissing
	}

	return &config.DataSourcesConfig{Map: sources}, nil
}
