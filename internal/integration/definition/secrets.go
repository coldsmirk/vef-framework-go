package definition

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

// secretScheme is the codec's view of an auth scheme — outbound or inbound —
// reduced to the declaration of which parameters are secrets.
type secretScheme interface {
	// SensitiveParams names the encrypted-at-rest parameters; the
	// integration.SensitiveAll wildcard marks every parameter sensitive.
	SensitiveParams() []string
}

// sensitiveNames resolves a scheme's sensitivity declaration against the
// actual parameters: a nil scheme (no longer registered) and the SensitiveAll
// wildcard both select every parameter — fail closed.
func sensitiveNames(scheme secretScheme, params map[string]string) []string {
	declared := []string{integration.SensitiveAll}
	if scheme != nil {
		declared = scheme.SensitiveParams()
	}

	return resolveSensitiveNames(declared, params)
}

// resolveSensitiveNames turns a sensitivity declaration into the concrete
// parameter names present: the SensitiveAll wildcard selects every parameter.
func resolveSensitiveNames(declared []string, params map[string]string) []string {
	if slices.Contains(declared, integration.SensitiveAll) {
		return slices.Collect(maps.Keys(params))
	}

	return declared
}

// SensitiveValues returns the non-empty values of the parameters declared
// sensitive (SensitiveAll selecting every parameter). Wire captures scrub
// these values so a credential never lands in the invocation log or dry-run
// trace under whatever header or query name a scheme carries it — the point
// masking by a fixed name set cannot reach.
func SensitiveValues(declared []string, params map[string]string) []string {
	names := resolveSensitiveNames(declared, params)
	values := make([]string, 0, len(names))

	for _, name := range names {
		if value := params[name]; value != "" {
			values = append(values, value)
		}
	}

	return values
}

// EncryptOutboundAuth prepares auth for persistence, mutating its params in place:
// every sensitive parameter is encrypted, and a submitted MaskedSecret
// placeholder is replaced by the prior stored value (prior is nil on create).
func (c *SecretCodec) EncryptOutboundAuth(scheme secretScheme, auth, prior *integration.OutboundAuthConfig) error {
	if auth == nil {
		return nil
	}

	var priorParams map[string]string
	if prior != nil {
		priorParams = prior.Params
	}

	return c.encryptParams(scheme, auth.Params, priorParams)
}

// DecryptOutboundAuth returns a copy of auth with every sensitive parameter
// decrypted, ready to hand to OutboundAuthScheme.Apply. A nil auth decrypts
// to an empty config so schemes never see a nil receiver.
func (c *SecretCodec) DecryptOutboundAuth(scheme secretScheme, auth *integration.OutboundAuthConfig) (*integration.OutboundAuthConfig, error) {
	if auth == nil {
		return new(integration.OutboundAuthConfig), nil
	}

	params, err := c.decryptParams(scheme, auth.Params)
	if err != nil {
		return nil, err
	}

	return &integration.OutboundAuthConfig{Scheme: auth.Scheme, Params: params, Script: auth.Script}, nil
}

// MaskOutboundAuth returns a copy of auth with every non-empty sensitive parameter
// value replaced by MaskedSecret, for management API responses. A nil scheme
// (no longer registered) masks every parameter — fail closed. The signing
// script is code, not a secret; it passes through.
func MaskOutboundAuth(scheme secretScheme, auth *integration.OutboundAuthConfig) *integration.OutboundAuthConfig {
	if auth == nil {
		return nil
	}

	return &integration.OutboundAuthConfig{Scheme: auth.Scheme, Params: maskParams(scheme, auth.Params), Script: auth.Script}
}

// EncryptInboundAuth prepares an inbound auth config for persistence,
// mutating its params in place with the same masked-placeholder resolution as
// EncryptOutboundAuth. The verification script is code, not a secret; it stays
// plaintext.
func (c *SecretCodec) EncryptInboundAuth(scheme secretScheme, auth, prior *integration.InboundAuthConfig) error {
	if auth == nil {
		return nil
	}

	var priorParams map[string]string
	if prior != nil {
		priorParams = prior.Params
	}

	return c.encryptParams(scheme, auth.Params, priorParams)
}

// DecryptInboundAuth returns a copy of auth with every sensitive parameter
// decrypted, ready to hand to InboundAuthScheme.Verify.
func (c *SecretCodec) DecryptInboundAuth(scheme secretScheme, auth *integration.InboundAuthConfig) (*integration.InboundAuthConfig, error) {
	if auth == nil {
		return nil, nil
	}

	params, err := c.decryptParams(scheme, auth.Params)
	if err != nil {
		return nil, err
	}

	return &integration.InboundAuthConfig{Scheme: auth.Scheme, Params: params, Script: auth.Script}, nil
}

// MaskInboundAuth returns a copy of auth with every non-empty sensitive
// parameter value replaced by MaskedSecret, for management API responses. A
// nil scheme (no longer registered) masks every parameter — fail closed.
func MaskInboundAuth(scheme secretScheme, auth *integration.InboundAuthConfig) *integration.InboundAuthConfig {
	if auth == nil {
		return nil
	}

	return &integration.InboundAuthConfig{Scheme: auth.Scheme, Params: maskParams(scheme, auth.Params), Script: auth.Script}
}

// encryptParams encrypts the sensitive parameters in place, resolving
// submitted MaskedSecret placeholders against the prior stored values.
func (c *SecretCodec) encryptParams(scheme secretScheme, params, prior map[string]string) error {
	if len(params) == 0 {
		return nil
	}

	for _, name := range sensitiveNames(scheme, params) {
		value := params[name]
		if value == "" {
			continue
		}

		if value == integration.MaskedSecret {
			stored, ok := priorParam(prior, name)
			if !ok {
				return fmt.Errorf("%w: %s", ErrMaskedSecretWithoutPrior, name)
			}

			params[name] = stored

			continue
		}

		encrypted, err := c.encryptValue(value)
		if err != nil {
			return fmt.Errorf("integration: encrypt auth parameter %s: %w", name, err)
		}

		params[name] = encrypted
	}

	return nil
}

// decryptParams returns a copy of params with every sensitive parameter
// decrypted.
func (c *SecretCodec) decryptParams(scheme secretScheme, params map[string]string) (map[string]string, error) {
	if len(params) == 0 {
		return nil, nil
	}

	decrypted := maps.Clone(params)

	for _, name := range sensitiveNames(scheme, decrypted) {
		value, ok := decrypted[name]
		if !ok {
			continue
		}

		plain, err := c.decryptValue(value)
		if err != nil {
			return nil, fmt.Errorf("integration: decrypt auth parameter %s: %w", name, err)
		}

		decrypted[name] = plain
	}

	return decrypted, nil
}

// maskParams returns a copy of params with every non-empty sensitive value
// replaced by MaskedSecret.
func maskParams(scheme secretScheme, params map[string]string) map[string]string {
	masked := maps.Clone(params)

	for _, name := range sensitiveNames(scheme, masked) {
		if masked[name] != "" {
			masked[name] = integration.MaskedSecret
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

// priorParam looks up a stored parameter value on the prior params.
func priorParam(prior map[string]string, name string) (string, bool) {
	value, ok := prior[name]
	if !ok || value == "" {
		return "", false
	}

	return value, true
}
