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

// SSLMode controls the TLS posture of a database connection. The values follow
// the PostgreSQL libpq vocabulary so operators familiar with `sslmode` can
// reuse it. It applies to the network-based dialects (Postgres, MySQL); SQLite
// is a local file and ignores it.
type SSLMode string

// Supported SSL modes. The empty value is treated as SSLDisable so a
// zero-config data source keeps connecting over plaintext exactly as before —
// TLS is strictly opt-in.
const (
	// SSLDisable connects without TLS. This is the default when the field
	// is omitted.
	SSLDisable SSLMode = "disable"
	// SSLRequire negotiates TLS but performs no certificate or hostname
	// verification (encryption without authentication; vulnerable to MITM).
	SSLRequire SSLMode = "require"
	// SSLVerifyCA negotiates TLS and verifies that the server certificate
	// chains to a trusted CA, but does not check the hostname.
	SSLVerifyCA SSLMode = "verify-ca"
	// SSLVerifyFull negotiates TLS and verifies both the CA chain and that
	// the certificate matches the server hostname. This is the strongest mode.
	SSLVerifyFull SSLMode = "verify-full"
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

	// SSLMode selects the TLS posture for network dialects (Postgres, MySQL).
	// It defaults to SSLDisable (plaintext) when omitted, so TLS is opt-in
	// and existing zero-config deployments are unaffected. SQLite ignores it.
	SSLMode SSLMode `config:"ssl_mode"`
	// SSLRootCert is an optional path to a PEM file holding the CA
	// certificate(s) used to verify the server in the verify-ca / verify-full
	// modes. When empty, the host's system certificate pool is used.
	SSLRootCert string `config:"ssl_root_cert"`
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
