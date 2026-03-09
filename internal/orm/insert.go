package orm

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// NewInsertQuery creates a new InsertQuery instance with the provided database instance.
// It initializes the query builders and sets up the table schema context for proper query building.
func NewInsertQuery(db *BunDB) *BunInsertQuery {
	eb := &QueryExprBuilder{}
	iq := db.db.NewInsert()
	dialect := db.db.Dialect()
	query := &BunInsertQuery{
		QueryBuilder: newQueryBuilder(db, dialect, iq, eb),

		db:      db,
		dialect: dialect,
		eb:      eb,
		query:   iq,

		returningColumns: newReturningColumns(),
	}
	eb.qb = query

	return query
}

// BunInsertQuery is the concrete implementation of InsertQuery interface.
// It wraps bun.InsertQuery and provides additional functionality for expression building.
type BunInsertQuery struct {
	QueryBuilder

	db      *BunDB
	dialect schema.Dialect
	eb      ExprBuilder
	query   *bun.InsertQuery

	returningColumns *returningColumns
}

func (q *BunInsertQuery) DB() DB {
	return q.db
}

func (q *BunInsertQuery) With(name string, builder func(SelectQuery)) InsertQuery {
	q.query.With(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunInsertQuery) WithValues(name string, model any, withOrder ...bool) InsertQuery {
	values := q.query.NewValues(model)
	if len(withOrder) > 0 && withOrder[0] {
		values.WithOrder()
	}

	q.query.With(name, values)

	return q
}

func (q *BunInsertQuery) WithRecursive(name string, builder func(SelectQuery)) InsertQuery {
	q.query.WithRecursive(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunInsertQuery) Model(model any) InsertQuery {
	q.query.Model(model)

	return q
}

func (q *BunInsertQuery) ModelTable(name string, alias ...string) InsertQuery {
	applyModelTable(name, alias, q.query.ModelTableExpr)

	return q
}

func (q *BunInsertQuery) Table(name string, alias ...string) InsertQuery {
	applyTable(name, alias, q.query.TableExpr, q.query.Table)

	return q
}

func (q *BunInsertQuery) TableFrom(model any, alias ...string) InsertQuery {
	applyTableFrom(q.query.TableExpr, q.db, model, alias)

	return q
}

func (q *BunInsertQuery) TableExpr(builder func(ExprBuilder) any, alias ...string) InsertQuery {
	applyTableExpr(q.query.TableExpr, q.eb, builder, alias)

	return q
}

func (q *BunInsertQuery) TableSubQuery(builder func(SelectQuery), alias ...string) InsertQuery {
	applyTableSubQuery(q.query.TableExpr, q.BuildSubQuery(builder), alias)

	return q
}

// OnConflict configures conflict handling via a dialect-aware builder.
func (q *BunInsertQuery) OnConflict(builder func(ConflictBuilder)) InsertQuery {
	cb := newConflictBuilder(q)
	builder(cb)
	cb.build(q.query)

	return q
}

func (q *BunInsertQuery) SelectAll() InsertQuery {
	q.query.Column(columnAll)

	return q
}

func (q *BunInsertQuery) Select(columns ...string) InsertQuery {
	q.query.Column(columns...)

	return q
}

func (q *BunInsertQuery) Exclude(columns ...string) InsertQuery {
	q.query.ExcludeColumn(columns...)

	return q
}

func (q *BunInsertQuery) ExcludeAll() InsertQuery {
	q.query.ExcludeColumn(columnAll)

	return q
}

func (q *BunInsertQuery) Column(name string, value any) InsertQuery {
	q.query.Value(name, "?", value)

	return q
}

func (q *BunInsertQuery) ColumnExpr(name string, builder func(ExprBuilder) any) InsertQuery {
	q.query.Value(name, "?", builder(q.eb))

	return q
}

func (q *BunInsertQuery) Returning(columns ...string) InsertQuery {
	q.returningColumns.AddAll(columns...)

	return q
}

func (q *BunInsertQuery) ReturningAll() InsertQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(columnAll)

	return q
}

func (q *BunInsertQuery) ReturningNone() InsertQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(sqlNull)

	return q
}

func (q *BunInsertQuery) Apply(fns ...ApplyFunc[InsertQuery]) InsertQuery {
	for _, fn := range fns {
		if fn != nil {
			fn(q)
		}
	}

	return q
}

func (q *BunInsertQuery) ApplyIf(condition bool, fns ...ApplyFunc[InsertQuery]) InsertQuery {
	if condition {
		return q.Apply(fns...)
	}

	return q
}

// beforeInsert applies auto column handlers before executing the insert operation.
// It processes InsertHandler to automatically set values like IDs, timestamps, and user tracking.
func (q *BunInsertQuery) beforeInsert() {
	if table := q.GetTable(); table != nil {
		modelValue := q.query.GetModel().Value()
		mv := reflect.Indirect(reflect.ValueOf(modelValue))

		processAutoColumns(q, table, modelValue, mv)
	}

	if !q.returningColumns.IsEmpty() {
		q.query.Returning("?", buildReturningExpr(q.returningColumns.Values(), q.eb))
	}
}

func (q *BunInsertQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	q.beforeInsert()

	res, err := q.query.Exec(ctx, dest...)
	if err != nil {
		return nil, translateWriteError(err)
	}

	return res, nil
}

func (q *BunInsertQuery) Scan(ctx context.Context, dest ...any) error {
	q.beforeInsert()

	if err := q.query.Scan(ctx, dest...); err != nil {
		return translateWriteError(err)
	}

	return nil
}
