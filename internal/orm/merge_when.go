package orm

import (
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// MergeWhenBuilder is an interface for defining actions in MERGE WHEN clauses.
// It provides methods to specify what action to take when merge conditions are met.
// This interface is returned by WhenMatched, WhenNotMatched, and related methods on MergeQuery.
type MergeWhenBuilder interface {
	// ThenUpdate specifies an UPDATE action for the WHEN clause.
	// Use the provided MergeUpdateBuilder to configure which columns to update.
	ThenUpdate(func(MergeUpdateBuilder)) MergeQuery
	// ThenInsert specifies an INSERT action for the WHEN clause.
	// Use the provided MergeInsertBuilder to configure which columns and values to insert.
	ThenInsert(func(MergeInsertBuilder)) MergeQuery
	// ThenDelete specifies a DELETE action for the WHEN clause.
	ThenDelete() MergeQuery
	// ThenDoNothing specifies no action for the WHEN clause.
	ThenDoNothing() MergeQuery
}

// MergeUpdateBuilder is an interface for configuring UPDATE actions in MERGE queries.
// It allows setting column values and expressions for update operations.
// This builder is used within ThenUpdate() to specify which columns should be updated.
type MergeUpdateBuilder interface {
	// Set sets a single column to a specific value.
	Set(column string, value any) MergeUpdateBuilder
	// SetExpr sets a single column using an expression builder.
	SetExpr(column string, builder func(ExprBuilder) any) MergeUpdateBuilder
	// SetColumns sets multiple columns from the SOURCE table (e.g., column = SOURCE.column).
	// Each column will be updated with its corresponding value from the source.
	SetColumns(columns ...string) MergeUpdateBuilder
	// SetAll sets all columns from the table schema to their corresponding SOURCE values.
	// Columns specified in excludedColumns will be skipped (useful for excluding id, created_at, etc.).
	SetAll(excludedColumns ...string) MergeUpdateBuilder
}

// MergeInsertBuilder is an interface for configuring INSERT actions in MERGE queries.
// It allows specifying which columns and values to insert.
// This builder is used within ThenInsert() to configure the insert operation.
type MergeInsertBuilder interface {
	// Value sets a single column to a specific value for insertion.
	Value(column string, value any) MergeInsertBuilder
	// ValueExpr sets a single column using an expression builder for insertion.
	ValueExpr(column string, builder func(ExprBuilder) any) MergeInsertBuilder
	// Values sets multiple columns from the SOURCE table (e.g., INSERT (col1, col2) VALUES (SOURCE.col1, SOURCE.col2)).
	// Each column will be inserted with its corresponding value from the source.
	Values(columns ...string) MergeInsertBuilder
	// ValuesAll sets all columns from the table schema to their corresponding SOURCE values for insertion.
	// Columns specified in excludedColumns will be skipped (useful for excluding auto-generated columns).
	ValuesAll(excludedColumns ...string) MergeInsertBuilder
}

type mergeWhenBuilder struct {
	parent   *BunMergeQuery
	srcAlias string
	when     string
	cb       func(ConditionBuilder)
}

func newMergeWhenBuilder(query *BunMergeQuery, srcAlias, when string, builder ...func(ConditionBuilder)) *mergeWhenBuilder {
	var cb func(ConditionBuilder)
	if len(builder) > 0 {
		cb = builder[0]
	}

	return &mergeWhenBuilder{
		parent:   query,
		srcAlias: srcAlias,
		when:     when,
		cb:       cb,
	}
}

// buildWhenExpr constructs the WHEN condition expression for the MERGE statement.
// If a condition builder was provided, it combines it with the base WHEN clause.
func (b *mergeWhenBuilder) buildWhenExpr() string {
	if b.cb != nil {
		condition := b.parent.BuildCondition(b.cb)

		whenExpr, err := b.parent.eb.
			Expr("? AND ?", bun.Safe(b.when), condition).
			AppendQuery(b.parent.query.DB().QueryGen(), nil)
		if err != nil {
			panic(fmt.Errorf("merge: failed to build WHEN condition expression: %w", err))
		}

		return string(whenExpr)
	}

	return b.when
}

func (b *mergeWhenBuilder) ThenUpdate(builder func(MergeUpdateBuilder)) MergeQuery {
	b.parent.query.WhenUpdate(b.buildWhenExpr(), func(query *bun.UpdateQuery) *bun.UpdateQuery {
		mub := newMergeUpdateBuilder(getTableSchemaFromQuery(b.parent.query), b.parent.eb, b.srcAlias, query)
		builder(mub)
		mub.apply()

		return query
	})

	return b.parent
}

func (b *mergeWhenBuilder) ThenInsert(builder func(MergeInsertBuilder)) MergeQuery {
	b.parent.query.WhenInsert(b.buildWhenExpr(), func(query *bun.InsertQuery) *bun.InsertQuery {
		mib := newMergeInsertBuilder(getTableSchemaFromQuery(b.parent.query), b.parent.eb, b.srcAlias, query)
		builder(mib)
		mib.apply()

		return query
	})

	return b.parent
}

func (b *mergeWhenBuilder) ThenDelete() MergeQuery {
	b.parent.query.WhenDelete(b.buildWhenExpr())

	return b.parent
}

func (b *mergeWhenBuilder) ThenDoNothing() MergeQuery {
	b.parent.query.When("? THEN DO NOTHING", bun.Safe(b.buildWhenExpr()))

	return b.parent
}

type mergeUpdateBuilder struct {
	table    *schema.Table
	eb       ExprBuilder
	srcAlias string
	query    *bun.UpdateQuery
	setExprs []schema.QueryAppender
}

func newMergeUpdateBuilder(table *schema.Table, eb ExprBuilder, srcAlias string, query *bun.UpdateQuery) *mergeUpdateBuilder {
	return &mergeUpdateBuilder{
		table:    table,
		eb:       eb,
		srcAlias: srcAlias,
		query:    query,
	}
}

func (b *mergeUpdateBuilder) Set(column string, value any) MergeUpdateBuilder {
	setExpr := b.eb.Expr("? = ?", bun.Name(column), value)
	b.setExprs = append(b.setExprs, setExpr)

	return b
}

func (b *mergeUpdateBuilder) SetExpr(column string, builder func(ExprBuilder) any) MergeUpdateBuilder {
	setExpr := b.eb.Expr("? = ?", bun.Name(column), builder(b.eb))
	b.setExprs = append(b.setExprs, setExpr)

	return b
}

func (b *mergeUpdateBuilder) SetColumns(columns ...string) MergeUpdateBuilder {
	for _, column := range columns {
		setExpr := b.eb.Expr("? = ?.?", bun.Name(column), bun.Name(b.srcAlias), bun.Name(column))
		b.setExprs = append(b.setExprs, setExpr)
	}

	return b
}

func (b *mergeUpdateBuilder) SetAll(excludedColumns ...string) MergeUpdateBuilder {
	if b.table == nil {
		panic("merge: SetAll() requires a table schema - call Model() before SetAll()")
	}

	excluded := toStringSet(excludedColumns)
	for _, field := range b.table.Fields {
		if !excluded[field.Name] {
			setExpr := b.eb.Expr("? = ?.?", bun.Name(field.Name), bun.Name(b.srcAlias), bun.Name(field.Name))
			b.setExprs = append(b.setExprs, setExpr)
		}
	}

	return b
}

// apply adds all accumulated SET expressions to the underlying bun update query.
func (b *mergeUpdateBuilder) apply() {
	for _, setExpr := range b.setExprs {
		b.query.Set("?", setExpr)
	}
}

// columnValuePair represents a column-value mapping for MERGE INSERT operations.
// It stores the column name and its corresponding value or expression.
type columnValuePair struct {
	column string
	value  any
}

// mergeInsertBuilder implements MergeInsertBuilder interface.
type mergeInsertBuilder struct {
	table    *schema.Table
	eb       ExprBuilder
	srcAlias string
	query    *bun.InsertQuery
	values   []columnValuePair
}

// newMergeInsertBuilder creates a new MergeInsertBuilder instance.
func newMergeInsertBuilder(table *schema.Table, eb ExprBuilder, srcAlias string, query *bun.InsertQuery) *mergeInsertBuilder {
	return &mergeInsertBuilder{
		table:    table,
		eb:       eb,
		srcAlias: srcAlias,
		query:    query,
	}
}

func (b *mergeInsertBuilder) Value(column string, value any) MergeInsertBuilder {
	b.values = append(b.values, columnValuePair{
		column: column,
		value:  value,
	})

	return b
}

func (b *mergeInsertBuilder) ValueExpr(column string, builder func(ExprBuilder) any) MergeInsertBuilder {
	b.values = append(b.values, columnValuePair{
		column: column,
		value:  builder(b.eb),
	})

	return b
}

func (b *mergeInsertBuilder) Values(columns ...string) MergeInsertBuilder {
	for _, column := range columns {
		valueExpr := b.eb.Expr("?.?", bun.Name(b.srcAlias), bun.Name(column))
		b.values = append(b.values, columnValuePair{
			column: column,
			value:  valueExpr,
		})
	}

	return b
}

func (b *mergeInsertBuilder) ValuesAll(excludedColumns ...string) MergeInsertBuilder {
	if b.table == nil {
		panic("merge: ValuesAll() requires a table schema - call Model() before ValuesAll()")
	}

	excluded := toStringSet(excludedColumns)
	for _, field := range b.table.Fields {
		if !excluded[field.Name] {
			valueExpr := b.eb.Expr("?.?", bun.Name(b.srcAlias), bun.Name(field.Name))
			b.values = append(b.values, columnValuePair{
				column: field.Name,
				value:  valueExpr,
			})
		}
	}

	return b
}

// apply adds all accumulated column-value pairs to the underlying bun insert query.
func (b *mergeInsertBuilder) apply() {
	for _, value := range b.values {
		b.query.Value(value.column, "?", value.value)
	}
}

// toStringSet converts a string slice to a map for O(1) lookups.
func toStringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}

	return set
}
