package config

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

// DataSourceConfig defines database connection settings.
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
