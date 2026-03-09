package testx

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/coldsmirk/vef-framework-go/config"
)

func NewMinIOContainer(ctx context.Context, t testing.TB) *MinIOContainer {
	t.Helper()

	container, err := minio.Run(
		ctx,
		MinIOImage,
		minio.WithUsername(TestMinIOAccessKey),
		minio.WithPassword(TestMinIOSecretKey),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForListeningPort("9000/tcp"),
				wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
				wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp"),
			).WithDeadline(DefaultContainerTimeout),
		),
	)
	require.NoError(t, err)
	t.Log("MinIO container started successfully")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	mc := &MinIOContainer{
		container: container,
		MinIO: &config.MinIOConfig{
			Endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
			AccessKey: TestMinIOAccessKey,
			SecretKey: TestMinIOSecretKey,
			UseSSL:    false,
			Bucket:    TestMinIOBucket,
		},
	}

	t.Cleanup(func() {
		if err := mc.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate MinIO container: %v", err)
		}
	})

	return mc
}

type MinIOContainer struct {
	MinIO *config.MinIOConfig

	container *minio.MinioContainer
}

func (c *MinIOContainer) Terminate(ctx context.Context) error {
	return c.container.Terminate(ctx)
}
