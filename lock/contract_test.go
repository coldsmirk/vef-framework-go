package lock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runLockerContract exercises the behavior every Locker implementation must
// share. steal forcibly evicts the named lock behind the locker's back,
// simulating a lease lost to expiry or takeover.
func runLockerContract(t *testing.T, locker Locker, steal func(t *testing.T, name string)) {
	t.Helper()

	ctx := context.Background()
	name := func(t *testing.T) string { return "contract:" + t.Name() }

	t.Run("MutualExclusion", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "the first acquisition should succeed")

		_, err = locker.TryAcquire(ctx, name(t))
		require.ErrorIs(t, err, ErrNotAcquired, "a held lock must refuse a second holder")

		require.NoError(t, held.Release(ctx), "release should succeed")

		reacquired, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "a released lock should be acquirable again")
		require.NoError(t, reacquired.Release(ctx), "cleanup release should succeed")
	})

	t.Run("WaitAcquiresWhenReleased", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "seeding the held lock should succeed")

		go func() {
			time.Sleep(150 * time.Millisecond)

			_ = held.Release(context.Background())
		}()

		waited, err := locker.Acquire(ctx, name(t), WithWait(3*time.Second), WithRetryInterval(20*time.Millisecond))
		require.NoError(t, err, "a waiting acquire should win once the holder releases")
		require.NoError(t, waited.Release(ctx), "cleanup release should succeed")
	})

	t.Run("WaitTimesOut", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "seeding the held lock should succeed")

		defer func() { _ = held.Release(ctx) }()

		_, err = locker.Acquire(ctx, name(t), WithWait(150*time.Millisecond), WithRetryInterval(20*time.Millisecond))
		require.ErrorIs(t, err, ErrNotAcquired, "an exhausted wait window must give up with ErrNotAcquired")
	})

	t.Run("WaitHonorsContextCancellation", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "seeding the held lock should succeed")

		defer func() { _ = held.Release(ctx) }()

		waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		_, err = locker.Acquire(waitCtx, name(t), WithWait(10*time.Second), WithRetryInterval(20*time.Millisecond))
		require.ErrorIs(t, err, context.DeadlineExceeded, "a canceled context must abort the wait")
	})

	t.Run("TTLExpiryFreesLock", func(t *testing.T) {
		stale, err := locker.TryAcquire(ctx, name(t), WithTTL(100*time.Millisecond))
		require.NoError(t, err, "the first acquisition should succeed")

		time.Sleep(250 * time.Millisecond)

		fresh, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "an expired lease must free the lock for the next holder")

		assert.ErrorIs(t, stale.Release(ctx), ErrNotHeld, "releasing an expired lease must report ErrNotHeld")
		require.NoError(t, fresh.Release(ctx), "cleanup release should succeed")
	})

	t.Run("RefreshExtendsLease", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t), WithTTL(time.Second))
		require.NoError(t, err, "the acquisition should succeed")

		// Keep refreshing past the original TTL: 3 × 400ms = 1.2s > 1s.
		for range 3 {
			time.Sleep(400 * time.Millisecond)
			require.NoError(t, held.Refresh(ctx), "refresh on a live lease should succeed")
		}

		_, err = locker.TryAcquire(ctx, name(t))
		require.ErrorIs(t, err, ErrNotAcquired, "a refreshed lease must still exclude other holders past the original TTL")

		require.NoError(t, held.Release(ctx), "release after refreshing should succeed")
	})

	t.Run("ReleaseTwiceReportsNotHeld", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "the acquisition should succeed")

		require.NoError(t, held.Release(ctx), "the first release should succeed")
		assert.ErrorIs(t, held.Release(ctx), ErrNotHeld, "a second release must report ErrNotHeld")
	})

	t.Run("RefreshAfterReleaseReportsNotHeld", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "the acquisition should succeed")

		require.NoError(t, held.Release(ctx), "release should succeed")
		assert.ErrorIs(t, held.Refresh(ctx), ErrNotHeld, "refreshing a released lease must report ErrNotHeld")
	})

	t.Run("ConcurrentAcquireAdmitsExactlyOne", func(t *testing.T) {
		var (
			wg      sync.WaitGroup
			winners atomic.Int32
			winner  atomic.Pointer[Lock]
		)

		for range 20 {
			wg.Go(func() {
				held, err := locker.TryAcquire(ctx, name(t))
				if err == nil {
					winners.Add(1)
					winner.Store(&held)
				}
			})
		}

		wg.Wait()

		require.Equal(t, int32(1), winners.Load(), "exactly one of the racing acquisitions must win")
		require.NoError(t, (*winner.Load()).Release(ctx), "the winner's release should succeed")
	})

	t.Run("FencingTokensStrictlyIncrease", func(t *testing.T) {
		var previous int64
		for range 3 {
			held, err := locker.TryAcquire(ctx, name(t))
			require.NoError(t, err, "each sequential acquisition should succeed")
			assert.Greater(t, held.FencingToken(), previous, "every new lease must draw a larger fencing token")

			previous = held.FencingToken()

			require.NoError(t, held.Release(ctx), "release should succeed")
		}
	})

	t.Run("FencingIncreasesAcrossExpiryTakeover", func(t *testing.T) {
		stale, err := locker.TryAcquire(ctx, name(t), WithTTL(80*time.Millisecond))
		require.NoError(t, err, "the first acquisition should succeed")

		time.Sleep(200 * time.Millisecond)

		fresh, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "the takeover acquisition should succeed")
		assert.Greater(t, fresh.FencingToken(), stale.FencingToken(), "a takeover after expiry must draw a larger fencing token than the stale lease")

		require.NoError(t, fresh.Release(ctx), "cleanup release should succeed")
	})

	t.Run("AutoRenewOutlivesTTL", func(t *testing.T) {
		held, err := locker.Acquire(ctx, name(t), WithTTL(500*time.Millisecond), WithAutoRenew(true))
		require.NoError(t, err, "the acquisition should succeed")

		time.Sleep(1200 * time.Millisecond)

		_, err = locker.TryAcquire(ctx, name(t))
		require.ErrorIs(t, err, ErrNotAcquired, "the watchdog must keep the lease alive past its TTL")

		select {
		case <-held.Done():
			t.Fatal("a healthy auto-renewed lease must not report loss")
		default:
		}

		require.NoError(t, held.Release(ctx), "release should stop the watchdog and succeed")
	})

	t.Run("DoneStaysOpenWithoutAutoRenew", func(t *testing.T) {
		held, err := locker.TryAcquire(ctx, name(t), WithTTL(80*time.Millisecond))
		require.NoError(t, err, "the acquisition should succeed")

		time.Sleep(200 * time.Millisecond)

		select {
		case <-held.Done():
			t.Fatal("without auto-renew the Done channel must never close, even after expiry")
		default:
		}
	})

	t.Run("DoneStaysOpenAfterCleanRelease", func(t *testing.T) {
		// A pending watchdog tick racing the release must not fire a spurious
		// loss signal; iterate to give a regression a real chance to surface.
		for range 10 {
			held, err := locker.Acquire(ctx, name(t), WithTTL(60*time.Millisecond), WithAutoRenew(true))
			require.NoError(t, err, "the acquisition should succeed")

			require.NoError(t, held.Release(ctx), "release should succeed")

			time.Sleep(70 * time.Millisecond)

			select {
			case <-held.Done():
				t.Fatal("a voluntarily released lease must not report loss")
			default:
			}
		}
	})

	t.Run("WithLockRunsExclusivelyAndReleases", func(t *testing.T) {
		err := WithLock(ctx, locker, name(t), func(ctx context.Context) error {
			_, inner := locker.TryAcquire(ctx, name(t))
			assert.ErrorIs(t, inner, ErrNotAcquired, "the lock must be held while fn runs")

			return nil
		})
		require.NoError(t, err, "WithLock should succeed when fn succeeds")

		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "WithLock must release the lock after fn returns")
		require.NoError(t, held.Release(ctx), "cleanup release should succeed")
	})

	t.Run("WithLockPropagatesFnError", func(t *testing.T) {
		sentinel := errors.New("business failure")

		err := WithLock(ctx, locker, name(t), func(context.Context) error { return sentinel })
		require.ErrorIs(t, err, sentinel, "fn's error must propagate out of WithLock")

		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "the lock must be released even when fn fails")
		require.NoError(t, held.Release(ctx), "cleanup release should succeed")
	})

	t.Run("WithLockReleasesOnPanic", func(t *testing.T) {
		func() {
			defer func() {
				require.NotNil(t, recover(), "the panic must propagate out of WithLock")
			}()

			_ = WithLock(ctx, locker, name(t), func(context.Context) error { panic("business panic") })
		}()

		held, err := locker.TryAcquire(ctx, name(t))
		require.NoError(t, err, "the lock must be released even when fn panics — the watchdog must not keep the abandoned lease alive")
		require.NoError(t, held.Release(ctx), "cleanup release should succeed")
	})

	t.Run("WithLockCancelsFnWhenLeaseLost", func(t *testing.T) {
		err := WithLock(ctx, locker, name(t), func(fnCtx context.Context) error {
			steal(t, name(t))

			select {
			case <-fnCtx.Done():
				return nil
			case <-time.After(3 * time.Second):
				return errors.New("fn context was not canceled after the lease was lost")
			}
		}, WithTTL(120*time.Millisecond))
		require.ErrorIs(t, err, ErrNotHeld, "WithLock must surface the lost lease through the release error")
	})
}
