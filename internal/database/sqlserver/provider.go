package sqlserver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/samber/lo"

	mssql "github.com/microsoft/go-mssqldb"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database/dbtls"
)

type Provider struct {
	dbKind config.DBKind
}

func NewProvider() *Provider {
	return &Provider{
		dbKind: config.SQLServer,
	}
}

func (p *Provider) Kind() config.DBKind {
	return p.dbKind
}

func (p *Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, error) {
	connCfg, err := p.buildConfig(cfg)
	if err != nil {
		return nil, err
	}

	return sql.OpenDB(mssql.NewConnectorConfig(*connCfg)), nil
}

func (*Provider) Version(ctx context.Context, db *sql.DB) (string, error) {
	var version string

	return version, db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&version)
}

// buildConfig maps the data source settings onto the driver's native config.
// An empty Database lands on the login's default catalog, mirroring the
// server's own behavior. The libpq-style ssl_mode vocabulary translates
// through the shared dbtls resolver: any TLS mode forces TDS encryption on,
// while disable refuses encryption entirely; verify-ca/verify-full semantics
// (custom PEM root, hostname pinning) are carried by the resulting
// *tls.Config exactly as for Postgres and MySQL.
func (*Provider) buildConfig(cfg *config.DataSourceConfig) (*msdsn.Config, error) {
	host := lo.Ternary(cfg.Host != "", cfg.Host, "127.0.0.1")

	tlsConfig, err := dbtls.Config(cfg.SSLMode, cfg.SSLRootCert, host)
	if err != nil {
		return nil, fmt.Errorf("configure sqlserver tls: %w", err)
	}

	connCfg := &msdsn.Config{
		Host:     host,
		Port:     uint64(lo.Ternary(cfg.Port != 0, cfg.Port, uint16(1433))),
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.Database,
		AppName:  "vef",
		// msdsn.Parse populates Protocols from the DSN; a hand-built config
		// must list the dial protocol itself or the connector has nothing to
		// dial.
		Protocols:  []string{"tcp"},
		Encryption: msdsn.EncryptionDisabled,
	}

	if tlsConfig != nil {
		connCfg.Encryption = msdsn.EncryptionRequired
		connCfg.TLSConfig = tlsConfig
	}

	return connCfg, nil
}
