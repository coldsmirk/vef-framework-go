package security

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

// NewConfigAPIKeyLoader builds the framework's default security.APIKeyLoader
// from the static vef.security.api_keys configuration. Because the source is
// immutable deployment config, every entry is validated eagerly: a blank name
// or a blank key value fails construction (and therefore application
// start-up) instead of silently denying every request at runtime.
func NewConfigAPIKeyLoader(cfg *config.SecurityConfig) (security.APIKeyLoader, error) {
	for name, key := range cfg.APIKeys {
		if strings.TrimSpace(name) == "" {
			return nil, ErrAPIKeyNameBlank
		}

		if key.Key == "" {
			return nil, fmt.Errorf("%w: api key %q", ErrAPIKeyValueBlank, name)
		}
	}

	return &configAPIKeyLoader{keys: cfg.APIKeys}, nil
}

type configAPIKeyLoader struct {
	keys map[string]config.APIKeyConfig
}

// LoadByKey scans the configured keys with constant-time comparison and
// resolves a match to a synthesized external-app principal ("api_key:<name>").
// Every candidate is compared even after a match, so the scan time does not
// depend on which entry (if any) matched.
func (l *configAPIKeyLoader) LoadByKey(_ context.Context, key string) (*security.Principal, error) {
	var matched string

	for name, candidate := range l.keys {
		if subtle.ConstantTimeCompare([]byte(candidate.Key), []byte(key)) == 1 {
			matched = name
		}
	}

	if matched == "" {
		return nil, nil
	}

	return security.NewExternalApp("api_key:"+matched, matched, l.keys[matched].Roles...), nil
}
