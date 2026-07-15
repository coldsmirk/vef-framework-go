package jscrypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jscrypto"
)

// newCryptoRuntime builds a bare runtime with the crypto library enabled.
func newCryptoRuntime(t *testing.T) *js.Runtime {
	t.Helper()

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(jscrypto.New()))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs(jscrypto.Name))
	require.NoError(t, err, "NewRuntime should succeed")

	return rt
}

// TestDigests tests the hash and HMAC functions against known vectors.
func TestDigests(t *testing.T) {
	rt := newCryptoRuntime(t)

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "Md5",
			script: `crypto.md5('abc')`,
			want:   "900150983cd24fb0d6963f7d28e17f72",
		},
		{
			name:   "Sha1",
			script: `crypto.sha1('abc')`,
			want:   "a9993e364706816aba3e25717850c26c9cd0d89d",
		},
		{
			name:   "Sha256",
			script: `crypto.sha256('abc')`,
			want:   "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name:   "Sha512",
			script: `crypto.sha512('abc')`,
			want:   "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		},
		{
			name:   "Sm3",
			script: `crypto.sm3('abc')`,
			want:   "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0",
		},
		{
			name:   "HmacSha256",
			script: `crypto.hmac('sha256', 'key', 'The quick brown fox jumps over the lazy dog')`,
			want:   "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8",
		},
		{
			name:   "HmacMd5",
			script: `crypto.hmac('md5', 'key', 'The quick brown fox jumps over the lazy dog')`,
			want:   "80070713463e7749b90c2dc24911e275",
		},
		{
			name:   "HmacAlgorithmCaseInsensitive",
			script: `crypto.hmac('SHA256', 'key', 'The quick brown fox jumps over the lazy dog')`,
			want:   "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rt.RunString(t.Context(), tt.script)
			require.NoError(t, err, "Script should execute successfully")
			assert.Equal(t, tt.want, result.String(), "Digest should match the known vector")
		})
	}

	t.Run("HmacUnsupportedAlgorithm", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `
			try {
				crypto.hmac('rot13', 'key', 'data');
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Contains(t, result.String(), "unsupported algorithm", "Unknown algorithm should throw a catchable error")
	})
}

// TestEncodings tests base64 and hex helpers.
func TestEncodings(t *testing.T) {
	rt := newCryptoRuntime(t)

	t.Run("Base64RoundTrip", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `
			const encoded = crypto.base64Encode('hello');
			encoded + ':' + crypto.base64Decode(encoded)
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "aGVsbG8=:hello", result.String(), "Base64 should round-trip")
	})

	t.Run("Base64DecodeError", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `crypto.base64Decode('!!!')`)
		require.Error(t, err, "Invalid base64 input should throw")
	})

	t.Run("HexRoundTrip", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `
			const hexed = crypto.hexEncode('hi');
			hexed + ':' + crypto.hexDecode(hexed)
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "6869:hi", result.String(), "Hex should round-trip")
	})

	t.Run("HexDecodeError", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `crypto.hexDecode('zz')`)
		require.Error(t, err, "Invalid hex input should throw")
	})
}

// TestUUID tests uuid generation.
func TestUUID(t *testing.T) {
	rt := newCryptoRuntime(t)

	result, err := rt.RunString(t.Context(), `
		const a = crypto.uuid();
		const b = crypto.uuid();
		({ a, b, distinct: a !== b })
	`)
	require.NoError(t, err, "Script should execute successfully")

	obj := result.ToObject(rt.VM())
	assert.Regexp(t,
		`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
		obj.Get("a").String(), "UUID should be a valid v4")
	assert.True(t, obj.Get("distinct").ToBoolean(), "Consecutive UUIDs should differ")
}
