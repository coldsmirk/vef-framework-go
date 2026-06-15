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

	host, port := hostPort(ctx, t, container, "5432")
	terminateOnCleanup(ctx, t, container, "postgres")

	return &PostgresContainer{
		container: container,
		DataSource: &config.DataSourceConfig{
			Kind:     "postgres",
			Host:     host,
			Port:     port.Num(),
			User:     TestUsername,
			Password: TestPassword,
			Database: TestDatabaseName,
		},
	}
}

type PostgresContainer struct {
	DataSource *config.DataSourceConfig

	container *postgres.PostgresContainer
}
