package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// BunAddColumnQuery implements the AddColumnQuery interface with type-safe DDL operations.
type BunAddColumnQuery struct {
	*BaseQueryBuilder

	query *bun.AddColumnQuery
}

// NewAddColumnQuery creates a new AddColumnQuery with BaseQueryBuilder for expression support.
func NewAddColumnQuery(db *BunDB) *BunAddColumnQuery {
	eb := &QueryExprBuilder{}
	bunQuery := db.db.NewAddColumn()
	q := &BunAddColumnQuery{
		query: bunQuery,
	}
	q.BaseQueryBuilder = newDDLQueryBuilder(db, db.db.Dialect(), bunQuery, eb)
	eb.qb = q

	return q
}

func (q *BunAddColumnQuery) Model(model any) AddColumnQuery {
	q.query.Model(model)

	return q
}

func (q *BunAddColumnQuery) Table(tables ...string) AddColumnQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunAddColumnQuery) Column(name string, dataType DataTypeDef, constraints ...ColumnConstraint) AddColumnQuery {
	queryStr, args := renderColumnDef(q.Dialect(), name, dataType, constraints, q)
	q.query.ColumnExpr(queryStr, args...)

	return q
}

func (q *BunAddColumnQuery) IfNotExists() AddColumnQuery {
	q.query.IfNotExists()

	return q
}

func (q *BunAddColumnQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}
