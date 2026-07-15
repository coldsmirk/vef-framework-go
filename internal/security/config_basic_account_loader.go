package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

// NewConfigBasicAccountLoader builds the framework's default
// security.BasicAccountLoader from the static vef.security.basic_accounts
// configuration. Because the source is immutable deployment config, every
// account is validated eagerly: a blank username or a blank password fails
// construction (and therefore application start-up) instead of silently
// denying every request at runtime.
func NewConfigBasicAccountLoader(cfg *config.SecurityConfig) (security.BasicAccountLoader, error) {
	for username, account := range cfg.BasicAccounts {
		if strings.TrimSpace(username) == "" {
			return nil, ErrBasicAccountUsernameBlank
		}

		if account.Password == "" {
			return nil, fmt.Errorf("%w: basic account %q", ErrBasicAccountPasswordBlank, username)
		}
	}

	return &configBasicAccountLoader{accounts: cfg.BasicAccounts}, nil
}

type configBasicAccountLoader struct {
	accounts map[string]config.BasicAccountConfig
}

// LoadByUsername resolves a configured account to a synthesized external-app
// principal ("http_basic:<username>") and its stored secret; the comparison
// against the presented password stays in the framework.
func (l *configBasicAccountLoader) LoadByUsername(_ context.Context, username string) (*security.Principal, string, error) {
	account, ok := l.accounts[username]
	if !ok {
		return nil, "", nil
	}

	return security.NewExternalApp("http_basic:"+username, username, account.Roles...), account.Password, nil
}
