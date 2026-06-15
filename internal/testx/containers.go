package testx

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// hostPort resolves a started container's external host and the host-side
// mapping for the given container port, failing the test if either lookup
// errors. The returned network.Port exposes both Num() (int) and Port()
// (string) so each constructor can build its own config shape.
func hostPort(ctx context.Context, t testing.TB, c testcontainers.Container, port string) (string, network.Port) {
	t.Helper()

	host, err := c.Host(ctx)
	require.NoError(t, err, "container.Host should succeed")

	mapped, err := c.MappedPort(ctx, port)
	require.NoError(t, err, "container.MappedPort(%s) should succeed", port)

	return host, mapped
}

// terminateOnCleanup registers a t.Cleanup that terminates the container,
// logging a single uniformly-formatted message on failure. label names the
// container kind in that message.
func terminateOnCleanup(ctx context.Context, t testing.TB, c testcontainers.Container, label string) {
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate %s container: %v", label, err)
		}
	})
}
