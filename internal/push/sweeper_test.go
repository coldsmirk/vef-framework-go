package push

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/push"
	"github.com/coldsmirk/vef-framework-go/security"
)

// ErroringSessionStore fails every lookup, standing in for an unavailable
// session backend.
type ErroringSessionStore struct{}

func (*ErroringSessionStore) Create(context.Context, string, security.Session, time.Duration) error {
	return nil
}

func (*ErroringSessionStore) Lookup(context.Context, string) (*security.Session, error) {
	return nil, errors.New("store unavailable")
}

func (*ErroringSessionStore) Renew(context.Context, string, time.Time, time.Duration) error {
	return nil
}

func (*ErroringSessionStore) Revoke(context.Context, string) error {
	return nil
}

func (*ErroringSessionStore) ListByUser(context.Context, string) ([]security.Session, error) {
	return nil, nil
}

func (*ErroringSessionStore) RevokeUser(context.Context, string) error {
	return nil
}

func TestSessionSweep(t *testing.T) {
	ctx := context.Background()

	t.Run("KicksMissingSessionsOnly", func(t *testing.T) {
		store := security.NewMemorySessionStore()
		require.NoError(t, store.Create(ctx, "hash-live",
			security.Session{ID: "s1", UserID: "alice", ExpiresAt: time.Now().Add(time.Hour)}, time.Hour),
			"Fixture session should be created")

		hub := NewHub(new(config.PushConfig))
		live := NewTestConnection("alice", "hash-live")
		revoked := NewTestConnection("bob", "hash-gone")
		jwt := NewTestConnection("carol", "")

		for _, conn := range []*connection{live, revoked, jwt} {
			require.NoError(t, hub.register(conn), "Fixture connections should register")
		}

		newSessionSweeper(hub, store, time.Minute).sweep(ctx)

		assert.False(t, ConnectionClosing(live), "A live session must survive the sweep")
		assert.False(t, ConnectionClosing(jwt), "A jwt connection carries no session and must be left alone")
		require.True(t, ConnectionClosing(revoked), "A connection without a session must be closed")
		assert.Equal(t, push.CloseSessionInvalid, revoked.closeCode, "The kick should use the session-invalid close code")
	})

	t.Run("FailsOpenOnStoreError", func(t *testing.T) {
		hub := NewHub(new(config.PushConfig))
		conn := NewTestConnection("alice", "hash-a")
		require.NoError(t, hub.register(conn), "Fixture connection should register")

		newSessionSweeper(hub, new(ErroringSessionStore), time.Minute).sweep(ctx)

		assert.False(t, ConnectionClosing(conn), "A store error must not kick connections (fail open)")
	})

	t.Run("StartStop", func(t *testing.T) {
		sweeper := newSessionSweeper(NewHub(new(config.PushConfig)), security.NewMemorySessionStore(), time.Hour)
		sweeper.start()
		sweeper.shutdown()

		select {
		case <-sweeper.stopped:
		default:
			assert.Fail(t, "shutdown should wait for the sweep loop to exit")
		}
	})
}
