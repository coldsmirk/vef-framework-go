package lock

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var logger = logx.Named("lock")

// releaseTimeout bounds the best-effort release performed by WithLock after
// the callback returns, so a canceled request context never leaks a held lock
// until its TTL.
const releaseTimeout = 5 * time.Second

// lockTokenBytes is the entropy of a lock ownership token before encoding.
const lockTokenBytes = 16

// Locker acquires named distributed locks. Locks are lease-based: every
// acquisition carries a TTL that auto-expires if the holder crashes, and only
// the holder (identified by a random ownership token) can release or extend
// its lease. Mutual exclusion is therefore cooperative — a process pause that
// outlives the TTL can let a second holder in, so guard state that must never
// be corrupted with Lock.FencingToken, idempotency, or database constraints.
type Locker interface {
	// Acquire obtains the named lock, retrying until the WithWait window is
	// exhausted (no waiting by default). It returns ErrNotAcquired when the
	// lock stays held by someone else, and fails closed on backend errors.
	Acquire(ctx context.Context, name string, opts ...Option) (Lock, error)
	// TryAcquire attempts a single non-blocking acquisition, returning
	// ErrNotAcquired immediately when the lock is held — the natural guard for
	// "only one replica runs this job" cron patterns.
	TryAcquire(ctx context.Context, name string, opts ...Option) (Lock, error)
}

// Lock is a held lease returned by a successful acquisition.
type Lock interface {
	// Release relinquishes the lease. It returns ErrNotHeld when the lease has
	// already expired or been released — a signal that mutual exclusion may
	// have been violated in the meantime.
	Release(ctx context.Context) error
	// Refresh extends the lease by the acquisition TTL from now. It returns
	// ErrNotHeld once the lease is no longer owned. With auto-renewal enabled
	// the lease refreshes itself and calling Refresh is unnecessary.
	Refresh(ctx context.Context) error
	// FencingToken returns the lease's monotonically increasing sequence
	// number (unique and ordered per lock name). Pass it to the protected
	// resource so a delayed writer holding a stale lease can be rejected.
	FencingToken() int64
	// Done returns a channel closed once the lease is known to be lost.
	// Loss is detected by the auto-renewal watchdog, so without WithAutoRenew
	// the channel never closes.
	Done() <-chan struct{}
}

// WithLock runs fn while holding the named lock: it acquires (auto-renewal on
// by default, so fn may safely outlive the TTL), cancels fn's context if the
// lease is lost, and always releases afterwards — even when fn panics — on a
// context that survives request cancellation. It returns the joined error of
// fn and the release — a successful fn still yields an error when the lease
// was lost mid-run, because the exclusive section can no longer be trusted.
func WithLock(ctx context.Context, locker Locker, name string, fn func(ctx context.Context) error, opts ...Option) (err error) {
	held, err := locker.Acquire(ctx, name, append([]Option{WithAutoRenew(true)}, opts...)...)
	if err != nil {
		return err
	}

	// Release in a defer so a panicking fn still frees the lock; without this
	// the auto-renewal watchdog would keep the abandoned lease alive forever.
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
		defer releaseCancel()

		err = errors.Join(err, held.Release(releaseCtx))
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-held.Done():
			cancel()
		case <-runCtx.Done():
		}
	}()

	return fn(runCtx)
}

// acquireLoop drives a waiting acquisition: it retries tryOnce every
// retryInterval until it succeeds, a non-ErrNotAcquired error occurs, the wait
// window is exhausted, or ctx is canceled.
func acquireLoop(ctx context.Context, cfg acquireConfig, tryOnce func(ctx context.Context) (Lock, error)) (Lock, error) {
	deadline := time.Now().Add(cfg.wait)

	for {
		held, err := tryOnce(ctx)
		if !errors.Is(err, ErrNotAcquired) {
			return held, err
		}

		if time.Now().Add(cfg.retryInterval).After(deadline) {
			return nil, ErrNotAcquired
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(cfg.retryInterval):
		}
	}
}

// newLockToken returns a new random ownership token for a lease.
func newLockToken() (string, error) {
	buf := make([]byte, lockTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
