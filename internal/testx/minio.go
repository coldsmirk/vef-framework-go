package testx

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/wait"

	miniogo "github.com/minio/minio-go/v7"

	"github.com/coldsmirk/vef-framework-go/config"
)

// s3ProbeInterval paces the readiness probe below. The window it covers is
// short, so a tight interval keeps the common case near-instant.
const s3ProbeInterval = 100 * time.Millisecond

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

	host, port := hostPort(ctx, t, container, "9000")
	terminateOnCleanup(ctx, t, container, "minio")

	cfg := &config.MinIOConfig{
		Endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
		AccessKey: TestMinIOAccessKey,
		SecretKey: TestMinIOSecretKey,
		UseSSL:    false,
		Bucket:    TestMinIOBucket,
	}

	waitForS3API(ctx, t, cfg)

	return &MinIOContainer{container: container, MinIO: cfg}
}

// waitForS3API blocks until the S3 endpoint actually serves a request.
//
// The health endpoints above are not that guarantee: MinIO answers
// /minio/health/live and /minio/health/ready while its object layer is still
// coming up, so the first S3 call a test makes can still fail with "Server not
// initialized yet". Polling the very operation the caller will perform is the
// only readiness signal that means what the caller needs it to mean — a health
// endpoint reports on the process, this reports on the API.
func waitForS3API(ctx context.Context, t testing.TB, cfg *config.MinIOConfig) {
	t.Helper()

	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	require.NoError(t, err, "Building the MinIO readiness probe client should succeed")

	probeCtx, cancel := context.WithTimeout(ctx, DefaultContainerTimeout)
	defer cancel()

	ticker := time.NewTicker(s3ProbeInterval)
	defer ticker.Stop()

	started := time.Now()

	// BucketExists is the call the storage initializer makes first, and it
	// reports a missing bucket as (false, nil) — so a nil error means the API
	// served the request, not that the bucket is there.
	for attempt := 1; ; attempt++ {
		_, err = client.BucketExists(probeCtx, cfg.Bucket)
		if err == nil {
			// Logged only when the endpoint was not ready on the first try, so
			// the window this probe exists to cover leaves a trace instead of
			// resurfacing later as an unexplained failure somewhere else.
			if attempt > 1 {
				t.Logf("MinIO S3 endpoint became ready after %s (%d attempts)", time.Since(started).Round(time.Millisecond), attempt)
			}

			return
		}

		select {
		case <-probeCtx.Done():
			require.NoError(t, err, "The MinIO S3 endpoint should start serving within %s", DefaultContainerTimeout)

			return
		case <-ticker.C:
		}
	}
}

type MinIOContainer struct {
	MinIO *config.MinIOConfig

	container *minio.MinioContainer
}
