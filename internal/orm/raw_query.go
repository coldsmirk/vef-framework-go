package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

type bunRawQuery struct {
	db    *BunDB
	query *bun.RawQuery
}

func newRawQuery(db *BunDB, query string, args ...any) *bunRawQuery {
	return &bunRawQuery{
		db:    db,
		query: db.db.NewRaw(query, args...),
	}
}

func (b *bunRawQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return b.query.Exec(ctx, dest...)
}

func (b *bunRawQuery) Scan(ctx context.Context, dest ...any) error {
	return b.query.Scan(ctx, dest...)
}
