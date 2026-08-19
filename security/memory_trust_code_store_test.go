package security

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trustCodeTestTTL = time.Minute

func trustCodeTestState() TrustCodeState {
	return TrustCodeState{
		Principal: NewUser("user001", "Test User", "admin"),
		AppID:     "his",
		UserAgent: "Mozilla/5.0",
		ClientIP:  "192.168.10.9",
	}
}

func TestMemoryTrustCodeStore(t *testing.T) {
	ctx := context.Background()

	t.Run("IssueThenConsume", func(t *testing.T) {
		store := NewMemoryTrustCodeStore()

		code, err := store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
		require.NoError(t, err, "Issuing a code should succeed")
		assert.NotEmpty(t, code, "The issued code should not be empty")

		state, err := store.Consume(ctx, code)
		require.NoError(t, err, "Consuming a freshly issued code should succeed")
		require.NotNil(t, state, "Consuming should return the parked state")
		assert.Equal(t, "his", state.AppID, "The app ID should survive the round trip")
		assert.Equal(t, "Mozilla/5.0", state.UserAgent, "The user agent should survive the round trip")
		assert.Equal(t, "192.168.10.9", state.ClientIP, "The client IP should survive the round trip")
		require.NotNil(t, state.Principal, "The principal should survive the round trip")
		assert.Equal(t, "user001", state.Principal.ID, "The principal ID should survive the round trip")
		assert.Equal(t, []string{"admin"}, state.Principal.Roles, "The principal roles should survive the round trip")
	})

	t.Run("SecondConsumeFails", func(t *testing.T) {
		store := NewMemoryTrustCodeStore()

		code, err := store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
		require.NoError(t, err, "Issuing a code should succeed")

		_, err = store.Consume(ctx, code)
		require.NoError(t, err, "The first consume should succeed")

		_, err = store.Consume(ctx, code)
		assert.ErrorIs(t, err, ErrTrustCodeInvalid,
			"A code must be redeemable exactly once — a redirect URL left in browser history must not replay")
	})

	t.Run("UnknownCodeFails", func(t *testing.T) {
		store := NewMemoryTrustCodeStore()

		_, err := store.Consume(ctx, "never-issued")
		assert.ErrorIs(t, err, ErrTrustCodeInvalid, "An unknown code should be rejected")
	})

	t.Run("EmptyCodeFails", func(t *testing.T) {
		store := NewMemoryTrustCodeStore()

		_, err := store.Consume(ctx, "")
		assert.ErrorIs(t, err, ErrTrustCodeInvalid, "An empty code should be rejected without touching the cache")
	})

	t.Run("ExpiredCodeFails", func(t *testing.T) {
		store := NewMemoryTrustCodeStore()

		code, err := store.Issue(ctx, trustCodeTestState(), 10*time.Millisecond)
		require.NoError(t, err, "Issuing a code should succeed")

		time.Sleep(80 * time.Millisecond)

		_, err = store.Consume(ctx, code)
		assert.ErrorIs(t, err, ErrTrustCodeInvalid, "A code past its TTL should be rejected")
	})

	// The single-use property is the security value of the code, so it has to
	// hold under a race, not just sequentially: the cache offers no atomic
	// read-and-delete, and an unguarded Get-then-Delete would let every racing
	// request through.
	t.Run("ConcurrentConsumeAdmitsExactlyOne", func(t *testing.T) {
		const racers = 32

		store := NewMemoryTrustCodeStore()

		code, err := store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
		require.NoError(t, err, "Issuing a code should succeed")

		var (
			succeeded atomic.Int32
			start     sync.WaitGroup
			done      sync.WaitGroup
		)

		start.Add(1)
		done.Add(racers)

		for range racers {
			go func() {
				defer done.Done()

				start.Wait()

				if _, err := store.Consume(ctx, code); err == nil {
					succeeded.Add(1)
				}
			}()
		}

		start.Done()
		done.Wait()

		assert.Equal(t, int32(1), succeeded.Load(),
			"Exactly one of the racing consumers may redeem the code")
	})
}
