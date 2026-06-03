package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/config"
)

type Provider struct {
	dbKind config.DBKind
}

func NewProvider() *Provider {
	return &Provider{
		dbKind: config.MySQL,
	}
}

func (p *Provider) Kind() config.DBKind {
	return p.dbKind
}

func (p *Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, error) {
	if cfg.Database == "" {
		return nil, ErrMySQLDatabaseRequired
	}

	connector, err := mysql.NewConnector(p.buildConfig(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connector: %w", err)
	}

	return sql.OpenDB(connector), nil
}

func (*Provider) Version(ctx context.Context, db *sql.DB) (string, error) {
	return queryVersion(ctx, db)
}

func (*Provider) buildConfig(cfg *config.DataSourceConfig) *mysql.Config {
	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = lo.Ternary(cfg.User != "", cfg.User, "root")
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = fmt.Sprintf(
		"%s:%d",
		lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1"),
		lo.Ternary(cfg.Port != 0, cfg.Port, uint16(3306)),
	)
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.ParseTime = true
	mysqlCfg.Collation = "utf8mb4_unicode_ci"
	mysqlCfg.MultiStatements = true

	return mysqlCfg
}
