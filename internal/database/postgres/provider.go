package postgres

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"

	"github.com/samber/lo"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/dbtls"
)

type Provider struct {
	dbKind config.DBKind
}

func NewProvider() *Provider {
	return &Provider{
		dbKind: config.Postgres,
	}
}

func (p *Provider) Kind() config.DBKind {
	return p.dbKind
}

func (*Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, error) {
	host := lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1")

	tlsConfig, err := dbtls.Config(cfg.SSLMode, cfg.SSLRootCert, host)
	if err != nil {
		return nil, fmt.Errorf("configure postgres tls: %w", err)
	}

	connector := pgdriver.NewConnector(
		pgdriver.WithNetwork("tcp"),
		pgdriver.WithAddr(fmt.Sprintf(
			"%s:%d",
			host,
			lo.Ternary(cfg.Port != 0, cfg.Port, uint16(5432)),
		)),
		tlsOption(tlsConfig),
		pgdriver.WithUser(lo.Ternary(cfg.User != "", cfg.User, "postgres")),
		pgdriver.WithPassword(lo.Ternary(cfg.Password != "", cfg.Password, "postgres")),
		pgdriver.WithDatabase(lo.Ternary(cfg.Database != "", cfg.Database, "postgres")),
		pgdriver.WithApplicationName("vef"),
		pgdriver.WithConnParams(map[string]any{
			"search_path": lo.Ternary(cfg.Schema != "", cfg.Schema, "public"),
		}),
	)

	return sql.OpenDB(connector), nil
}

// tlsOption maps the resolved *tls.Config onto a pgdriver option. A nil config
// means TLS is disabled, which pgdriver expresses as WithInsecure(true).
func tlsOption(tlsConfig *tls.Config) pgdriver.Option {
	if tlsConfig == nil {
		return pgdriver.WithInsecure(true)
	}

	return pgdriver.WithTLSConfig(tlsConfig)
}

func (*Provider) Version(ctx context.Context, db *sql.DB) (string, error) {
	var version string

	return version, db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
}
