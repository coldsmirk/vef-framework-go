package oracle

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/samber/lo"

	goora "github.com/sijms/go-ora/v2"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/dbtls"
)

type Provider struct {
	dbKind config.DBKind
}

func NewProvider() *Provider {
	return &Provider{
		dbKind: config.Oracle,
	}
}

func (p *Provider) Kind() config.DBKind {
	return p.dbKind
}

func (p *Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, error) {
	databaseURL, err := p.buildDatabaseURL(cfg)
	if err != nil {
		return nil, err
	}

	return sql.OpenDB(goora.NewConnector(databaseURL)), nil
}

func (*Provider) Version(ctx context.Context, db *sql.DB) (string, error) {
	var version string

	return version, db.QueryRowContext(ctx, "SELECT banner FROM v$version WHERE ROWNUM = 1").Scan(&version)
}

// buildDatabaseURL assembles the go-ora connection URL. The Database field
// carries the Oracle service name and is mandatory; Schema is not mapped —
// Oracle resolves unqualified names against the connecting user's own schema.
func (*Provider) buildDatabaseURL(cfg *config.DataSourceConfig) (string, error) {
	if cfg.Database == "" {
		return "", ErrOracleServiceRequired
	}

	options, err := sslOptions(cfg)
	if err != nil {
		return "", err
	}

	return goora.BuildUrl(
		lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1"),
		int(lo.Ternary(cfg.Port != 0, cfg.Port, uint16(1521))),
		cfg.Database,
		cfg.User,
		cfg.Password,
		options,
	), nil
}

// sslOptions maps the libpq-style ssl_mode vocabulary onto go-ora's TCPS URL
// options. go-ora offers a single verification switch that performs full
// standard TLS verification (chain and hostname) against an Oracle wallet or
// the system pool, so verify-ca is served with verify-full semantics — the
// stronger check — and a PEM ssl_root_cert fails fast (see
// ErrOracleRootCertUnsupported).
func sslOptions(cfg *config.DataSourceConfig) (map[string]string, error) {
	switch cfg.SSLMode {
	case "", config.SSLDisable:
		return nil, nil

	case config.SSLRequire:
		return map[string]string{"SSL": "TRUE", "SSL VERIFY": "FALSE"}, nil

	case config.SSLVerifyCA, config.SSLVerifyFull:
		if cfg.SSLRootCert != "" {
			return nil, ErrOracleRootCertUnsupported
		}

		return map[string]string{"SSL": "TRUE"}, nil

	default:
		return nil, fmt.Errorf("%w: %s", dbtls.ErrUnknownSSLMode, cfg.SSLMode)
	}
}
