package service

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cryptox"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

// logger is the integration module's framework logger, following the
// repo-wide package-level convention.
var logger = logx.Named("integration")

// encryptedPrefix marks an auth parameter value as encrypted at rest, so
// plaintext values from key-less deployments stay readable and re-encryption
// is detectable.
const encryptedPrefix = "enc:"

// SecretCodec encrypts sensitive auth parameter values at rest with the
// AES-GCM key from vef.integration.secret_key. Without a configured key it
// degrades to plaintext storage — NewSecretCodec logs the warning once at
// boot — but still refuses to load values a previous configuration encrypted.
type SecretCodec struct {
	cipher cryptox.Cipher
}

// NewSecretCodec builds the codec from the configured secret key, failing
// fast on a malformed key.
func NewSecretCodec(cfg *config.IntegrationConfig) (*SecretCodec, error) {
	if cfg.SecretKey == "" {
		logger.Warn("vef.integration.secret_key is not configured; sensitive auth parameters are stored in plaintext")

		return new(SecretCodec), nil
	}

	cipher, err := cryptox.NewAESFromBase64(cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("integration: invalid vef.integration.secret_key: %w", err)
	}

	return &SecretCodec{cipher: cipher}, nil
}

// EncryptAuth prepares auth for persistence, mutating its params in place:
// every sensitive parameter is encrypted, and a submitted MaskedSecret
// placeholder is replaced by the prior stored value (prior is nil on create).
func (c *SecretCodec) EncryptAuth(scheme integration.AuthScheme, auth, prior *integration.AuthConfig) error {
	if auth == nil || len(auth.Params) == 0 {
		return nil
	}

	for _, name := range scheme.SensitiveParams() {
		value, ok := auth.Params[name]
		if !ok || value == "" {
			continue
		}

		if value == integration.MaskedSecret {
			stored, ok := priorParam(prior, name)
			if !ok {
				return fmt.Errorf("%w: %s", ErrMaskedSecretWithoutPrior, name)
			}

			auth.Params[name] = stored

			continue
		}

		encrypted, err := c.encryptValue(value)
		if err != nil {
			return fmt.Errorf("integration: encrypt auth parameter %s: %w", name, err)
		}

		auth.Params[name] = encrypted
	}

	return nil
}

// DecryptAuth returns a copy of auth's params with every sensitive parameter
// decrypted, ready to hand to AuthScheme.Apply.
func (c *SecretCodec) DecryptAuth(scheme integration.AuthScheme, auth *integration.AuthConfig) (map[string]string, error) {
	if auth == nil || len(auth.Params) == 0 {
		return nil, nil
	}

	params := maps.Clone(auth.Params)

	for _, name := range scheme.SensitiveParams() {
		value, ok := params[name]
		if !ok {
			continue
		}

		decrypted, err := c.decryptValue(value)
		if err != nil {
			return nil, fmt.Errorf("integration: decrypt auth parameter %s: %w", name, err)
		}

		params[name] = decrypted
	}

	return params, nil
}

// MaskAuth returns a copy of auth with every non-empty sensitive parameter
// value replaced by MaskedSecret, for management API responses. A nil scheme
// (no longer registered) masks every parameter — fail closed.
func MaskAuth(scheme integration.AuthScheme, auth *integration.AuthConfig) *integration.AuthConfig {
	if auth == nil {
		return nil
	}

	masked := &integration.AuthConfig{Scheme: auth.Scheme, Params: maps.Clone(auth.Params)}

	sensitive := maps.Keys(masked.Params)
	if scheme != nil {
		sensitive = slices.Values(scheme.SensitiveParams())
	}

	for name := range sensitive {
		if masked.Params[name] != "" {
			masked.Params[name] = integration.MaskedSecret
		}
	}

	return masked
}

// EncryptDataSource prepares a system's data source config for persistence,
// mutating it in place: the password is encrypted, and a submitted
// MaskedSecret placeholder is replaced by the prior stored value (prior is
// nil on create).
func (c *SecretCodec) EncryptDataSource(ds, prior *integration.DataSourceConfig) error {
	if ds == nil || ds.Password == "" {
		return nil
	}

	if ds.Password == integration.MaskedSecret {
		if prior == nil || prior.Password == "" {
			return fmt.Errorf("%w: password", ErrMaskedSecretWithoutPrior)
		}

		ds.Password = prior.Password

		return nil
	}

	encrypted, err := c.encryptValue(ds.Password)
	if err != nil {
		return fmt.Errorf("integration: encrypt data source password: %w", err)
	}

	ds.Password = encrypted

	return nil
}

// DecryptDataSource returns a copy of ds with the password decrypted, ready
// for the datasource registry.
func (c *SecretCodec) DecryptDataSource(ds *integration.DataSourceConfig) (integration.DataSourceConfig, error) {
	decrypted := *ds

	password, err := c.decryptValue(ds.Password)
	if err != nil {
		return integration.DataSourceConfig{}, fmt.Errorf("integration: decrypt data source password: %w", err)
	}

	decrypted.Password = password

	return decrypted, nil
}

// MaskDataSource returns a copy of ds with a non-empty password replaced by
// MaskedSecret, for management API responses.
func MaskDataSource(ds *integration.DataSourceConfig) *integration.DataSourceConfig {
	if ds == nil {
		return nil
	}

	masked := *ds
	if masked.Password != "" {
		masked.Password = integration.MaskedSecret
	}

	return &masked
}

// encryptValue seals one plaintext value; already-encrypted values pass
// through so re-saving a record never double-encrypts.
func (c *SecretCodec) encryptValue(value string) (string, error) {
	if c.cipher == nil || strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}

	ciphertext, err := c.cipher.Encrypt(value)
	if err != nil {
		return "", err
	}

	return encryptedPrefix + ciphertext, nil
}

// decryptValue opens one stored value; values without the encryption marker
// (plaintext from key-less deployments) pass through.
func (c *SecretCodec) decryptValue(value string) (string, error) {
	payload, ok := strings.CutPrefix(value, encryptedPrefix)
	if !ok {
		return value, nil
	}

	if c.cipher == nil {
		return "", ErrSecretKeyMissing
	}

	return c.cipher.Decrypt(payload)
}

// priorParam looks up a stored parameter value on the prior auth config.
func priorParam(prior *integration.AuthConfig, name string) (string, bool) {
	if prior == nil {
		return "", false
	}

	value, ok := prior.Params[name]
	if !ok || value == "" {
		return "", false
	}

	return value, true
}
