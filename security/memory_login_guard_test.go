package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lockPolicy() LockoutPolicy {
	return LockoutPolicy{
		MaxFailures:  3,
		Window:       time.Minute,
		LockDuration: 15 * time.Minute,
		Strategy:     LockoutStrategyLock,
		Key:          LockoutKeyUserIP,
	}
}

func TestMemoryLoginGuardLockStrategy(t *testing.T) {
	ctx := context.Background()
	attempt := LoginAttempt{Identity: "alice", ClientIP: "10.0.0.1"}

	t.Run("AllowsUntilThreshold", func(t *testing.T) {
		guard := NewMemoryLoginGuard(lockPolicy())

		for i := 1; i < 3; i++ {
			decision, err := guard.RecordFailure(ctx, attempt)
			require.NoError(t, err, "recording a failure should not error")
			assert.True(t, decision.Allowed, "should stay allowed below the threshold")
		}

		decision, err := guard.Check(ctx, attempt)
		require.NoError(t, err, "check should not error")
		assert.True(t, decision.Allowed, "should still be allowed after 2 of 3 failures")
	})

	t.Run("LocksAtThreshold", func(t *testing.T) {
		guard := NewMemoryLoginGuard(lockPolicy())

		var last LoginDecision
		for range 3 {
			d, err := guard.RecordFailure(ctx, attempt)
			require.NoError(t, err, "recording a failure should not error")

			last = d
		}

		assert.False(t, last.Allowed, "third failure should trip the lock")
		assert.InDelta(t, (15 * time.Minute).Seconds(), last.RetryAfter.Seconds(), 2, "retry-after should be the lock duration")

		decision, err := guard.Check(ctx, attempt)
		require.NoError(t, err, "check should not error")
		assert.False(t, decision.Allowed, "a locked identity should be denied on check")
		assert.Positive(t, decision.RetryAfter, "a locked identity should report a positive retry-after")
	})

	t.Run("SuccessClearsFailures", func(t *testing.T) {
		guard := NewMemoryLoginGuard(lockPolicy())

		for range 3 {
			_, err := guard.RecordFailure(ctx, attempt)
			require.NoError(t, err, "recording a failure should not error")
		}

		require.NoError(t, guard.RecordSuccess(ctx, attempt), "recording success should not error")

		decision, err := guard.Check(ctx, attempt)
		require.NoError(t, err, "check should not error")
		assert.True(t, decision.Allowed, "a successful login should clear the lock")
	})

	t.Run("IsolatesByKey", func(t *testing.T) {
		guard := NewMemoryLoginGuard(lockPolicy())

		for range 3 {
			_, err := guard.RecordFailure(ctx, attempt)
			require.NoError(t, err, "recording a failure should not error")
		}

		other := LoginAttempt{Identity: "alice", ClientIP: "10.0.0.2"}
		decision, err := guard.Check(ctx, other)
		require.NoError(t, err, "check should not error")
		assert.True(t, decision.Allowed, "a different source IP must not inherit another key's lock")
	})
}

func TestMemoryLoginGuardBackoffStrategy(t *testing.T) {
	ctx := context.Background()
	attempt := LoginAttempt{Identity: "bob", ClientIP: "10.0.0.9"}
	guard := NewMemoryLoginGuard(LockoutPolicy{
		MaxFailures: 1,
		Window:      time.Minute,
		Strategy:    LockoutStrategyBackoff,
		BackoffBase: time.Second,
		BackoffMax:  8 * time.Second,
		Key:         LockoutKeyUserIP,
	})

	wantSeconds := []float64{1, 2, 4, 8, 8}
	for i, want := range wantSeconds {
		decision, err := guard.RecordFailure(ctx, attempt)
		require.NoError(t, err, "recording a failure should not error")
		assert.False(t, decision.Allowed, "backoff should block from the first failure past the threshold")
		assert.InDelta(t, want, decision.RetryAfter.Seconds(), 0.5, "backoff delay at failure %d should escalate then cap", i+1)
	}
}

// TestMemoryLoginGuardWindowExpiry verifies the failure counter resets once the
// counting window elapses.
func TestMemoryLoginGuardWindowExpiry(t *testing.T) {
	ctx := context.Background()
	attempt := LoginAttempt{Identity: "carol", ClientIP: "10.0.0.3"}
	guard := NewMemoryLoginGuard(LockoutPolicy{
		MaxFailures:  2,
		Window:       50 * time.Millisecond,
		LockDuration: 50 * time.Millisecond,
		Strategy:     LockoutStrategyLock,
		Key:          LockoutKeyUserIP,
	})

	_, err := guard.RecordFailure(ctx, attempt)
	require.NoError(t, err, "recording a failure should not error")

	time.Sleep(80 * time.Millisecond)

	// The first failure has aged out of the window, so a fresh failure is the
	// first in a new window and must not immediately lock.
	decision, err := guard.RecordFailure(ctx, attempt)
	require.NoError(t, err, "recording a failure should not error")
	assert.True(t, decision.Allowed, "a failure after the window expires should start a new count")
}
