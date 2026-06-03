package testx

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver for the ForSQL readiness probe

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
			// Probe the mapped port with a real SQL connection: this is the
			// only strategy that proves the server accepts queries end-to-end.
			// A port-open or log-line check returns before MySQL is truly ready
			// and yields "invalid connection" on the first query.
			wait.ForSQL("3306/tcp", "mysql", func(host, port string) string {
				return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
					TestUsername, TestPassword, host, port, TestDatabaseName)
			}).WithStartupTimeout(DefaultContainerTimeout),
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
