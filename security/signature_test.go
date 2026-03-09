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
		result, err := sig.Sign("test-app")

		require.NoError(t, err, "Should sign without error")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Equal(t, "test-app", result.AppID, "AppID should match input")
		assert.NotZero(t, result.Timestamp, "Timestamp should not be zero")
		assert.NotEmpty(t, result.Nonce, "Nonce should not be empty")
		assert.NotEmpty(t, result.Signature, "Signature should not be empty")
		assert.Len(t, result.Signature, 64, "Signature length should be 64 for SHA256")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		result, err := sig.Sign("")

		assert.ErrorIs(t, err, ErrSignatureAppIDRequired, "Should return app ID required error")
		assert.Nil(t, result, "Result should be nil on error")
	})

	t.Run("UniqueNoncePerSign", func(t *testing.T) {
		result1, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign first request without error")

		result2, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign second request without error")

		assert.NotEqual(t, result1.Nonce, result2.Nonce, "Nonces should be unique")
		assert.NotEqual(t, result1.Signature, result2.Signature, "Signatures should be different")
	})

	t.Run("TimestampIsRecent", func(t *testing.T) {
		beforeSec := time.Now().Unix()
		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		afterSec := time.Now().Unix()

		assert.GreaterOrEqual(t, result.Timestamp, beforeSec, "Timestamp should be >= before time")
		assert.LessOrEqual(t, result.Timestamp, afterSec, "Timestamp should be <= after time")
	})

	t.Run("DifferentAppsProduceDifferentSignatures", func(t *testing.T) {
		result1, err := sig.Sign("test-app-1")
		require.NoError(t, err, "Should sign first app without error")

		result2, err := sig.Sign("test-app-2")
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

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify valid signature without error")
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, "0000000000000000000000000000000000000000000000000000000000000000")
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error")
	})

	t.Run("MalformedSignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, "not-valid-hex")
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for malformed hex")
	})

	t.Run("WrongAppID", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, "wrong-app", result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for wrong app ID")
	})

	t.Run("WrongNonce", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, "wrong-nonce", result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for wrong nonce")
	})

	t.Run("WrongTimestamp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp+1, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid signature error for wrong timestamp")
	})

	t.Run("ExpiredTimestamp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(1*time.Second))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		oldTimestampSec := time.Now().Add(-10 * time.Second).Unix()
		err = sig.Verify(ctx, result.AppID, oldTimestampSec, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureExpired, "Should return expired error for old timestamp")
	})

	t.Run("FutureTimestamp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(1*time.Second))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		futureTimestampSec := time.Now().Add(10 * time.Second).Unix()
		err = sig.Verify(ctx, result.AppID, futureTimestampSec, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureExpired, "Should return expired error for future timestamp")
	})

	t.Run("TimestampAtBoundary", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithTimestampTolerance(5*time.Second))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify signature at boundary without error")
	})

	t.Run("EmptyAppID", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		err = sig.Verify(ctx, "", time.Now().Unix(), "test-nonce", "signature")
		assert.ErrorIs(t, err, ErrSignatureAppIDRequired, "Should return app ID required error")
	})

	t.Run("EmptyNonce", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		err = sig.Verify(ctx, "test-app", time.Now().Unix(), "", "signature")
		assert.ErrorIs(t, err, ErrSignatureNonceRequired, "Should return nonce required error")
	})

	t.Run("EmptySignature", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		err = sig.Verify(ctx, "test-app", time.Now().Unix(), "test-nonce", "")
		assert.ErrorIs(t, err, ErrSignatureRequired, "Should return signature required error")
	})

	t.Run("ReplayAttackPrevention", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify first request without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureNonceUsed, "Should return nonce used error for replay attack")
	})

	t.Run("ConcurrentReplayAttackPrevention", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		start := make(chan struct{})
		errs := make([]error, 2)

		var wg sync.WaitGroup

		for i := range 2 {
			wg.Go(func() {
				<-start

				errs[i] = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
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

			assert.ErrorIs(t, verifyErr, ErrSignatureNonceUsed, "One concurrent request should be rejected as replay")
		}

		assert.Equal(t, 1, successCount, "Only one concurrent verify should succeed for same nonce")
	})

	t.Run("WithoutNonceStoreAllowsReplay", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify first request without error")

		err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should allow replay when nonce store is nil")
	})

	t.Run("DifferentNoncesSameApp", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result1, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign first request without error")

		result2, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign second request without error")

		err = sig.Verify(ctx, result1.AppID, result1.Timestamp, result1.Nonce, result1.Signature)
		assert.NoError(t, err, "Should verify first signature without error")

		err = sig.Verify(ctx, result2.AppID, result2.Timestamp, result2.Nonce, result2.Signature)
		assert.NoError(t, err, "Should verify second signature without error")
	})
}

