package testx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/coldsmirk/vef-framework-go/config"
)

func NewMySQLContainer(ctx context.Context, t testing.TB) *MySQLContainer {
	t.Helper()

	container, err := mysql.Run(
		ctx,
		MySQLImage,
		mysql.WithDatabase(TestDatabaseName),
		mysql.WithUsername(TestUsername),
		mysql.WithPassword(TestPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("port: 3306  MySQL Community Server - GPL").
				WithStartupTimeout(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("MySQL container started successfully")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "3306")
	require.NoError(t, err)

	mc := &MySQLContainer{
		container: container,
		DataSource: &config.DataSourceConfig{
			Kind:     "mysql",
			Host:     host,
			Port:     uint16(port.Int()),
			User:     TestUsername,
			Password: TestPassword,
			Database: TestDatabaseName,
		},
	}

	t.Cleanup(func() {
		if err := mc.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate mysql container: %v", err)
		}
	})

	return mc
}

type MySQLContainer struct {
	DataSource *config.DataSourceConfig

	container *mysql.MySQLContainer
}

func (c *MySQLContainer) Terminate(ctx context.Context) error {
	return c.container.Terminate(ctx)
}
