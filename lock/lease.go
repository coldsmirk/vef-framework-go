package lock

import (
	"context"
	"errors"
	"sync"
	"time"
)

// minRenewInterval floors the watchdog cadence so a tiny TTL cannot spin the
// renewal loop.
const minRenewInterval = 10 * time.Millisecond

// leaseBackend is the store-specific pair of token-guarded primitives a lease
// runs on; both must return ErrNotHeld when the token no longer owns the lock.
type leaseBackend struct {
	release func(ctx context.Context, name, token string) error
	refresh func(ctx context.Context, name, token string, ttl time.Duration) error
}

// lease is the shared Lock implementation used by every Locker: it holds the
// ownership token and delegates release/refresh to the backend, optionally
// running the auto-renewal watchdog.
type lease struct {
	name    string
	token   string
	ttl     time.Duration
	fencing int64
	backend leaseBackend

	done      chan struct{}
	loseOnce  sync.Once
	stopRenew func()
}

// newLease builds a held lease and starts the watchdog when autoRenew is set.
func newLease(name, token string, cfg acquireConfig, fencing int64, backend leaseBackend) *lease {
	l := &lease{
		name:    name,
		token:   token,
		ttl:     cfg.ttl,
		fencing: fencing,
		backend: backend,
		done:    make(chan struct{}),
	}

	if cfg.autoRenew {
		l.startRenew()
	}

	return l
}

func (l *lease) Release(ctx context.Context) error {
	if l.stopRenew != nil {
		l.stopRenew()
	}

	return l.backend.release(ctx, l.name, l.token)
}

func (l *lease) Refresh(ctx context.Context) error {
	return l.backend.refresh(ctx, l.name, l.token, l.ttl)
}

func (l *lease) FencingToken() int64 { return l.fencing }

func (l *lease) Done() <-chan struct{} { return l.done }

// markLost closes the done channel exactly once.
func (l *lease) markLost() {
	l.loseOnce.Do(func() { close(l.done) })
}

// startRenew launches the watchdog goroutine: it refreshes the lease at a
// third of the TTL and declares it lost on a definitive ErrNotHeld, or when
// transient backend errors have kept it from refreshing for a full TTL (at
// which point the lease has expired server-side). Each refresh is bounded by
// the renewal interval so a hung backend call cannot delay loss detection by
// more than one extra tick.
func (l *lease) startRenew() {
	stop := make(chan struct{})
	l.stopRenew = sync.OnceFunc(func() { close(stop) })

	// markLostUnlessStopped suppresses the loss signal after Release: Release
	// closes stop before deleting the key, so a refresh that observed
	// ErrNotHeld because of our own clean release always finds stop closed
	// here — a voluntarily released lease was never "lost".
	markLostUnlessStopped := func() {
		select {
		case <-stop:
		default:
			l.markLost()
		}
	}

	go func() {
		interval := max(l.ttl/3, minRenewInterval)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		lastRefreshed := time.Now()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), interval)
				err := l.backend.refresh(ctx, l.name, l.token, l.ttl)

				cancel()

				switch {
				case err == nil:
					lastRefreshed = time.Now()
				case errors.Is(err, ErrNotHeld):
					markLostUnlessStopped()

					return
				default:
					logger.Warnf("Failed to renew lock %q, retrying: %v", l.name, err)

					if time.Since(lastRefreshed) >= l.ttl {
						markLostUnlessStopped()

						return
					}
				}
			}
		}
	}()
}
