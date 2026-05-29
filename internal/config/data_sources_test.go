package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	pkgconfig "github.com/coldsmirk/vef-framework-go/config"
)

type DataSourcesTestConfig struct {
	values map[string]any
}

func (c *DataSourcesTestConfig) Unmarshal(key string, target any) error {
	v, ok := c.values[key]
	if !ok {
		return nil
	}

	switch out := target.(type) {
	case *map[string]pkgconfig.DataSourceConfig:
		sources, ok := v.(map[string]pkgconfig.DataSourceConfig)
		if !ok {
			return errors.New("unexpected data sources target")
		}

		*out = sources

	case *pkgconfig.DataSourceConfig:
		source, ok := v.(pkgconfig.DataSourceConfig)
		if !ok {
			return errors.New("unexpected data source target")
		}

		*out = source

	default:
		return errors.New("unexpected target type")
	}

	return nil
}

func TestNewDataSourcesConfig(t *testing.T) {
	t.Run("NewConfigWinsWhenBothKeysExist", func(t *testing.T) {
		cfg, err := newDataSourcesConfig(&DataSourcesTestConfig{
			values: map[string]any{
				"vef.data_sources": map[string]pkgconfig.DataSourceConfig{
					"primary": {Kind: pkgconfig.Postgres},
					"audit":   {Kind: pkgconfig.MySQL},
				},
				"vef.data_source": pkgconfig.DataSourceConfig{Kind: pkgconfig.SQLite},
			},
		})

		require.NoError(t, err, "new data sources config should be accepted")
		require.Equal(t, pkgconfig.Postgres, cfg.Primary().Kind, "primary should come from vef.data_sources")
		require.Equal(t, pkgconfig.MySQL, cfg.Map["audit"].Kind, "secondary sources should be preserved")
	})

	t.Run("LegacyDataSourceBecomesPrimary", func(t *testing.T) {
		cfg, err := newDataSourcesConfig(&DataSourcesTestConfig{
			values: map[string]any{
				"vef.data_source": pkgconfig.DataSourceConfig{Kind: pkgconfig.SQLite, Path: "legacy.db"},
			},
		})

		require.NoError(t, err, "legacy data source config should be accepted")
		require.Equal(t, pkgconfig.SQLite, cfg.Primary().Kind, "legacy source should become primary")
		require.Equal(t, "legacy.db", cfg.Primary().Path, "legacy source fields should be preserved")
	})

	t.Run("MissingPrimaryFails", func(t *testing.T) {
		_, err := newDataSourcesConfig(&DataSourcesTestConfig{
			values: map[string]any{
				"vef.data_sources": map[string]pkgconfig.DataSourceConfig{
					"audit": {Kind: pkgconfig.MySQL},
				},
			},
		})

		require.ErrorIs(t, err, ErrPrimaryDataSourceMissing, "primary source is required")
	})
}
