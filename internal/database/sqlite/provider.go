package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/config"
)

type Provider struct {
	dbKind config.DBKind
}

const (
	sqliteBusyTimeoutMs = 5000
	sqliteJournalMode   = "WAL"
)

func NewProvider() *Provider {
	return &Provider{
		dbKind: config.SQLite,
	}
}

func (p *Provider) Kind() config.DBKind {
	return p.dbKind
}

func (p *Provider) Connect(cfg *config.DataSourceConfig) (*sql.DB, schema.Dialect, error) {
	if err := p.ValidateConfig(cfg); err != nil {
		return nil, nil, err
	}

	db, err := sql.Open(sqliteshim.ShimName, p.buildDsn(cfg))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	return db, sqlitedialect.New(), nil
}

func (*Provider) ValidateConfig(_ *config.DataSourceConfig) error {
	return nil
}

func (*Provider) QueryVersion(db *bun.DB) (string, error) {
	return queryVersion(db)
}

// buildDsn returns the DSN for SQLite. When no path is specified, it uses
// file::memory: with shared cache to ensure multiple connections share
// the same in-memory database.
func (*Provider) buildDsn(cfg *config.DataSourceConfig) string {
	busyTimeoutParam := fmt.Sprintf("_busy_timeout=%d", sqliteBusyTimeoutMs)
	busyTimeoutPragma := fmt.Sprintf("_pragma=busy_timeout(%d)", sqliteBusyTimeoutMs)
	baseParams := []string{busyTimeoutParam, busyTimeoutPragma}

	if cfg.Path == "" {
		memParams := append(baseParams, "_pragma=foreign_keys(ON)")

		return withSQLiteParams(
			"file::memory:?mode=memory&cache=shared",
			memParams...,
		)
	}

	fileParams := append(baseParams,
		fmt.Sprintf("_pragma=journal_mode(%s)", sqliteJournalMode),
		"_pragma=synchronous(NORMAL)",
		"_pragma=foreign_keys(ON)",
	)

	return withSQLiteParams("file:"+cfg.Path, fileParams...)
}

func withSQLiteParams(dsn string, params ...string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + strings.Join(params, "&")
	}

	return dsn + "?" + strings.Join(params, "&")
}
