package orm

import (
	"context"
	"database/sql"
	"reflect"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/feature"
	"github.com/uptrace/bun/schema"

	collections "github.com/coldsmirk/go-collections"
)

// NewUpdateQuery creates a new UpdateQuery instance with the provided database instance.
// It initializes the query builders and sets up the table schema context for proper query building.
func NewUpdateQuery(db *BunDB) *BunUpdateQuery {
	eb := &QueryExprBuilder{}
	uq := db.db.NewUpdate()
	dialect := db.db.Dialect()
	query := &BunUpdateQuery{
		QueryBuilder: newQueryBuilder(db, dialect, uq, eb),

		db:      db,
		dialect: dialect,
		eb:      eb,
		query:   uq,

		selectedColumns:  collections.NewHashSet[string](),
		returningColumns: newReturningColumns(),
	}
	eb.qb = query

	return query
}

// BunUpdateQuery is the concrete implementation of UpdateQuery interface.
// It wraps bun.UpdateQuery and provides additional functionality for expression building.
type BunUpdateQuery struct {
	QueryBuilder

	db               *BunDB
	dialect          schema.Dialect
	eb               ExprBuilder
	query            *bun.UpdateQuery
	hasSet           bool
	isBulk           bool
	selectedColumns  collections.Set[string]
	returningColumns *returningColumns
}

func (q *BunUpdateQuery) DB() DB {
	return q.db
}

func (q *BunUpdateQuery) With(name string, builder func(SelectQuery)) UpdateQuery {
	q.query.With(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunUpdateQuery) WithValues(name string, model any, withOrder ...bool) UpdateQuery {
	values := q.query.NewValues(model)
	if len(withOrder) > 0 && withOrder[0] {
		values.WithOrder()
	}

	q.query.With(name, values)

	return q
}

func (q *BunUpdateQuery) WithRecursive(name string, builder func(SelectQuery)) UpdateQuery {
	q.query.WithRecursive(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunUpdateQuery) Model(model any) UpdateQuery {
	q.query.Model(model)

	return q
}

func (q *BunUpdateQuery) ModelTable(name string, alias ...string) UpdateQuery {
	applyModelTable(name, alias, q.query.ModelTableExpr)

	return q
}

func (q *BunUpdateQuery) Table(name string, alias ...string) UpdateQuery {
	applyTable(name, alias, q.query.TableExpr, q.query.Table)

	return q
}

func (q *BunUpdateQuery) TableFrom(model any, alias ...string) UpdateQuery {
	applyTableFrom(q.query.TableExpr, q.db, model, alias)

	return q
}

func (q *BunUpdateQuery) TableExpr(builder func(ExprBuilder) any, alias ...string) UpdateQuery {
	applyTableExpr(q.query.TableExpr, q.eb, builder, alias)

	return q
}

func (q *BunUpdateQuery) TableSubQuery(builder func(SelectQuery), alias ...string) UpdateQuery {
	applyTableSubQuery(q.query.TableExpr, q.BuildSubQuery(builder), alias)

	return q
}

func (q *BunUpdateQuery) Where(builder func(ConditionBuilder)) UpdateQuery {
	cb := newQueryConditionBuilder(q.query.QueryBuilder(), q)
	builder(cb)

	return q
}

func (q *BunUpdateQuery) WherePK(columns ...string) UpdateQuery {
	q.query.WherePK(columns...)

	return q
}

func (q *BunUpdateQuery) WhereDeleted() UpdateQuery {
	q.query.WhereDeleted()

	return q
}

func (q *BunUpdateQuery) IncludeDeleted() UpdateQuery {
	q.query.WhereAllWithDeleted()

	return q
}

func (q *BunUpdateQuery) SelectAll() UpdateQuery {
	return q
}

func (q *BunUpdateQuery) Select(columns ...string) UpdateQuery {
	q.query.Column(columns...)
	q.selectedColumns.AddAll(columns...)

	return q
}

func (q *BunUpdateQuery) Exclude(columns ...string) UpdateQuery {
	q.query.ExcludeColumn(columns...)
	q.selectedColumns.RemoveAll(columns...)

	return q
}

func (q *BunUpdateQuery) ExcludeAll() UpdateQuery {
	q.query.ExcludeColumn(columnAll)
	q.selectedColumns.Clear()

	return q
}

func (q *BunUpdateQuery) Column(name string, value any) UpdateQuery {
	q.query.Value(name, "?", value)

	return q
}

func (q *BunUpdateQuery) ColumnExpr(name string, builder func(ExprBuilder) any) UpdateQuery {
	q.query.Value(name, "?", builder(q.eb))

	return q
}

func (q *BunUpdateQuery) Set(name string, value any) UpdateQuery {
	q.setColumn(name, value)

	return q
}

func (q *BunUpdateQuery) SetExpr(name string, builder func(ExprBuilder) any) UpdateQuery {
	q.setColumn(name, builder(q.eb))

	return q
}

// setColumn applies a SET clause using qualified or simple column names based on dialect support.
func (q *BunUpdateQuery) setColumn(name string, value any) {
	if q.query.DB().HasFeature(feature.UpdateMultiTable) {
		q.query.Set("? = ?", q.eb.Column(name), value)
	} else {
		// Strip table alias prefix (e.g., "t.field_name" → "field_name")
		// since databases without UpdateMultiTable do not support aliased SET targets.
		if _, after, ok := strings.Cut(name, "."); ok {
			name = after
		}

		q.query.Set("? = ?", bun.Name(name), value)
	}

	q.hasSet = true
}

func (q *BunUpdateQuery) OmitZero() UpdateQuery {
	q.query.OmitZero()

	return q
}

func (q *BunUpdateQuery) OrderBy(columns ...string) UpdateQuery {
	q.query.Order(columns...)

	return q
}

func (q *BunUpdateQuery) OrderByDesc(columns ...string) UpdateQuery {
	for _, column := range columns {
		q.query.OrderExpr("? DESC", q.eb.Column(column))
	}

	return q
}

func (q *BunUpdateQuery) OrderByExpr(builder func(ExprBuilder) any) UpdateQuery {
	q.query.OrderExpr("?", builder(q.eb))

	return q
}

func (q *BunUpdateQuery) Limit(limit int) UpdateQuery {
	q.query.Limit(limit)

	return q
}

func (q *BunUpdateQuery) Returning(columns ...string) UpdateQuery {
	q.returningColumns.AddAll(columns...)

	return q
}

func (q *BunUpdateQuery) ReturningAll() UpdateQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(columnAll)

	return q
}

func (q *BunUpdateQuery) ReturningNone() UpdateQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(sqlNull)

	return q
}

