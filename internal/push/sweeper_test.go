package push

import (
	"context"
	"errors"
	"sync"
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

// BlockingSessionStore blocks every lookup until the caller's context is
// canceled, standing in for a legitimate store waiting on a dead backend.
type BlockingSessionStore struct {
	ErroringSessionStore

	entered chan struct{}
	once    sync.Once
}

func (b *BlockingSessionStore) Lookup(ctx context.Context, _ string) (*security.Session, error) {
	b.once.Do(func() { close(b.entered) })
	<-ctx.Done()

	return nil, ctx.Err()
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

	t.Run("ShutdownCancelsInFlightLookups", func(t *testing.T) {
		hub := NewHub(new(config.PushConfig))
		require.NoError(t, hub.register(NewTestConnection("alice", "hash-a")), "Fixture connection should register")

		store := &BlockingSessionStore{entered: make(chan struct{})}
		sweeper := newSessionSweeper(hub, store, 10*time.Millisecond)
		sweeper.start()

		select {
		case <-store.entered:
		case <-time.After(3 * time.Second):
			require.FailNow(t, "The sweep should reach the store lookup")
		}

		done := make(chan struct{})
		go func() {
			sweeper.shutdown()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			assert.Fail(t, "shutdown must cancel the in-flight lookup instead of waiting out the store")
		}
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
