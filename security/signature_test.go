package security

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSignatureSecret = DefaultJWTSecret

const (
	testSigMethod = "POST"
	testSigPath   = "/api"
)

// TestNewSignature tests new signature functionality.
func TestNewSignature(t *testing.T) {
	t.Run("ValidSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)

		require.NoError(t, err, "Should create signature without error")
		assert.NotNil(t, sig, "Signature should not be nil")
		assert.Equal(t, SignatureAlgHmacSHA256, sig.algorithm, "Default algorithm should be HMAC-SHA256")
		assert.Equal(t, 5*time.Minute, sig.timestampTolerance, "Default timestamp tolerance should be 5 minutes")
		assert.NotNil(t, sig.nonceStore, "Default nonce store should not be nil")
	})

	t.Run("WithAlgorithmOption", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA512))

		require.NoError(t, err, "Should create signature without error")
		assert.Equal(t, SignatureAlgHmacSHA512, sig.algorithm, "Algorithm should be HMAC-SHA512")
	})

	t.Run("WithTimestampToleranceOption", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(10*time.Minute))

		require.NoError(t, err, "Should create signature without error")
		assert.Equal(t, 10*time.Minute, sig.timestampTolerance, "Timestamp tolerance should be 10 minutes")
	})

	t.Run("WithNonceStoreOption", func(t *testing.T) {
		customStore := NewMemoryNonceStore()
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(customStore))

		require.NoError(t, err, "Should create signature without error")
		assert.Equal(t, customStore, sig.nonceStore, "Nonce store should be custom store")
	})

	t.Run("WithNilNonceStore", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))

		require.NoError(t, err, "Should create signature without error")
		assert.Nil(t, sig.nonceStore, "Nonce store should be nil")
	})

	t.Run("WithMultipleOptions", func(t *testing.T) {
		sig, err := NewSignature(
			testSignatureSecret,
			WithAlgorithm(SignatureAlgHmacSM3),
			WithTimestampTolerance(15*time.Minute),
		)

		require.NoError(t, err, "Should create signature without error")
		assert.Equal(t, SignatureAlgHmacSM3, sig.algorithm, "Algorithm should be HMAC-SM3")
		assert.Equal(t, 15*time.Minute, sig.timestampTolerance, "Timestamp tolerance should be 15 minutes")
	})

	t.Run("EmptySecret", func(t *testing.T) {
		_, err := NewSignature("")

		assert.ErrorIs(t, err, ErrSignatureSecretRequired, "Should return secret required error")
	})

	t.Run("InvalidHexSecret", func(t *testing.T) {
		_, err := NewSignature("not-valid-hex")

		assert.ErrorIs(t, err, ErrDecodeSignatureSecretFailed, "Should return decode failed error")
	})

	t.Run("ShortSecret", func(t *testing.T) {
		sig, err := NewSignature("abcd")

		require.NoError(t, err, "Should create signature with short secret")
		assert.NotNil(t, sig, "Signature should not be nil")
	})
}

