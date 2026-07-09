package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLockoutPolicyStorageKey(t *testing.T) {
	attempt := LoginAttempt{Identity: "alice", ClientIP: "10.0.0.1"}

	tests := []struct {
		name string
		key  LockoutKey
		want string
	}{
		{"ByUser", LockoutKeyUser, "user:alice"},
		{"ByIP", LockoutKeyIP, "ip:10.0.0.1"},
		{"ByUserIP", LockoutKeyUserIP, "user:alice|ip:10.0.0.1"},
		{"UnknownFallsBackToUserIP", LockoutKey("bogus"), "user:alice|ip:10.0.0.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := LockoutPolicy{Key: tc.key}
			assert.Equal(t, tc.want, policy.storageKey(attempt), "storage key should reflect the keying dimension")
		})
	}
}

func TestLockoutPolicyCooldownForLock(t *testing.T) {
	policy := LockoutPolicy{
		MaxFailures:  3,
		LockDuration: 15 * time.Minute,
		Strategy:     LockoutStrategyLock,
	}

	tests := []struct {
		name  string
		count int
		want  time.Duration
	}{
		{"BelowThreshold", 1, 0},
		{"OneBelowThreshold", 2, 0},
		{"AtThresholdLocks", 3, 15 * time.Minute},
		{"AboveThresholdStaysLocked", 5, 15 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, policy.cooldownFor(tc.count), "lock cooldown should engage exactly at the threshold")
		})
	}
}

func TestLockoutPolicyCooldownForBackoff(t *testing.T) {
	policy := LockoutPolicy{
		MaxFailures: 2,
		Strategy:    LockoutStrategyBackoff,
		BackoffBase: 1 * time.Second,
		BackoffMax:  8 * time.Second,
	}

	tests := []struct {
		name  string
		count int
		want  time.Duration
	}{
		{"BelowThresholdNoDelay", 1, 0},
		{"AtThresholdBaseDelay", 2, 1 * time.Second},
		{"DoublesPerFailure", 3, 2 * time.Second},
		{"DoublesAgain", 4, 4 * time.Second},
		{"ReachesCap", 5, 8 * time.Second},
		{"StaysCappedForLargeExcess", 50, 8 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, policy.cooldownFor(tc.count), "backoff delay should double per failure and cap at BackoffMax")
		})
	}
}

// TestLockoutPolicyZeroValueImposesNoLockout documents that a hand-constructed
// zero policy is inert rather than locking on the first failure.
func TestLockoutPolicyZeroValueImposesNoLockout(t *testing.T) {
	var policy LockoutPolicy

	assert.Equal(t, time.Duration(0), policy.cooldownFor(1), "a zero policy should never block")
	assert.Equal(t, time.Duration(0), policy.cooldownFor(100), "a zero policy should never block even after many failures")
}
