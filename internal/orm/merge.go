package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

const defaultSourceAlias = "src"

// NewMergeQuery creates a new MergeQuery instance with the provided database instance.
// It initializes the query builders and sets up the table schema context for proper query building.
func NewMergeQuery(db *BunDB) *BunMergeQuery {
	eb := &QueryExprBuilder{}
	mq := db.db.NewMerge()
	dialect := db.db.Dialect()
	query := &BunMergeQuery{
		QueryBuilder: newQueryBuilder(db, dialect, mq, eb),

		query: mq,

		returningColumns: newReturningColumns(),
	}
	eb.qb = query

	return query
}

// BunMergeQuery is the concrete implementation of MergeQuery interface.
// It wraps bun.MergeQuery and provides additional functionality for expression building.
type BunMergeQuery struct {
	QueryBuilder

	query    *bun.MergeQuery
	srcAlias string

	returningColumns *returningColumns
}

func (q *BunMergeQuery) With(name string, builder func(SelectQuery)) MergeQuery {
	q.query.With(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunMergeQuery) WithValues(name string, model any) MergeQuery {
	q.query.With(name, q.query.NewValues(model))

	return q
}

func (q *BunMergeQuery) WithOrderedValues(name string, model any) MergeQuery {
	q.query.With(name, q.query.NewValues(model).WithOrder())

	return q
}

func (q *BunMergeQuery) WithRecursive(name string, builder func(SelectQuery)) MergeQuery {
	q.query.WithRecursive(name, q.BuildSubQuery(builder))

	return q
}

func (q *BunMergeQuery) Model(model any) MergeQuery {
	q.query.Model(model)

	return q
}

func (q *BunMergeQuery) ModelTable(name string, alias ...string) MergeQuery {
	applyModelTable(name, alias, q.query.ModelTableExpr)

	return q
}

func (q *BunMergeQuery) Table(name string, alias ...string) MergeQuery {
	applyTable(name, alias, q.query.TableExpr, q.query.Table)

	return q
}

func (q *BunMergeQuery) TableFrom(model any, alias ...string) MergeQuery {
	applyTableFrom(q.query.TableExpr, q.DB(), model, alias)

	return q
}

func (q *BunMergeQuery) TableExpr(builder func(ExprBuilder) any, alias ...string) MergeQuery {
	applyTableExpr(q.query.TableExpr, q.ExprBuilder(), builder, alias)

	return q
}

func (q *BunMergeQuery) TableSubQuery(builder func(SelectQuery), alias ...string) MergeQuery {
	applyTableSubQuery(q.query.TableExpr, q.BuildSubQuery(builder), alias)

	return q
}

func (q *BunMergeQuery) Using(model any, alias ...string) MergeQuery {
	table := q.DB().TableOf(model)

	q.srcAlias = table.Alias
	if len(alias) > 0 && alias[0] != "" {
		q.srcAlias = alias[0]
	}

	if q.srcAlias == "" {
		q.srcAlias = table.Name
	}

	q.query.Using("? AS ?", bun.Name(table.Name), bun.Name(q.srcAlias))

	return q
}

func (q *BunMergeQuery) UsingTable(table string, alias ...string) MergeQuery {
	if len(alias) > 0 && alias[0] != "" {
		q.srcAlias = alias[0]
		q.query.Using("? AS ?", bun.Name(table), bun.Name(alias[0]))
	} else {
		q.srcAlias = table
		q.query.Using("?", bun.Name(table))
	}

	return q
}

func (q *BunMergeQuery) UsingExpr(builder func(ExprBuilder) any, alias ...string) MergeQuery {
	q.srcAlias = defaultSourceAlias
	if len(alias) > 0 && alias[0] != "" {
		q.srcAlias = alias[0]
	}

	q.query.Using("(?) AS ?", builder(q.ExprBuilder()), bun.Name(q.srcAlias))

	return q
}

func (q *BunMergeQuery) UsingSubQuery(builder func(SelectQuery), alias ...string) MergeQuery {
	q.srcAlias = defaultSourceAlias
	if len(alias) > 0 && alias[0] != "" {
		q.srcAlias = alias[0]
	}

	q.query.Using("(?) AS ?", q.BuildSubQuery(builder), bun.Name(q.srcAlias))

	return q
}

func (q *BunMergeQuery) On(builder func(ConditionBuilder)) MergeQuery {
	q.query.On("?", q.BuildCondition(builder))

	return q
}

func (q *BunMergeQuery) WhenMatched(builder ...func(ConditionBuilder)) MergeWhenBuilder {
	return newMergeWhenBuilder(q, q.srcAlias, "MATCHED", builder...)
}

func (q *BunMergeQuery) WhenNotMatched(builder ...func(ConditionBuilder)) MergeWhenBuilder {
	return newMergeWhenBuilder(q, q.srcAlias, "NOT MATCHED", builder...)
}

func (q *BunMergeQuery) WhenNotMatchedByTarget(builder ...func(ConditionBuilder)) MergeWhenBuilder {
	return newMergeWhenBuilder(q, q.srcAlias, "NOT MATCHED BY TARGET", builder...)
}

func (q *BunMergeQuery) WhenNotMatchedBySource(builder ...func(ConditionBuilder)) MergeWhenBuilder {
	return newMergeWhenBuilder(q, q.srcAlias, "NOT MATCHED BY SOURCE", builder...)
}

func (q *BunMergeQuery) Returning(columns ...string) MergeQuery {
	q.returningColumns.AddAll(columns...)

	return q
}

func (q *BunMergeQuery) ReturningAll() MergeQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(columnAll)

	return q
}

func (q *BunMergeQuery) ReturningNone() MergeQuery {
	q.returningColumns.Clear()
	q.returningColumns.AddAll(sqlNull)

	return q
}

func (q *BunMergeQuery) Apply(fns ...ApplyFunc[MergeQuery]) MergeQuery {
	for _, fn := range fns {
		if fn != nil {
			fn(q)
		}
	}

	return q
}

func (q *BunMergeQuery) ApplyIf(condition bool, fns ...ApplyFunc[MergeQuery]) MergeQuery {
	if condition {
		return q.Apply(fns...)
	}

	return q
}

func (q *BunMergeQuery) beforeMerge() {
	if q.returningColumns.IsEmpty() {
		return
	}

	// MERGE joins target and source, so a bare column name in RETURNING is
	// ambiguous; specific columns are qualified through eb.Column (which resolves
	// the target ?TableAlias). columnAll/sqlNull are emitted verbatim for
	// ReturningAll/ReturningNone.
	values := q.returningColumns.Values()
	exprs := make([]any, len(values))

	for i, column := range values {
		switch column {
		case columnAll, sqlNull:
			exprs[i] = bun.Safe(column)
		default:
			exprs[i] = q.ExprBuilder().Column(column)
		}
	}

	q.query.Returning("?", q.ExprBuilder().Exprs(exprs...))
}

func (q *BunMergeQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	q.beforeMerge()

	res, err := q.query.Exec(ctx, dest...)
	if err != nil {
		return nil, translateWriteError(err)
	}

	return res, nil
}

func (q *BunMergeQuery) Scan(ctx context.Context, dest ...any) error {
	q.beforeMerge()

	if err := q.query.Scan(ctx, dest...); err != nil {
		return translateWriteError(err)
	}

	return nil
}
