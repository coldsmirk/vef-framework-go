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

	out, ok := target.(*map[string]pkgconfig.DataSourceConfig)
	if !ok {
		return errors.New("unexpected target type")
	}

	sources, ok := v.(map[string]pkgconfig.DataSourceConfig)
	if !ok {
		return errors.New("unexpected data sources value")
	}

	*out = sources

	return nil
}

func TestNewDataSourcesConfig(t *testing.T) {
	t.Run("LoadsPrimaryAndSecondary", func(t *testing.T) {
		cfg, err := newDataSourcesConfig(&DataSourcesTestConfig{
			values: map[string]any{
				"vef.data_sources": map[string]pkgconfig.DataSourceConfig{
					"primary": {Kind: pkgconfig.Postgres},
					"audit":   {Kind: pkgconfig.MySQL},
				},
			},
		})

		require.NoError(t, err, "valid multi-source config should be accepted")
		require.Equal(t, pkgconfig.Postgres, cfg.Primary().Kind, "primary should come from vef.data_sources.primary")
		require.Equal(t, pkgconfig.MySQL, cfg.Map["audit"].Kind, "secondary sources should be preserved")
	})

	t.Run("MissingPrimaryFails", func(t *testing.T) {
		_, err := newDataSourcesConfig(&DataSourcesTestConfig{
			values: map[string]any{
				"vef.data_sources": map[string]pkgconfig.DataSourceConfig{
					"audit": {Kind: pkgconfig.MySQL},
				},
			},
		})

		require.ErrorIs(t, err, ErrPrimaryDataSourceMissing, "primary source is required even when secondaries exist")
	})

	t.Run("EmptyConfigFails", func(t *testing.T) {
		_, err := newDataSourcesConfig(&DataSourcesTestConfig{values: map[string]any{}})

		require.ErrorIs(t, err, ErrPrimaryDataSourceMissing, "an absent vef.data_sources is rejected")
	})
}
