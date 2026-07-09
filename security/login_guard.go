package security

import (
	"context"
	"time"
)

// LockoutStrategy selects how a LoginGuard penalizes repeated failures. It
// mirrors config.LockoutStrategy as the resolved domain value the guard acts on.
type LockoutStrategy string

const (
	// LockoutStrategyLock blocks all attempts for a fixed duration once the
	// failure threshold is reached.
	LockoutStrategyLock LockoutStrategy = "lock"
	// LockoutStrategyBackoff imposes an exponentially growing delay past the
	// threshold instead of a hard lock.
	LockoutStrategyBackoff LockoutStrategy = "backoff"
)

// LockoutKey selects the identity dimension a LoginGuard counts failures by.
type LockoutKey string

const (
	// LockoutKeyUser counts failures per login identifier.
	LockoutKeyUser LockoutKey = "user"
	// LockoutKeyIP counts failures per source address.
	LockoutKeyIP LockoutKey = "ip"
	// LockoutKeyUserIP counts failures per identifier-and-source pair.
	LockoutKeyUserIP LockoutKey = "user_ip"
)

// LockoutPolicy is the fully-resolved brute-force policy a LoginGuard enforces.
// The framework builds it from config; an application constructing a guard
// directly must populate every field, as a zero policy imposes no lockout.
type LockoutPolicy struct {
	// MaxFailures is the number of failures tolerated before a penalty engages.
	MaxFailures int
	// Window is how long failures are remembered before the counter resets.
	Window time.Duration
	// LockDuration is the block length under the lock strategy.
	LockDuration time.Duration
	// Strategy selects lock vs. backoff.
	Strategy LockoutStrategy
	// BackoffBase is the first backoff delay past the threshold; it doubles per
	// further failure.
	BackoffBase time.Duration
	// BackoffMax caps the per-attempt backoff delay.
	BackoffMax time.Duration
	// Key selects the dimension failures are counted by.
	Key LockoutKey
}

// storageKey composes the per-identity counter key for the attempt under the
// configured keying dimension.
func (p LockoutPolicy) storageKey(attempt LoginAttempt) string {
	switch p.Key {
	case LockoutKeyIP:
		return "ip:" + attempt.ClientIP
	case LockoutKeyUser:
		return "user:" + attempt.Identity
	default:
		return "user:" + attempt.Identity + "|ip:" + attempt.ClientIP
	}
}

// cooldownFor returns how long attempts must be blocked after count cumulative
// failures, or zero when the count is still within the tolerated allowance.
func (p LockoutPolicy) cooldownFor(count int) time.Duration {
	excess := count - p.MaxFailures
	if excess < 0 {
		return 0
	}

	if p.Strategy == LockoutStrategyBackoff {
		return p.backoffDelay(excess)
	}

	return p.LockDuration
}

// backoffDelay returns BackoffBase * 2^excess, capped at BackoffMax. The cap is
// checked each doubling so the delay never overflows for a large excess.
func (p LockoutPolicy) backoffDelay(excess int) time.Duration {
	delay := p.BackoffBase
	for range excess {
		delay *= 2
		if delay >= p.BackoffMax {
			return p.BackoffMax
		}
	}

	return delay
}

// LoginAttempt identifies a single login attempt for brute-force accounting.
type LoginAttempt struct {
	// Identity is the login identifier supplied by the client (the username).
	Identity string
	// ClientIP is the resolved source address of the request.
	ClientIP string
}

// LoginDecision is the verdict a LoginGuard returns for an attempt.
type LoginDecision struct {
	// Allowed reports whether the attempt may proceed.
	Allowed bool
	// RetryAfter is how long the caller should wait before retrying when the
	// attempt is blocked; it is zero when Allowed is true.
	RetryAfter time.Duration
}

// LoginGuard tracks failed login attempts and decides whether an identity may
// currently attempt authentication, providing brute-force protection through a
// fixed-window lock or an exponential backoff.
type LoginGuard interface {
	// Check reports whether an attempt for the identity may proceed now, without
	// recording anything.
	Check(ctx context.Context, attempt LoginAttempt) (LoginDecision, error)
	// RecordFailure registers a failed attempt and returns the resulting
	// decision (whether the identity is now blocked, and for how long).
	RecordFailure(ctx context.Context, attempt LoginAttempt) (LoginDecision, error)
	// RecordSuccess clears any accumulated failure state for the identity.
	RecordSuccess(ctx context.Context, attempt LoginAttempt) error
}
