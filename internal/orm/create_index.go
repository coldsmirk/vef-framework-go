package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
)

// BunCreateIndexQuery implements the CreateIndexQuery interface with type-safe DDL operations.
type BunCreateIndexQuery struct {
	*BaseQueryBuilder

	query *bun.CreateIndexQuery
}

// NewCreateIndexQuery creates a new CreateIndexQuery with BaseQueryBuilder for expression support.
func NewCreateIndexQuery(db *BunDB) *BunCreateIndexQuery {
	eb := &QueryExprBuilder{}
	bunQuery := db.db.NewCreateIndex()
	q := &BunCreateIndexQuery{
		query: bunQuery,
	}
	q.BaseQueryBuilder = newDDLQueryBuilder(db, db.db.Dialect(), bunQuery, eb)
	eb.qb = q

	return q
}

func (q *BunCreateIndexQuery) Model(model any) CreateIndexQuery {
	q.query.Model(model)

	return q
}

func (q *BunCreateIndexQuery) Table(tables ...string) CreateIndexQuery {
	q.query.Table(tables...)

	return q
}

func (q *BunCreateIndexQuery) Index(name string) CreateIndexQuery {
	q.query.Index(name)

	return q
}

func (q *BunCreateIndexQuery) Column(columns ...string) CreateIndexQuery {
	q.query.Column(columns...)

	return q
}

func (q *BunCreateIndexQuery) ColumnExpr(builder func(ExprBuilder) any) CreateIndexQuery {
	expr := builder(q.eb)
	q.query.ColumnExpr("?", expr)

	return q
}

func (q *BunCreateIndexQuery) ExcludeColumn(columns ...string) CreateIndexQuery {
	q.query.ExcludeColumn(columns...)

	return q
}

func (q *BunCreateIndexQuery) Unique() CreateIndexQuery {
	q.query.Unique()

	return q
}

func (q *BunCreateIndexQuery) Concurrently() CreateIndexQuery {
	q.query.Concurrently()

	return q
}

func (q *BunCreateIndexQuery) IfNotExists() CreateIndexQuery {
	q.query.IfNotExists()

	return q
}

func (q *BunCreateIndexQuery) Include(columns ...string) CreateIndexQuery {
	q.query.Include(columns...)

	return q
}

func (q *BunCreateIndexQuery) Using(method IndexMethod) CreateIndexQuery {
	q.query.Using(method.String())

	return q
}

func (q *BunCreateIndexQuery) Where(builder func(ConditionBuilder)) CreateIndexQuery {
	condition := q.BuildCondition(builder)
	q.query.Where("?", condition)

	return q
}

func (q *BunCreateIndexQuery) Exec(ctx context.Context, dest ...any) (sql.Result, error) {
	return q.query.Exec(ctx, dest...)
}
