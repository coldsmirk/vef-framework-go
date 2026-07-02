package security

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

// NewConfigIPWhitelistLoader builds the framework's default security.IPWhitelistLoader
// from the static vef.security.ip_whitelists configuration. Because the source is
// immutable deployment config, every whitelist is validated eagerly: a blank name,
// an empty whitelist, or an entry that is neither an IP address nor a CIDR range
// fails construction (and therefore application start-up) instead of silently
// denying every request at runtime.
func NewConfigIPWhitelistLoader(cfg *config.SecurityConfig) (security.IPWhitelistLoader, error) {
	for name, entries := range cfg.IPWhitelists {
		if strings.TrimSpace(name) == "" {
			return nil, ErrIPWhitelistNameBlank
		}

		if err := validateIPWhitelistEntries(name, entries); err != nil {
			return nil, err
		}
	}

	return &configIPWhitelistLoader{whitelists: cfg.IPWhitelists}, nil
}

type configIPWhitelistLoader struct {
	whitelists map[string][]string
}

// LoadByName returns the named whitelist, or nil when the name is unknown.
func (l *configIPWhitelistLoader) LoadByName(_ context.Context, name string) (*security.IPWhitelist, error) {
	entries, ok := l.whitelists[name]
	if !ok {
		return nil, nil
	}

	return &security.IPWhitelist{Entries: entries}, nil
}

// validateIPWhitelistEntries rejects whitelists that could never authenticate a
// request: no usable entries, or an entry the validator would treat as a parse
// failure (which denies all IPs).
func validateIPWhitelistEntries(name string, entries []string) error {
	hasEntry := false

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		hasEntry = true

		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return fmt.Errorf("%w: whitelist %q entry %q: %w", ErrIPWhitelistEntryInvalid, name, entry, err)
			}

			continue
		}

		if net.ParseIP(entry) == nil {
			return fmt.Errorf("%w: whitelist %q entry %q", ErrIPWhitelistEntryInvalid, name, entry)
		}
	}

	if !hasEntry {
		return fmt.Errorf("%w: whitelist %q", ErrIPWhitelistEmpty, name)
	}

	return nil
}
