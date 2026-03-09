package testx

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// DBSetupFunc creates a DataSourceConfig (spinning up a container if needed).
type DBSetupFunc func(ctx context.Context, t *testing.T) *config.DataSourceConfig

var providers = []struct {
	name  string
	setup DBSetupFunc
}{
	{"Postgres", func(ctx context.Context, t *testing.T) *config.DataSourceConfig {
		return NewPostgresContainer(ctx, t).DataSource
	}},
	{"MySQL", func(ctx context.Context, t *testing.T) *config.DataSourceConfig {
		return NewMySQLContainer(ctx, t).DataSource
	}},
	{"SQLite", func(_ context.Context, t *testing.T) *config.DataSourceConfig {
		return &config.DataSourceConfig{
			Kind: config.SQLite,
			Path: filepath.Join(t.TempDir(), "test.db"),
		}
	}},
}

// ForEachDB runs fn once per enabled database, managing container lifecycle automatically.
// Test hierarchy: t.Run("<DisplayName>", fn).
func ForEachDB(t *testing.T, fn func(t *testing.T, env *DBEnv)) {
	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			ctx := context.Background()
			dataSource := p.setup(ctx, t)
			env := newDBEnv(t, ctx, dataSource)
			fn(t, env)
		})
	}
}

// newDBEnv creates a complete DBEnv with database connection and automatic cleanup.
func newDBEnv(t *testing.T, ctx context.Context, ds *config.DataSourceConfig) *DBEnv {
	db, err := database.New(ds)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Error closing database connection for %s: %v", ds.Kind, err)
		}
	})

	return &DBEnv{
		T:     t,
		Ctx:   ctx,
		RawDB: db.DB,
		BunDB: db,
		DB:    orm.New(db),
		DS:    ds,
	}
}
