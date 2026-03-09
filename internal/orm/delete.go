package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// NewDeleteQuery creates a new DeleteQuery instance with the provided database instance.
// It initializes the query builders and sets up the table schema context for proper query building.
func NewDeleteQuery(db *BunDB) *BunDeleteQuery {
	eb := &QueryExprBuilder{}
	dq := db.db.NewDelete()
	dialect := db.db.Dialect()
	query := &BunDeleteQuery{
		QueryBuilder: newQueryBuilder(db, dialect, dq, eb),

		db:      db,
		dialect: dialect,
		query:   dq,
		eb:      eb,

		returningColumns: newReturningColumns(),
	}
	eb.qb = query

	return query
}

// BunDeleteQuery is the concrete implementation of DeleteQuery interface.
// It wraps bun.DeleteQuery and provides additional functionality for expression building.
type BunDeleteQuery struct {
	QueryBuilder

	db      *BunDB
	dialect schema.Dialect
	eb      ExprBuilder
	query   *bun.DeleteQuery

	returningColumns *returningColumns
}

func (q *BunDeleteQuery) DB() DB {
	return q.db
}

func (q *BunDeleteQuery) With(name string, builder func(SelectQuery)) DeleteQuery {
	q.query.With(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunDeleteQuery) WithValues(name string, model any, withOrder ...bool) DeleteQuery {
	values := q.query.NewValues(model)
	if len(withOrder) > 0 && withOrder[0] {
		values.WithOrder()
	}

	q.query.With(name, values)

	return q
}

func (q *BunDeleteQuery) WithRecursive(name string, builder func(SelectQuery)) DeleteQuery {
	q.query.WithRecursive(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunDeleteQuery) Model(model any) DeleteQuery {
	q.query.Model(model)

	return q
}

func (q *BunDeleteQuery) ModelTable(name string, alias ...string) DeleteQuery {
	applyModelTable(name, alias, q.query.ModelTableExpr)

	return q
}

func (q *BunDeleteQuery) Table(name string, alias ...string) DeleteQuery {
	applyTable(name, alias, q.query.TableExpr, q.query.Table)

	return q
}

func (q *BunDeleteQuery) TableFrom(model any, alias ...string) DeleteQuery {
	applyTableFrom(q.query.TableExpr, q.db, model, alias)

	return q
}

func (q *BunDeleteQuery) TableExpr(builder func(ExprBuilder) any, alias ...string) DeleteQuery {
	applyTableExpr(q.query.TableExpr, q.eb, builder, alias)

	return q
}

func (q *BunDeleteQuery) TableSubQuery(builder func(SelectQuery), alias ...string) DeleteQuery {
	applyTableSubQuery(q.query.TableExpr, q.BuildSubQuery(builder), alias)

	return q
}

func (q *BunDeleteQuery) Where(builder func(ConditionBuilder)) DeleteQuery {
	cb := newQueryConditionBuilder(q.query.QueryBuilder(), q)
	builder(cb)

	return q
}

func (q *BunDeleteQuery) WherePK(columns ...string) DeleteQuery {
	q.query.WherePK(columns...)

	return q
}

func (q *BunDeleteQuery) WhereDeleted() DeleteQuery {
	q.query.WhereDeleted()

	return q
}

func (q *BunDeleteQuery) IncludeDeleted() DeleteQuery {
	q.query.WhereAllWithDeleted()

	return q
}

func (q *BunDeleteQuery) OrderBy(columns ...string) DeleteQuery {
	q.query.Order(columns...)

	return q
}

func (q *BunDeleteQuery) OrderByDesc(columns ...string) DeleteQuery {
	for _, column := range columns {
		q.query.OrderExpr("? DESC", q.eb.Column(column))
	}

	return q
}

func (q *BunDeleteQuery) OrderByExpr(builder func(ExprBuilder) any) DeleteQuery {
	q.query.OrderExpr("?", builder(q.eb))

	return q
}

func (q *BunDeleteQuery) ForceDelete() DeleteQuery {
	q.query.ForceDelete()

	return q
}

func (q *BunDeleteQuery) Limit(limit int) DeleteQuery {
	q.query.Limit(limit)

	return q
}

func (q *BunDeleteQuery) Returning(columns ...string) DeleteQuery {
	q.returningColumns.AddAll(columns...)

	return q
}

func (q *BunDeleteQuery) ReturningAll() DeleteQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(columnAll)

	return q
}

func (q *BunDeleteQuery) ReturningNone() DeleteQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(sqlNull)

	return q
}

func (q *BunDeleteQuery) Apply(fns ...ApplyFunc[DeleteQuery]) DeleteQuery {
	for _, fn := range fns {
		if fn != nil {
			fn(q)
		}
	}

	return q
}

func (q *BunDeleteQuery) ApplyIf(condition bool, fns ...ApplyFunc[DeleteQuery]) DeleteQuery {
	if condition {
		return q.Apply(fns...)
	}

	return q
}

func (q *BunDeleteQuery) beforeDelete() {
	if !q.returningColumns.IsEmpty() {
		q.query.Returning("?", buildReturningExpr(q.returningColumns.Values(), q.eb))
	}
}

func (q *BunDeleteQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	q.beforeDelete()

	res, err := q.query.Exec(ctx, dest...)
	if err != nil {
		return nil, translateDeleteError(err)
	}

	return res, nil
}

func (q *BunDeleteQuery) Scan(ctx context.Context, dest ...any) error {
	q.beforeDelete()

	if err := q.query.Scan(ctx, dest...); err != nil {
		return translateDeleteError(err)
	}

	return nil
}
