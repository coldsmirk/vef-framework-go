package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/coldsmirk/vef-framework-go/config"
)

func NewPostgresContainer(ctx context.Context, t testing.TB) *PostgresContainer {
	t.Helper()

	container, err := postgres.Run(
		ctx,
		PostgresImage,
		postgres.WithDatabase(TestDatabaseName),
		postgres.WithUsername(TestUsername),
		postgres.WithPassword(TestPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("PostgreSQL container started successfully")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pc := &PostgresContainer{
		container: container,
		DataSource: &config.DataSourceConfig{
			Kind:     "postgres",
			Host:     host,
			Port:     uint16(port.Int()),
			User:     TestUsername,
			Password: TestPassword,
			Database: TestDatabaseName,
		},
	}

	t.Cleanup(func() {
		if err := pc.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate postgres container: %v", err)
		}
	})

	return pc
}

type PostgresContainer struct {
	DataSource *config.DataSourceConfig

	container *postgres.PostgresContainer
}

func (c *PostgresContainer) Terminate(ctx context.Context) error {
	return c.container.Terminate(ctx)
}