// TestSignatureSign tests Signature sign scenarios.
func TestSignatureSign(t *testing.T) {
	sig, err := NewSignature(testSignatureSecret)
	require.NoError(t, err, "Should create signature without error")

	t.Run("BasicSign", func(t *testing.T) {
		result, err := sig.Sign("test-app", testSigMethod, testSigPath)

		require.NoError(t, err, "Should sign without error")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Equal(t, "test-app", result.AppID, "AppID should match input")
		assert.NotZero(t, result.Timestamp, "Timestamp should not be zero")
		assert.NotEmpty(t, result.Nonce, "Nonce should not be empty")
		assert.NotEmpty(t, result.Signature, "Signature should not be empty")
		assert.Len(t, result.Signature, 64, "Signature length should be 64 for SHA256")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		result, err := sig.Sign("", testSigMethod, testSigPath)

		assert.ErrorIs(t, err, ErrAppIDRequired, "Should return app ID required error")
		assert.Nil(t, result, "Result should be nil on error")
	})

	t.Run("UniqueNoncePerSign", func(t *testing.T) {
		result1, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign first request without error")

		result2, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign second request without error")

		assert.NotEqual(t, result1.Nonce, result2.Nonce, "Nonces should be unique")
		assert.NotEqual(t, result1.Signature, result2.Signature, "Signatures should be different")
	})

	t.Run("TimestampIsRecent", func(t *testing.T) {
		beforeSec := time.Now().Unix()
		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		afterSec := time.Now().Unix()

		assert.GreaterOrEqual(t, result.Timestamp, beforeSec, "Timestamp should be >= before time")
		assert.LessOrEqual(t, result.Timestamp, afterSec, "Timestamp should be <= after time")
	})

	t.Run("DifferentAppsProduceDifferentSignatures", func(t *testing.T) {
		result1, err := sig.Sign("test-app-1", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign first app without error")

		result2, err := sig.Sign("test-app-2", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign second app without error")

		assert.NotEqual(t, result1.Signature, result2.Signature, "Different apps should produce different signatures")
	})
}

// TestSignatureVerify tests Signature verify scenarios.
func TestSignatureVerify(t *testing.T) {
	ctx := context.Background()

	t.Run("ValidSignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify valid signature without error")
	})

	t.Run("WrongMethodRejected", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, "GET", testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "A signature bound to POST must not verify under a different method")
	})

	t.Run("WrongPathRejected", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, "/api/other", result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "A signature bound to /api must not verify under a different path")
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, "0000000000000000000000000000000000000000000000000000000000000000")
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error")
	})

	t.Run("MalformedSignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, "not-valid-hex")
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for malformed hex")
	})

	t.Run("WrongAppID", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, "wrong-app", testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for wrong app ID")
	})

	t.Run("WrongNonce", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, "wrong-nonce", result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for wrong nonce")
	})

	t.Run("WrongTimestamp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp+1, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for wrong timestamp")
	})

	t.Run("ExpiredTimestamp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(1*time.Second))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		oldTimestampSec := time.Now().Add(-10 * time.Second).Unix()
		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, oldTimestampSec, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureExpired, "Should return expired error for old timestamp")
	})

	t.Run("FutureTimestamp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(1*time.Second))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		futureTimestampSec := time.Now().Add(10 * time.Second).Unix()
		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, futureTimestampSec, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureExpired, "Should return expired error for future timestamp")
	})

	t.Run("TimestampAtBoundary", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(5*time.Second))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify signature at boundary without error")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		err = sig.Verify(ctx, "", testSigMethod, testSigPath, time.Now().Unix(), "test-nonce", "signature")
		assert.ErrorIs(t, err, ErrAppIDRequired, "Should return app ID required error")
	})

	t.Run("EmptyNonce", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		err = sig.Verify(ctx, "test-app", testSigMethod, testSigPath, time.Now().Unix(), "", "signature")
		assert.ErrorIs(t, err, ErrNonceRequired, "Should return nonce required error")
	})

	t.Run("EmptySignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		err = sig.Verify(ctx, "test-app", testSigMethod, testSigPath, time.Now().Unix(), "test-nonce", "")
		assert.ErrorIs(t, err, ErrSignatureRequired, "Should return signature required error")
	})

	t.Run("ReplayAttackPrevention", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify first request without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrNonceAlreadyUsed, "Should return nonce used error for replay attack")
	})

	t.Run("ConcurrentReplayAttackPrevention", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		start := make(chan struct{})
		errs := make([]error, 2)

		var wg sync.WaitGroup

		for i := range 2 {
			wg.Go(func() {
				<-start

				errs[i] = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
			})
		}

		close(start)
		wg.Wait()

		successCount := 0
		for _, verifyErr := range errs {
			if verifyErr == nil {
				successCount++

				continue
			}

			assert.ErrorIs(t, verifyErr, ErrNonceAlreadyUsed, "One concurrent request should be rejected as replay")
		}

		assert.Equal(t, 1, successCount, "Only one concurrent verify should succeed for same nonce")
	})

	t.Run("WithoutNonceStoreAllowsReplay", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify first request without error")

		err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should allow replay when nonce store is nil")
	})

	t.Run("DifferentNoncesSameApp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result1, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign first request without error")

		result2, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign second request without error")

		err = sig.Verify(ctx, result1.AppID, testSigMethod, testSigPath, result1.Timestamp, result1.Nonce, result1.Signature)
		assert.NoError(t, err, "Should verify first signature without error")

		err = sig.Verify(ctx, result2.AppID, testSigMethod, testSigPath, result2.Timestamp, result2.Nonce, result2.Signature)
		assert.NoError(t, err, "Should verify second signature without error")
	})
}

