package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// TestResolveRetriesAfterAFailedOpen pins that a failed inspector open is not
// remembered. Opening queries the server, so it can fail for reasons that pass;
// caching that failure would leave schema inspection broken for the life of the
// process, recoverable only by a restart.
func TestResolveRetriesAfterAFailedOpen(t *testing.T) {
	ctx := context.Background()
	container := testx.NewPostgresContainer(ctx, t)

	unreachable := *container.DataSource
	unreachable.Port = 1

	broken, err := database.Open(unreachable)
	require.NoError(t, err, "opening a lazy *sql.DB against a dead port must succeed")

	t.Cleanup(func() { _ = broken.Close() })

	service := &DefaultService{db: broken, kind: config.Postgres}

	_, err = service.resolve()
	require.Error(t, err, "the first open must fail against an unreachable server")
	require.Nil(t, service.inspector, "a failed open must leave nothing memoized")

	// The same service, now handed a reachable connection, must open rather
	// than replay the earlier failure.
	live, err := database.Open(*container.DataSource)
	require.NoError(t, err, "opening the live container must succeed")

	t.Cleanup(func() { _ = live.Close() })

	service.db = live

	inspector, err := service.resolve()
	require.NoError(t, err, "a failure must not be cached: the next attempt has to try again")
	require.NotNil(t, inspector, "the successful open must be memoized")

	again, err := service.resolve()
	require.NoError(t, err, "a second call must succeed")
	require.Same(t, inspector, again, "success must be memoized rather than re-opened per call")
}
