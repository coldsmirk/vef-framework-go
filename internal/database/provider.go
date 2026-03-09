package database

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/mysql"
	"github.com/coldsmirk/vef-framework-go/internal/database/postgres"
	"github.com/coldsmirk/vef-framework-go/internal/database/sqlite"
)

// DatabaseProvider defines the contract for database-specific connection and validation logic.
type DatabaseProvider interface {
	// Connect establishes a database connection and returns the sql.DB, dialect, and any error.
	Connect(config *config.DataSourceConfig) (*sql.DB, schema.Dialect, error)
	// Kind returns the database kind this provider handles (postgres, mysql, or sqlite).
	Kind() config.DBKind
	// ValidateConfig checks that the data source configuration is valid before attempting to connect.
	ValidateConfig(config *config.DataSourceConfig) error
	// QueryVersion queries and returns the database server version string.
	QueryVersion(db *bun.DB) (string, error)
}

type providerRegistry struct {
	providers map[config.DBKind]DatabaseProvider
}

func newProviderRegistry() *providerRegistry {
	registry := &providerRegistry{
		providers: make(map[config.DBKind]DatabaseProvider),
	}

	registry.register(sqlite.NewProvider())
	registry.register(postgres.NewProvider())
	registry.register(mysql.NewProvider())

	return registry
}

func (r *providerRegistry) register(provider DatabaseProvider) {
	r.providers[provider.Kind()] = provider
}

func (r *providerRegistry) provider(dbKind config.DBKind) (DatabaseProvider, bool) {
	provider, exists := r.providers[dbKind]

	return provider, exists
}

var registry = newProviderRegistry()
