package database

import (
	"context"
	"database/sql"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/mysql"
	"github.com/coldsmirk/vef-framework-go/internal/database/postgres"
	"github.com/coldsmirk/vef-framework-go/internal/database/sqlite"
)

// Provider defines the contract for database-specific connection and validation logic.
type Provider interface {
	// Connect establishes a database connection and returns the *sql.DB and any error.
	Connect(config *config.DataSourceConfig) (*sql.DB, error)
	// Kind returns the database kind this provider handles (postgres, mysql, or sqlite).
	Kind() config.DBKind
	// ValidateConfig checks that the data source configuration is valid before attempting to connect.
	ValidateConfig(config *config.DataSourceConfig) error
	// Version queries and returns the database server version string.
	Version(ctx context.Context, db *sql.DB) (string, error)
}

type providerRegistry struct {
	providers map[config.DBKind]Provider
}

func newProviderRegistry() *providerRegistry {
	registry := &providerRegistry{
		providers: make(map[config.DBKind]Provider),
	}

	registry.register(sqlite.NewProvider())
	registry.register(postgres.NewProvider())
	registry.register(mysql.NewProvider())

	return registry
}

func (r *providerRegistry) register(provider Provider) {
	r.providers[provider.Kind()] = provider
}

func (r *providerRegistry) lookup(kind config.DBKind) (Provider, bool) {
	provider, exists := r.providers[kind]

	return provider, exists
}

var registry = newProviderRegistry()

// SupportsKind reports whether a connection provider is registered for kind.
// It lets higher layers (e.g. orm.DialectFor) assert that the connector and
// dialect halves of a dialect agree on the same set of supported kinds.
func SupportsKind(kind config.DBKind) bool {
	_, ok := registry.lookup(kind)

	return ok
}