// TestSignatureVerifyWithSecret tests Signature verify with secret scenarios.
func TestSignatureVerifyWithSecret(t *testing.T) {
	ctx := context.Background()
	differentSecret := "bf7786789ce92be8d04d5b62e233ff72fa861ff6e53dfbc2d44c3a4a47cd25d3"

	t.Run("ValidSignatureWithMatchingSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, testSignatureSecret, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.NoError(t, err, "Should verify with matching secret without error")
	})

	t.Run("InvalidSignatureWithDifferentSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, differentSecret, result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should return invalid error for different secret")
	})

	t.Run("InvalidHexSecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, "not-valid-hex", result.AppID, result.Timestamp, result.Nonce, result.Signature)
		assert.ErrorIs(t, err, ErrDecodeSignatureSecretFailed, "Should return decode failed error for invalid hex")
	})

	t.Run("EmptySecret", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret, WithNonceStore(nil))
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		err = sig.VerifyWithSecret(ctx, "", result.AppID, result.Timestamp, result.Nonce, result.Signature)
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

			result, err := sig.Sign("test-app")
			require.NoError(t, err, "Should sign without error")
			assert.Len(t, result.Signature, tt.sigLen, "Signature length should match algorithm")

			err = sig.Verify(ctx, result.AppID, result.Timestamp, result.Nonce, result.Signature)
			assert.NoError(t, err, "Should verify signature without error")
		})
	}

	t.Run("DifferentAlgorithmsProduceDifferentSignatures", func(t *testing.T) {
		sig256, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA256), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA256 signature without error")

		sig512, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA512), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA512 signature without error")

		result256, err := sig256.Sign("test-app")
		require.NoError(t, err, "Should sign with SHA256 without error")

		result512, err := sig512.Sign("test-app")
		require.NoError(t, err, "Should sign with SHA512 without error")

		assert.NotEqual(t, result256.Signature, result512.Signature, "Different algorithms should produce different signatures")
	})

	t.Run("CrossAlgorithmVerificationFails", func(t *testing.T) {
		sig256, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA256), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA256 signature without error")

		sig512, err := NewSignature(testSignatureSecret, WithAlgorithm(SignatureAlgHmacSHA512), WithNonceStore(nil))
		require.NoError(t, err, "Should create SHA512 signature without error")

		result256, err := sig256.Sign("test-app")
		require.NoError(t, err, "Should sign with SHA256 without error")

		err = sig512.Verify(ctx, result256.AppID, result256.Timestamp, result256.Nonce, result256.Signature)
		assert.ErrorIs(t, err, ErrSignatureInvalid, "Should fail cross-algorithm verification")
	})
}

// TestSignatureResult tests signature result functionality.
func TestSignatureResult(t *testing.T) {
	t.Run("ContainsAllFields", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
		require.NoError(t, err, "Should sign without error")

		assert.Equal(t, "test-app", result.AppID, "AppID should match input")
		assert.NotZero(t, result.Timestamp, "Timestamp should not be zero")
		assert.NotEmpty(t, result.Nonce, "Nonce should not be empty")
		assert.NotEmpty(t, result.Signature, "Signature should not be empty")
	})

	t.Run("NonceLength", func(t *testing.T) {
		sig, err := NewSignature(testSignatureSecret)
		require.NoError(t, err, "Should create signature without error")

		result, err := sig.Sign("test-app")
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
