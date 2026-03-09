package postgres

import (
	"database/sql"
	"fmt"

	"github.com/samber/lo"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/schema"

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

func (p *Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, schema.Dialect, error) {
	if err := p.ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}

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

	return sql.OpenDB(connector), pgdialect.New(), nil
}

func (*Provider) ValidateConfig(_ *config.DataSourceConfig) error {
	return nil
}

func (*Provider) QueryVersion(db *bun.DB) (string, error) {
	return queryVersion(db)
}
