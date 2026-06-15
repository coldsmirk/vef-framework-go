package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/dbtls"
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

	mysqlCfg, err := p.buildConfig(cfg)
	if err != nil {
		return nil, err
	}

	connector, err := mysql.NewConnector(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql connector: %w", err)
	}

	return sql.OpenDB(connector), nil
}

func (*Provider) Version(ctx context.Context, db *sql.DB) (string, error) {
	var version string

	return version, db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
}

func (*Provider) buildConfig(cfg *config.DataSourceConfig) (*mysql.Config, error) {
	host := lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1")

	tlsConfig, err := dbtls.Config(cfg.SSLMode, cfg.SSLRootCert, host)
	if err != nil {
		return nil, fmt.Errorf("configure mysql tls: %w", err)
	}

	mysqlCfg := mysql.NewConfig()
	mysqlCfg.User = lo.Ternary(cfg.User != "", cfg.User, "root")
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = fmt.Sprintf(
		"%s:%d",
		host,
		lo.Ternary(cfg.Port != 0, cfg.Port, uint16(3306)),
	)
	mysqlCfg.DBName = cfg.Database
	mysqlCfg.ParseTime = true
	mysqlCfg.Collation = "utf8mb4_unicode_ci"
	mysqlCfg.MultiStatements = true
	// A nil TLS config leaves the driver in its default plaintext mode.
	mysqlCfg.TLS = tlsConfig

	return mysqlCfg, nil
}
