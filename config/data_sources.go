package config

// PrimaryDataSourceName is the reserved name of the data source declared
// under vef.data_sources.primary. It is mandatory and powers the
// framework-wide orm.DB injection. config is the lowest layer, so this is
// the canonical home for the constant.
const PrimaryDataSourceName = "primary"

// DBKind represents supported database kinds.
type DBKind string

// Supported database kinds.
const (
	Oracle    DBKind = "oracle"
	SQLServer DBKind = "sqlserver"
	Postgres  DBKind = "postgres"
	MySQL     DBKind = "mysql"
	SQLite    DBKind = "sqlite"
)

// DataSourceConfig defines database connection settings for one named
// data source. Sources live under `vef.data_sources.<name>` in the TOML
// configuration; the entry under name "primary" is mandatory and is the
// source exposed in the FX container as orm.DB.
type DataSourceConfig struct {
	Kind           DBKind `config:"type"`
	Host           string `config:"host"`
	Port           uint16 `config:"port"`
	User           string `config:"user"`
	Password       string `config:"password"`
	Database       string `config:"database"`
	Schema         string `config:"schema"`
	Path           string `config:"path"`
	EnableSQLGuard bool   `config:"enable_sql_guard"`
}

// DataSourcesConfig groups every entry under vef.data_sources. Map keys are
// the data source names (lower-case, alphanumeric); the entry named "primary"
// is mandatory and powers the framework-wide orm.DB injection.
type DataSourcesConfig struct {
	Map map[string]DataSourceConfig
}

// Primary returns the configuration for the primary data source, or the zero
// value if absent. The framework validates the primary entry's presence at
// startup, so callers that obtain this config from the framework can rely on
// it being populated.
func (c *DataSourcesConfig) Primary() DataSourceConfig {
	return c.Map[PrimaryDataSourceName]
}
