package definition

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
)

// stubScheme is the codec's view of a scheme, declaring one sensitive param.
type stubScheme struct {
	sensitive []string
}

func (s *stubScheme) SensitiveParams() []string {
	return s.sensitive
}

// newTestCodec builds a codec with a fresh random key.
func newTestCodec(t *testing.T) *SecretCodec {
	t.Helper()

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err, "Key generation should succeed")

	codec, err := NewSecretCodec(&config.IntegrationConfig{SecretKey: base64.StdEncoding.EncodeToString(key)})
	require.NoError(t, err, "Codec construction should succeed")

	return codec
}

// bearerScheme mimics the built-in bearer scheme's sensitivity declaration
// (sensitive param: token).
func bearerScheme(*testing.T) *stubScheme {
	return &stubScheme{sensitive: []string{"token"}}
}

func TestSecretCodec(t *testing.T) {
	codec := newTestCodec(t)
	scheme := bearerScheme(t)

	t.Run("EncryptDecryptRoundTrip", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "top-secret"}}

		require.NoError(t, codec.EncryptOutboundAuth(scheme, cfg, nil), "Encryption should succeed")
		assert.True(t, strings.HasPrefix(cfg.Params["token"], "enc:"), "Stored value should carry the encryption marker")
		assert.NotContains(t, cfg.Params["token"], "top-secret", "Stored value should not contain the plaintext")

		params, err := codec.DecryptOutboundAuth(scheme, cfg)
		require.NoError(t, err, "Decryption should succeed")
		assert.Equal(t, "top-secret", params["token"], "Decrypted value should match the original")
		assert.True(t, strings.HasPrefix(cfg.Params["token"], "enc:"), "DecryptOutboundAuth should not mutate the stored config")
	})

	t.Run("EncryptIsIdempotent", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "top-secret"}}

		require.NoError(t, codec.EncryptOutboundAuth(scheme, cfg, nil), "First encryption should succeed")
		sealed := cfg.Params["token"]

		require.NoError(t, codec.EncryptOutboundAuth(scheme, cfg, nil), "Second encryption should succeed")
		assert.Equal(t, sealed, cfg.Params["token"], "Re-encrypting an encrypted value should not double-encrypt")
	})

	t.Run("MaskedPlaceholderRestoresPriorValue", func(t *testing.T) {
		prior := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "enc:stored"}}
		incoming := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": integration.MaskedSecret}}

		require.NoError(t, codec.EncryptOutboundAuth(scheme, incoming, prior), "Masked update should succeed")
		assert.Equal(t, "enc:stored", incoming.Params["token"], "Masked placeholder should restore the stored value")
	})

	t.Run("MaskedPlaceholderWithoutPriorFails", func(t *testing.T) {
		incoming := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": integration.MaskedSecret}}

		err := codec.EncryptOutboundAuth(scheme, incoming, nil)
		require.Error(t, err, "Masked value without a stored prior should fail")
		assert.ErrorIs(t, err, ErrMaskedSecretWithoutPrior, "Error should be the masked-without-prior sentinel")
	})

	t.Run("NonSensitiveParamsStayPlaintext", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "s", "region": "east"}}

		require.NoError(t, codec.EncryptOutboundAuth(scheme, cfg, nil), "Encryption should succeed")
		assert.Equal(t, "east", cfg.Params["region"], "Non-sensitive params should stay plaintext")
	})
}

func TestSecretCodecWithoutKey(t *testing.T) {
	codec, err := NewSecretCodec(new(config.IntegrationConfig))
	require.NoError(t, err, "Key-less codec construction should succeed")

	scheme := bearerScheme(t)

	t.Run("StoresPlaintext", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "plain"}}

		require.NoError(t, codec.EncryptOutboundAuth(scheme, cfg, nil), "Key-less encryption should pass through")
		assert.Equal(t, "plain", cfg.Params["token"], "Value should stay plaintext without a key")

		params, err := codec.DecryptOutboundAuth(scheme, cfg)
		require.NoError(t, err, "Key-less decryption of plaintext should succeed")
		assert.Equal(t, "plain", params["token"], "Plaintext should pass through")
	})

	t.Run("RefusesEncryptedValues", func(t *testing.T) {
		cfg := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "enc:abc"}}

		_, err := codec.DecryptOutboundAuth(scheme, cfg)
		require.Error(t, err, "Encrypted value without a key should fail loudly")
		assert.ErrorIs(t, err, ErrSecretKeyMissing, "Error should be the missing-key sentinel")
	})
}

func TestNewSecretCodecRejectsMalformedKey(t *testing.T) {
	_, err := NewSecretCodec(&config.IntegrationConfig{SecretKey: "not-base64!!!"})
	require.Error(t, err, "Malformed key should fail codec construction")
}

