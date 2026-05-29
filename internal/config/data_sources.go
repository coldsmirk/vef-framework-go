package config

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/config"
)

// ErrPrimaryDataSourceMissing is returned by newDataSourcesConfig when the
// TOML configuration omits vef.data_sources.primary.
var ErrPrimaryDataSourceMissing = errors.New("config: vef.data_sources.primary is required")

// newDataSourcesConfig reads vef.data_sources into a map keyed by data source
// name and validates that the primary entry is present. Additional entries
// (analytics, audit, ...) are passed through unchanged and registered later
// by the database module's seedStaticDataSources invoke.
//
// The legacy vef.data_source key is accepted as a compatibility fallback and
// mapped to primary. New configuration should always use vef.data_sources.
func newDataSourcesConfig(cfg config.Config) (*config.DataSourcesConfig, error) {
	sources := map[string]config.DataSourceConfig{}
	if err := cfg.Unmarshal("vef.data_sources", &sources); err != nil {
		return nil, fmt.Errorf("unmarshal vef.data_sources: %w", err)
	}

	if len(sources) == 0 {
		legacy := config.DataSourceConfig{}
		if err := cfg.Unmarshal("vef.data_source", &legacy); err != nil {
			return nil, fmt.Errorf("unmarshal vef.data_source: %w", err)
		}

		if !reflect.ValueOf(legacy).IsZero() {
			sources["primary"] = legacy
		}
	}

	if _, ok := sources["primary"]; !ok {
		return nil, ErrPrimaryDataSourceMissing
	}

	return &config.DataSourcesConfig{Map: sources}, nil
}