func (q *BunUpdateQuery) Bulk() UpdateQuery {
	q.isBulk = true
	q.query.Bulk()

	return q
}

func (q *BunUpdateQuery) Apply(fns ...ApplyFunc[UpdateQuery]) UpdateQuery {
	for _, fn := range fns {
		if fn != nil {
			fn(q)
		}
	}

	return q
}

func (q *BunUpdateQuery) ApplyIf(condition bool, fns ...ApplyFunc[UpdateQuery]) UpdateQuery {
	if condition {
		return q.Apply(fns...)
	}

	return q
}

func (q *BunUpdateQuery) beforeUpdate() {
	if table := q.GetTable(); table != nil {
		q.skipCreateAuditColumns(table)

		modelValue := q.query.GetModel().Value()
		mv := reflect.Indirect(reflect.ValueOf(modelValue))

		processAutoColumns(q, table, modelValue, mv)
	}

	if !q.returningColumns.IsEmpty() {
		q.query.Returning("?", buildReturningExpr(q.returningColumns.Values(), q.eb))
	}
}

func (q *BunUpdateQuery) skipCreateAuditColumns(table *schema.Table) {
	if q.hasSet || !q.selectedColumns.IsEmpty() {
		return
	}

	if table.HasField(ColumnCreatedAt) {
		q.Exclude(ColumnCreatedAt)
	}

	if table.HasField(ColumnCreatedBy) {
		q.Exclude(ColumnCreatedBy)
	}
}

func (q *BunUpdateQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	q.beforeUpdate()

	res, err := q.query.Exec(ctx, dest...)
	if err != nil {
		return nil, translateWriteError(err)
	}

	return res, nil
}

func (q *BunUpdateQuery) Scan(ctx context.Context, dest ...any) error {
	q.beforeUpdate()

	if err := q.query.Scan(ctx, dest...); err != nil {
		return translateWriteError(err)
	}

	return nil
}