func TestSecretCodecDataSource(t *testing.T) {
	codec := newTestCodec(t)

	t.Run("PasswordRoundTrip", func(t *testing.T) {
		ds := &integration.DataSourceConfig{Kind: config.Postgres, Host: "db.example.com", Password: "db-pass"}

		require.NoError(t, codec.EncryptDataSource(ds, nil), "Encryption should succeed")
		assert.True(t, strings.HasPrefix(ds.Password, "enc:"), "Stored password should carry the encryption marker")

		decrypted, err := codec.DecryptDataSource(ds)
		require.NoError(t, err, "Decryption should succeed")
		assert.Equal(t, "db-pass", decrypted.Password, "Decrypted password should match the original")
		assert.Equal(t, "db.example.com", decrypted.Host, "Non-sensitive fields should pass through")
		assert.True(t, strings.HasPrefix(ds.Password, "enc:"), "DecryptDataSource should not mutate the stored config")
	})

	t.Run("MaskedPlaceholderRestoresPriorValue", func(t *testing.T) {
		prior := &integration.DataSourceConfig{Kind: config.Postgres, Password: "enc:stored"}
		incoming := &integration.DataSourceConfig{Kind: config.Postgres, Password: integration.MaskedSecret}

		require.NoError(t, codec.EncryptDataSource(incoming, prior), "Masked update should succeed")
		assert.Equal(t, "enc:stored", incoming.Password, "Masked placeholder should restore the stored value")
	})

	t.Run("MaskedPlaceholderWithoutPriorFails", func(t *testing.T) {
		incoming := &integration.DataSourceConfig{Kind: config.Postgres, Password: integration.MaskedSecret}

		err := codec.EncryptDataSource(incoming, nil)
		require.Error(t, err, "Masked password without a stored prior should fail")
		assert.ErrorIs(t, err, ErrMaskedSecretWithoutPrior, "Error should be the masked-without-prior sentinel")
	})

	t.Run("EmptyPasswordPassesThrough", func(t *testing.T) {
		ds := &integration.DataSourceConfig{Kind: config.SQLite}

		require.NoError(t, codec.EncryptDataSource(ds, nil), "Password-less config should pass")
		assert.Empty(t, ds.Password, "Empty password should stay empty")
	})
}

func TestMaskDataSource(t *testing.T) {
	t.Run("MasksPassword", func(t *testing.T) {
		masked := MaskDataSource(&integration.DataSourceConfig{Kind: config.Postgres, User: "u", Password: "p"})

		assert.Equal(t, integration.MaskedSecret, masked.Password, "Password should be masked")
		assert.Equal(t, "u", masked.User, "User should stay visible")
	})

	t.Run("NilPassesThrough", func(t *testing.T) {
		assert.Nil(t, MaskDataSource(nil), "Nil config should stay nil")
	})

	t.Run("DoesNotMutateOriginal", func(t *testing.T) {
		original := &integration.DataSourceConfig{Kind: config.Postgres, Password: "p"}
		_ = MaskDataSource(original)

		assert.Equal(t, "p", original.Password, "Masking should copy, not mutate")
	})
}

func TestMaskAuth(t *testing.T) {
	scheme := bearerScheme(t)

	t.Run("MasksSensitiveOnly", func(t *testing.T) {
		masked := MaskOutboundAuth(scheme, &integration.OutboundAuthConfig{
			Scheme: "bearer",
			Params: map[string]string{"token": "secret", "region": "east"},
		})

		assert.Equal(t, integration.MaskedSecret, masked.Params["token"], "Sensitive value should be masked")
		assert.Equal(t, "east", masked.Params["region"], "Non-sensitive value should stay visible")
	})

	t.Run("NilSchemeMasksEverything", func(t *testing.T) {
		masked := MaskOutboundAuth(nil, &integration.OutboundAuthConfig{
			Scheme: "vanished",
			Params: map[string]string{"a": "1", "b": "2"},
		})

		assert.Equal(t, integration.MaskedSecret, masked.Params["a"], "Unknown scheme should mask every value")
		assert.Equal(t, integration.MaskedSecret, masked.Params["b"], "Unknown scheme should mask every value")
	})

	t.Run("NilAuthPassesThrough", func(t *testing.T) {
		assert.Nil(t, MaskOutboundAuth(scheme, nil), "Nil auth should stay nil")
	})

	t.Run("DoesNotMutateOriginal", func(t *testing.T) {
		original := &integration.OutboundAuthConfig{Scheme: "bearer", Params: map[string]string{"token": "secret"}}
		_ = MaskOutboundAuth(scheme, original)

		assert.Equal(t, "secret", original.Params["token"], "Masking should copy, not mutate")
	})
}