// recordingTTLNonceStore captures the TTL passed to StoreIfAbsent so tests can
// assert the nonce is retained long enough to cover the full replay window.
type recordingTTLNonceStore struct {
	ttl   time.Duration
	calls int
}

func (r *recordingTTLNonceStore) StoreIfAbsent(_ context.Context, _, _ string, ttl time.Duration) (bool, error) {
	r.ttl = ttl
	r.calls++

	return true, nil
}

// TestSignatureNonceTTLCoversReplayWindow pins the fix for the replay window
// where a future-dated timestamp outlived its nonce. validateTimestamp accepts
// ±timestampTolerance, so a single request stays valid for 2*timestampTolerance;
// the nonce must be retained at least that long or it could be replayed after
// expiry while its timestamp is still fresh.
func TestSignatureNonceTTLCoversReplayWindow(t *testing.T) {
	t.Run("TTLSpansFullSymmetricWindow", func(t *testing.T) {
		store := &recordingTTLNonceStore{}
		tolerance := 5 * time.Minute
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(tolerance), WithNonceStore(store))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(context.Background(), result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		require.NoError(t, err, "Should verify without error")

		require.Equal(t, 1, store.calls, "Verify should store the nonce exactly once")
		assert.GreaterOrEqual(t, store.ttl, 2*tolerance, "Nonce TTL must cover the full 2*tolerance replay window")
		assert.Equal(t, 2*tolerance+nonceTTLBuffer, store.ttl, "Nonce TTL should be 2*tolerance plus the jitter buffer")
	})

	t.Run("DefaultTolerance", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		assert.Equal(t, 2*defaultSignatureTimestampTolerance+nonceTTLBuffer, sig.nonceTTL(),
			"Default nonce TTL should span the full default window plus buffer")
	})
}

// clockNonceStore is a NonceStore whose entries expire against a test-controlled
// logical clock, mirroring MemoryNonceStore's StoreIfAbsent semantics (a still-
// live nonce reports present; an absent or expired one is (re)stored). Sharing
// the clock with the Signature lets a test advance the timestamp window and the
// nonce TTL in lockstep with no real-time waits.
type clockNonceStore struct {
	now     func() time.Time
	entries map[string]time.Time // key -> expiry instant
}

func (s *clockNonceStore) StoreIfAbsent(_ context.Context, appID, nonce string, ttl time.Duration) (bool, error) {
	key := appID + "\x00" + nonce
	now := s.now()

	if expiry, ok := s.entries[key]; ok && now.Before(expiry) {
		return false, nil
	}

	s.entries[key] = now.Add(ttl)

	return true, nil
}

// TestSignatureNonceOutlivesReplayWindowEndToEnd drives the full verify path on
// a shared logical clock to prove the nonce stays live for as long as a request
// is replayable. A maximally future-dated timestamp (ts = now + tolerance) is
// valid across the whole 2*tolerance span; advancing to the last fresh instant
// must still reject the replay. Under the pre-fix TTL (tolerance + buffer) the
// nonce would have expired by then and this very request would replay.
func TestSignatureNonceOutlivesReplayWindowEndToEnd(t *testing.T) {
	ctx := context.Background()
	tolerance := 5 * time.Minute

	current := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return current }

	store := &clockNonceStore{now: clock, entries: map[string]time.Time{}}

	sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(tolerance), WithNonceStore(store))
	require.NoError(t, err, "Should create signature without error")

	sig.clock = clock

	// Worst case for replay: the timestamp sits a full tolerance in the future,
	// so it stays valid across [ts-tolerance, ts+tolerance].
	ts := current.Add(tolerance).Unix()
	nonce := "replay-nonce"
	signature := sig.computeHMAC(sig.buildPayload("app", testSigMethod, testSigPath, ts, nonce))

	err = sig.Verify(ctx, "app", testSigMethod, testSigPath, ts, nonce, signature)
	require.NoError(t, err, "First request must verify and register the nonce")

	// Jump to the last instant the timestamp is still fresh (ts + tolerance, less
	// one second so validateTimestamp still accepts it).
	current = time.Unix(ts, 0).Add(tolerance - time.Second)
	require.NoError(t, sig.validateTimestamp(ts), "Timestamp must still be valid at the edge of its window")

	err = sig.Verify(ctx, "app", testSigMethod, testSigPath, ts, nonce, signature)
	require.ErrorIs(t, err, ErrNonceAlreadyUsed,
		"Replay at the edge of the timestamp window must be rejected — the nonce must outlive the request's validity")
}

