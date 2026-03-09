package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// BunDropColumnQuery implements the DropColumnQuery interface.
type BunDropColumnQuery struct {
	query *bun.DropColumnQuery
}

// NewDropColumnQuery creates a new DropColumnQuery.
func NewDropColumnQuery(db *BunDB) *BunDropColumnQuery {
	return &BunDropColumnQuery{
		query: db.db.NewDropColumn(),
	}
}

func (q *BunDropColumnQuery) Model(model any) DropColumnQuery {
	q.query.Model(model)

	return q
}

func (q *BunDropColumnQuery) Table(tables ...string) DropColumnQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunDropColumnQuery) Column(columns ...string) DropColumnQuery {
	q.query.Column(columns...)

	return q
}

func (q *BunDropColumnQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}
