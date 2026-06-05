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
			// Wait for the real server's startup log line. MySQL's entrypoint
			// runs a temporary init server (logged as "port: 0") before the real
			// one binds 3306, so this specific line marks genuine readiness.
			// A host-port SQL probe is unreliable here: during the init window
			// Docker's port forwarder accepts the mapped-port connection but
			// never completes the handshake, hanging the probe for the whole
			// startup budget. This mirrors the upstream module default and the
			// log-based wait used for Postgres; a host SQL connection succeeds
			// immediately once this line appears.
			wait.ForLog("port: 3306  MySQL Community Server").
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
			Port:     port.Num(),
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
