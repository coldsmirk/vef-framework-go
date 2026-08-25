package project

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	ischema "github.com/coldsmirk/vef-framework-go/internal/schema"
	"github.com/coldsmirk/vef-framework-go/mapx"
	"github.com/coldsmirk/vef-framework-go/schema"
)

// ErrDataSourceMissing reports that the requested data source is absent from
// the project's application.toml.
var ErrDataSourceMissing = errors.New("data source not declared in application.toml")

// AppConfigName is the runtime configuration file the framework loads and the
// generators read data source credentials from.
const AppConfigName = "application.toml"

// Inspector pairs a live schema.Service with the connection behind it so the
// caller can release it.
type Inspector struct {
	// Service reads table metadata from the connected data source.
	Service schema.Service
	// Kind is the data source's dialect, which drives type mapping.
	Kind config.DBKind

	close func() error
}

// Close releases the inspection connection.
func (i *Inspector) Close() error {
	if i == nil || i.close == nil {
		return nil
	}

	return i.close()
}

// Inspect opens the named data source (empty means primary) declared by the
// project's application.toml and returns a schema inspector over it.
//
// configPath overrides the file location; when empty the project's
// configs/application.toml is used, matching where the framework itself looks
// first.
func (p *Project) Inspect(configPath, source string) (*Inspector, error) {
	sources, err := p.loadDataSources(configPath)
	if err != nil {
		return nil, err
	}

	if source == "" {
		source = config.PrimaryDataSourceName
	}

	dsCfg, ok := sources.Map[source]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDataSourceMissing, source)
	}

	db, err := database.Open(dsCfg)
	if err != nil {
		return nil, fmt.Errorf("open data source %s: %w", source, err)
	}

	// schema.NewService resolves the active schema from the primary entry, so
	// hand it a config whose primary is the source actually being inspected.
	service, err := ischema.NewService(db, &config.DataSourcesConfig{
		Map: map[string]config.DataSourceConfig{config.PrimaryDataSourceName: dsCfg},
	})
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("inspect data source %s: %w", source, err)
	}

	return &Inspector{Service: service, Kind: dsCfg.Kind, close: db.Close}, nil
}

// loadDataSources reads vef.data_sources out of the project's application.toml
// using the framework's own decoding rules, so a value the running application
// accepts is a value the generators accept.
func (p *Project) loadDataSources(configPath string) (*config.DataSourcesConfig, error) {
	if configPath == "" {
		configPath = filepath.Join(p.Root, "configs", AppConfigName)
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}

	sources := map[string]config.DataSourceConfig{}
	if err := v.UnmarshalKey("vef.data_sources", &sources, decodeUsingConfigTag); err != nil {
		return nil, fmt.Errorf("parse vef.data_sources in %s: %w", configPath, err)
	}

	return &config.DataSourcesConfig{Map: sources}, nil
}

// decodeUsingConfigTag mirrors internal/config's decoder so the CLI reads
// application.toml exactly as the framework does.
var decodeUsingConfigTag viper.DecoderConfigOption = func(c *mapstructure.DecoderConfig) {
	c.TagName = "config"
	c.IgnoreUntaggedFields = true
	c.DecodeHook = mapx.DecoderHook
}
