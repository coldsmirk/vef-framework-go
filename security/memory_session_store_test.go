package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeSession(id, userID string, expiresAt time.Time) Session {
	now := time.Now()

	return Session{
		ID:         id,
		UserID:     userID,
		Principal:  NewUser(userID, "User "+userID),
		ClientIP:   "10.0.0.1",
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt,
	}
}

func TestMemorySessionStore(t *testing.T) {
	ctx := context.Background()
	future := time.Now().Add(time.Hour)

	t.Run("CreateAndLookup", func(t *testing.T) {
		store := NewMemorySessionStore()
		require.NoError(t, store.Create(ctx, "hash-1", makeSession("s1", "u1", future), time.Hour), "create should succeed")

		got, err := store.Lookup(ctx, "hash-1")
		require.NoError(t, err, "lookup should not error")
		require.NotNil(t, got, "an active session should be found")
		assert.Equal(t, "s1", got.ID, "lookup should return the session")
		assert.Equal(t, "u1", got.Principal.ID, "lookup should carry the principal snapshot")
	})

	t.Run("LookupMissing", func(t *testing.T) {
		store := NewMemorySessionStore()

		got, err := store.Lookup(ctx, "nope")
		require.NoError(t, err, "lookup of an unknown token should not error")
		assert.Nil(t, got, "an unknown token should resolve to no session")
	})

	t.Run("LookupExpiredReturnsNil", func(t *testing.T) {
		store := NewMemorySessionStore()
		require.NoError(t, store.Create(ctx, "hash-x", makeSession("sx", "u1", time.Now().Add(-time.Minute)), time.Hour), "create should succeed")

		got, err := store.Lookup(ctx, "hash-x")
		require.NoError(t, err, "lookup should not error")
		assert.Nil(t, got, "an expired session should not be returned")
	})

	t.Run("RenewExtendsExpiry", func(t *testing.T) {
		store := NewMemorySessionStore()
		require.NoError(t, store.Create(ctx, "hash-2", makeSession("s2", "u1", time.Now().Add(time.Minute)), time.Minute), "create should succeed")

		newExpiry := time.Now().Add(2 * time.Hour)
		require.NoError(t, store.Renew(ctx, "hash-2", newExpiry, 2*time.Hour), "renew should succeed")

		got, err := store.Lookup(ctx, "hash-2")
		require.NoError(t, err, "lookup should not error")
		require.NotNil(t, got, "the renewed session should still be found")
		assert.WithinDuration(t, newExpiry, got.ExpiresAt, time.Second, "renew should extend the expiry")
	})

	t.Run("RevokeRemovesSession", func(t *testing.T) {
		store := NewMemorySessionStore()
		require.NoError(t, store.Create(ctx, "hash-3", makeSession("s3", "u1", future), time.Hour), "create should succeed")

		require.NoError(t, store.Revoke(ctx, "s3"), "revoke should succeed")

		got, err := store.Lookup(ctx, "hash-3")
		require.NoError(t, err, "lookup should not error")
		assert.Nil(t, got, "a revoked session should be gone")
	})

	t.Run("ListByUserNewestFirstAndIsolated", func(t *testing.T) {
		store := NewMemorySessionStore()
		older := makeSession("a", "u1", future)
		older.LastSeenAt = time.Now().Add(-time.Hour)
		newer := makeSession("b", "u1", future)

		require.NoError(t, store.Create(ctx, "ha", older, time.Hour), "create should succeed")
		require.NoError(t, store.Create(ctx, "hb", newer, time.Hour), "create should succeed")
		require.NoError(t, store.Create(ctx, "hc", makeSession("c", "u2", future), time.Hour), "create should succeed")

		sessions, err := store.ListByUser(ctx, "u1")
		require.NoError(t, err, "list should not error")
		require.Len(t, sessions, 2, "only the user's own sessions should be listed")
		assert.Equal(t, "b", sessions[0].ID, "the most recently seen session should sort first")
		assert.Equal(t, "a", sessions[1].ID, "the older session should sort last")
	})

	t.Run("RevokeUserClearsOnlyThatUser", func(t *testing.T) {
		store := NewMemorySessionStore()
		require.NoError(t, store.Create(ctx, "h1", makeSession("s1", "u1", future), time.Hour), "create should succeed")
		require.NoError(t, store.Create(ctx, "h2", makeSession("s2", "u1", future), time.Hour), "create should succeed")
		require.NoError(t, store.Create(ctx, "h3", makeSession("s3", "u2", future), time.Hour), "create should succeed")

		require.NoError(t, store.RevokeUser(ctx, "u1"), "revoke-user should succeed")

		u1, err := store.ListByUser(ctx, "u1")
		require.NoError(t, err, "list should not error")
		assert.Empty(t, u1, "the target user's sessions should be cleared")

		other, err := store.Lookup(ctx, "h3")
		require.NoError(t, err, "lookup should not error")
		assert.NotNil(t, other, "another user's session must be untouched")
	})
}
