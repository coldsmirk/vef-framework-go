package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// BunTruncateTableQuery implements the TruncateTableQuery interface.
type BunTruncateTableQuery struct {
	query *bun.TruncateTableQuery
}

// NewTruncateTableQuery creates a new TruncateTableQuery.
func NewTruncateTableQuery(db *BunDB) *BunTruncateTableQuery {
	return &BunTruncateTableQuery{
		query: db.db.NewTruncateTable(),
	}
}

func (q *BunTruncateTableQuery) Model(model any) TruncateTableQuery {
	q.query.Model(model)

	return q
}

func (q *BunTruncateTableQuery) Table(tables ...string) TruncateTableQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunTruncateTableQuery) ContinueIdentity() TruncateTableQuery {
	q.query.ContinueIdentity()

	return q
}

func (q *BunTruncateTableQuery) Cascade() TruncateTableQuery {
	q.query.Cascade()

	return q
}

func (q *BunTruncateTableQuery) Restrict() TruncateTableQuery {
	q.query.Restrict()

	return q
}

func (q *BunTruncateTableQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}
