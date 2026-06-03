package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/samber/lo"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/coldsmirk/vef-framework-go/config"
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
	connector := pgdriver.NewConnector(
		pgdriver.WithNetwork("tcp"),
		pgdriver.WithAddr(fmt.Sprintf(
			"%s:%d",
			lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1"),
			lo.Ternary(cfg.Port != 0, cfg.Port, uint16(5432)),
		)),
		pgdriver.WithInsecure(true),
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

func (*Provider) Version(ctx context.Context, db *sql.DB) (string, error) {
	return queryVersion(ctx, db)
}
