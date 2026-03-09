package orm

import (
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// QueryBuilder defines the common interface for building subqueries and conditions.
type QueryBuilder interface {
	fmt.Stringer

	// Dialect returns the current database dialect (PostgreSQL, MySQL, or SQLite).
	Dialect() schema.Dialect
	// GetTable returns the Bun schema table metadata for the query's model.
	GetTable() *schema.Table
	// Query returns the underlying bun.Query instance.
	Query() bun.Query
	// ExprBuilder returns an expression builder for constructing SQL expressions.
	ExprBuilder() ExprBuilder
	// CreateSubQuery wraps a raw bun.SelectQuery into a VEF SelectQuery for use as a subquery.
	CreateSubQuery(subQuery *bun.SelectQuery) SelectQuery
	// BuildSubQuery creates a bun.SelectQuery by applying the builder function to a new subquery.
	BuildSubQuery(builder func(query SelectQuery)) *bun.SelectQuery
	// BuildCondition creates a WHERE condition clause by applying the builder function.
	BuildCondition(builder func(ConditionBuilder)) interface {
		schema.QueryAppender
		ConditionBuilder
	}
}

// BaseQueryBuilder provides a common implementation for QueryBuilder interface.
// It can be embedded in concrete query types to reduce code duplication.
type BaseQueryBuilder struct {
	db      *BunDB
	dialect schema.Dialect
	query   interface {
		bun.Query
		fmt.Stringer

		NewSelect() *bun.SelectQuery
	}
	eb ExprBuilder
}

// Dialect returns the dialect of the current database connection.
func (b *BaseQueryBuilder) Dialect() schema.Dialect {
	return b.dialect
}

// GetTable returns the table information for the current query.
func (b *BaseQueryBuilder) GetTable() *schema.Table {
	return getTableSchemaFromQuery(b.query)
}

// Query returns the query of the current query instance.
func (b *BaseQueryBuilder) Query() bun.Query {
	return b.query
}

// ExprBuilder returns the expression builder for this query.
func (b *BaseQueryBuilder) ExprBuilder() ExprBuilder {
	return b.eb
}

// CreateSubQuery creates a new subquery from the given bun.SelectQuery.
func (b *BaseQueryBuilder) CreateSubQuery(subQuery *bun.SelectQuery) SelectQuery {
	eb := &QueryExprBuilder{}
	query := &BunSelectQuery{
		QueryBuilder: newQueryBuilder(b.db, b.dialect, subQuery, eb),
		db:           b.db,
		dialect:      b.dialect,
		query:        subQuery,
		eb:           eb,
		isSubQuery:   true,
	}
	eb.qb = query

	return query
}

// BuildSubQuery constructs a subquery using a builder function.
func (b *BaseQueryBuilder) BuildSubQuery(builder func(query SelectQuery)) *bun.SelectQuery {
	subQuery := b.query.NewSelect()
	sq := b.CreateSubQuery(subQuery)
	builder(sq)

	// Apply deferred select state before returning the subquery
	sq.(*BunSelectQuery).applySelectState()

	return subQuery
}

// BuildCondition creates a condition builder for WHERE clauses.
func (b *BaseQueryBuilder) BuildCondition(builder func(ConditionBuilder)) interface {
	schema.QueryAppender
	ConditionBuilder
} {
	cb := newConditionBuilder(b)
	builder(cb)

	return cb
}

// String returns the SQL query string.
func (b *BaseQueryBuilder) String() string {
	return b.query.String()
}

// newQueryBuilder creates a new query builder.
func newQueryBuilder(db *BunDB, dialect schema.Dialect, query interface {
	bun.Query
	fmt.Stringer

	NewSelect() *bun.SelectQuery
}, eb ExprBuilder,
) *BaseQueryBuilder {
	return &BaseQueryBuilder{
		db:      db,
		dialect: dialect,
		query:   query,
		eb:      eb,
	}
}

// ddlQueryAdapter wraps a DDL query that does not implement fmt.Stringer
// so it can be used with BaseQueryBuilder.
type ddlQueryAdapter struct {
	bun.Query

	newSelectFn func() *bun.SelectQuery
}

func (*ddlQueryAdapter) String() string                { return "" }
func (d *ddlQueryAdapter) NewSelect() *bun.SelectQuery { return d.newSelectFn() }

// newDDLQueryBuilder creates a BaseQueryBuilder for DDL queries that lack fmt.Stringer.
func newDDLQueryBuilder(db *BunDB, dialect schema.Dialect, query interface {
	bun.Query

	NewSelect() *bun.SelectQuery
}, eb ExprBuilder,
) *BaseQueryBuilder {
	adapter := &ddlQueryAdapter{
		Query:       query,
		newSelectFn: query.NewSelect,
	}

	return &BaseQueryBuilder{
		db:      db,
		dialect: dialect,
		query:   adapter,
		eb:      eb,
	}
}