// TestSignatureVerifyWithSecret tests Signature verify with secret scenarios.
func TestSignatureVerifyWithSecret(t *testing.T) {
	ctx := context.Background()
	differentSecret := "bf7786789ce92be8d04d5b62e233ff72fa861ff6e53dfbc2d44c3a4a47cd25d3"

	t.Run("ValidSignatureWithMatchingSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, testSignatureSecret, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify with matching secret without error")
	})

	t.Run("InvalidSignatureWithDifferentSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, differentSecret, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid error for different secret")
	})

	t.Run("InvalidHexSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, "not-valid-hex", result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrDecodeSignatureSecretFailed, "Should return decode failed error for invalid hex")
	})

	t.Run("EmptySecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, "", result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid error for empty secret")
	})
}

// TestSignatureAlgorithms tests Signature algorithms scenarios.
func TestSignatureAlgorithms(t *testing.T) {
	ctx := context.Background()

	algorithms := []struct {
		name      string
		algorithm SignatureAlgorithm
		sigLen    int
	}{
		{"HmacSHA256", SignatureAlgHmacSHA256, 64},
		{"HmacSHA512", SignatureAlgHmacSHA512, 128},
		{"HmacSM3", SignatureAlgHmacSM3, 64},
	}

	for _, tt := range algorithms {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := NewSignature(testSignatureSecret, WithAlgorithm(tt.algorithm))
			require.NoError(t, err, "Should create signature without error")

			result, err := sig.Sign("test-app", testSigMethod, testSigPath)
			require.NoError(t, err, "Should sign without error")
			assert.Len(t, result.Signature, tt.sigLen, "Signature length should match algorithm")

			err = sig.Verify(ctx, result.AppID, testSigMethod, testSigPath, result.Timestamp, result.Nonce, result.Signature)
			assert.NoError(t, err, "Should verify signature without error")
		})
	}

	t.Run("DifferentAlgorithmsProduceDifferentSignatures", func(t *testing.T) {
		sig256, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA256), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA256 signature without error")

		sig512, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA512), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA512 signature without error")

		result256, err := sig256.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign with SHA256 without error")

		result512, err := sig512.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign with SHA512 without error")

		assert.NotEqual(t, result256.Signature, result512.Signature, "Different algorithms should produce different signatures")
	})

	t.Run("CrossAlgorithmVerificationFails", func(t *testing.T) {
		sig256, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA256), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA256 signature without error")

		sig512, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA512), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA512 signature without error")

		result256, err := sig256.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign with SHA256 without error")

		err = sig512.Verify(ctx, result256.AppID, testSigMethod, testSigPath, result256.Timestamp, result256.Nonce, result256.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should fail cross-algorithm verification")
	})
}

// TestSignatureResult tests signature result functionality.
func TestSignatureResult(t *testing.T) {
	t.Run("ContainsAllFields", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		assert.Equal(t, "test-app", result.AppID, "AppID should match input")
		assert.NotZero(t, result.Timestamp, "Timestamp should not be zero")
		assert.NotEmpty(t, result.Nonce, "Nonce should not be empty")
		assert.NotEmpty(t, result.Signature, "Signature should not be empty")
	})

	t.Run("NonceLength", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app", testSigMethod, testSigPath)
		require.NoError(t, err, "Should sign without error")

		assert.GreaterOrEqual(t, len(result.Nonce), 16, "Nonce should be at least 16 characters")
	})
}

// TestSignatureCredentials tests signature credentials functionality.
func TestSignatureCredentials(t *testing.T) {
	t.Run("StructFields", func(t *testing.T) {
		creds := SignatureCredentials{
			Timestamp: time.Now().Unix(),
			Nonce:     "test-nonce",
			Signature: "test-signature",
		}

		assert.NotZero(t, creds.Timestamp, "Timestamp should not be zero")
		assert.Equal(t, "test-nonce", creds.Nonce, "Nonce should match input")
		assert.Equal(t, "test-signature", creds.Signature, "Signature should match input")
	})
}
