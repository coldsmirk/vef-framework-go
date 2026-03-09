package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// BunDropTableQuery implements the DropTableQuery interface.
type BunDropTableQuery struct {
	query *bun.DropTableQuery
}

// NewDropTableQuery creates a new DropTableQuery.
func NewDropTableQuery(db *BunDB) *BunDropTableQuery {
	return &BunDropTableQuery{
		query: db.db.NewDropTable(),
	}
}

func (q *BunDropTableQuery) Model(model any) DropTableQuery {
	q.query.Model(model)

	return q
}

func (q *BunDropTableQuery) Table(tables ...string) DropTableQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunDropTableQuery) IfExists() DropTableQuery {
	q.query.IfExists()

	return q
}

func (q *BunDropTableQuery) Cascade() DropTableQuery {
	q.query.Cascade()

	return q
}

func (q *BunDropTableQuery) Restrict() DropTableQuery {
	q.query.Restrict()

	return q
}

func (q *BunDropTableQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}

func (q *BunDropTableQuery) String() string {
	return q.query.String()
}
