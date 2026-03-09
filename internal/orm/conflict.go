package orm

import (
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// ConflictBuilder is used to configure INSERT conflict handling (UPSERT) target in a dialect-aware way.
// This is the first stage that defines the conflict target (columns, constraints, conditions).
type ConflictBuilder interface {
	// Columns specifies the conflict target columns (used with DO UPDATE).
	Columns(columns ...string) ConflictBuilder
	// Constraint specifies a named constraint as the conflict target.
	Constraint(name string) ConflictBuilder
	// Where adds a condition to the conflict target (partial unique index filter).
	Where(func(ConditionBuilder)) ConflictBuilder

	// DoNothing performs DO NOTHING on conflict and finalizes the conflict handling.
	DoNothing()
	// DoUpdate performs DO UPDATE on conflict and returns a builder for update operations.
	DoUpdate() ConflictUpdateBuilder
}

// ConflictUpdateBuilder is used to configure the UPDATE part of conflict handling.
// This is the second stage that defines what to update when conflicts occur.
type ConflictUpdateBuilder interface {
	// Set adds an assignment in DO UPDATE clause. If no value provided, uses excluded/VALUES value when supported.
	Set(column string, value ...any) ConflictUpdateBuilder
	// SetExpr adds an expression assignment in DO UPDATE clause.
	SetExpr(column string, builder func(ExprBuilder) any) ConflictUpdateBuilder
	// Where adds a predicate to DO UPDATE (PostgreSQL/SQLite). Ignored on MySQL.
	Where(func(ConditionBuilder)) ConflictUpdateBuilder
}

type InsertQueryConflictBuilder struct {
	qb QueryBuilder
	eb ExprBuilder
	// target
	columns     []string
	constraint  string
	targetWhere schema.QueryAppender
	// action
	action ConflictAction
	// updates
	sets        []schema.QueryAppender
	updateWhere schema.QueryAppender
}

func newConflictBuilder(qb QueryBuilder) *InsertQueryConflictBuilder {
	return &InsertQueryConflictBuilder{
		qb: qb,
		eb: qb.ExprBuilder(),
	}
}

func (b *InsertQueryConflictBuilder) Columns(columns ...string) ConflictBuilder {
	b.columns = append(b.columns, columns...)

	return b
}

func (b *InsertQueryConflictBuilder) Constraint(name string) ConflictBuilder {
	b.constraint = name

	return b
}

func (b *InsertQueryConflictBuilder) Where(builder func(ConditionBuilder)) ConflictBuilder {
	b.targetWhere = b.qb.BuildCondition(builder)

	return b
}

func (b *InsertQueryConflictBuilder) DoNothing() {
	b.action = ConflictDoNothing
}

func (b *InsertQueryConflictBuilder) DoUpdate() ConflictUpdateBuilder {
	b.action = ConflictDoUpdate

	return &InsertQueryConflictUpdateBuilder{parent: b}
}

// InsertQueryConflictUpdateBuilder implements ConflictUpdateBuilder interface.
type InsertQueryConflictUpdateBuilder struct {
	parent *InsertQueryConflictBuilder
}

func (b *InsertQueryConflictUpdateBuilder) Set(column string, value ...any) ConflictUpdateBuilder {
	eb := b.parent.eb
	col := bun.Name(column)

	var valueExpr any
	if len(value) > 0 {
		valueExpr = value[0]
	} else {
		// MySQL uses column name reference; PostgreSQL/SQLite use EXCLUDED.column
		eb.ExecByDialect(DialectExecs{
			MySQL:   func() { valueExpr = col },
			Default: func() { valueExpr = eb.Expr("EXCLUDED.?", col) },
		})
	}

	b.parent.sets = append(b.parent.sets, eb.Expr("? = ?", col, valueExpr))

	return b
}

func (b *InsertQueryConflictUpdateBuilder) SetExpr(column string, builder func(ExprBuilder) any) ConflictUpdateBuilder {
	valueExpr := builder(b.parent.eb)
	setExpr := b.parent.eb.Expr("? = ?", bun.Name(column), valueExpr)
	b.parent.sets = append(b.parent.sets, setExpr)

	return b
}

func (b *InsertQueryConflictUpdateBuilder) Where(builder func(ConditionBuilder)) ConflictUpdateBuilder {
	b.parent.updateWhere = b.parent.qb.BuildCondition(builder)

	return b
}

// build applies the configured conflict handling to the underlying bun.InsertQuery.
func (b *InsertQueryConflictBuilder) build(query *bun.InsertQuery) {
	b.eb.ExecByDialect(DialectExecs{
		MySQL:   func() { b.buildMySQL(query) },
		Default: func() { b.buildPostgresSQLite(query) },
	})
}

func (b *InsertQueryConflictBuilder) buildMySQL(query *bun.InsertQuery) {
	if b.action == ConflictDoNothing {
		query.Ignore()

		return
	}

	query.On("DUPLICATE KEY UPDATE")

	for _, set := range b.sets {
		query.Set("?", set)
	}
}

func (b *InsertQueryConflictBuilder) buildPostgresSQLite(query *bun.InsertQuery) {
	var target schema.QueryAppender
	if b.constraint != "" {
		target = b.eb.Expr("CONSTRAINT ?", bun.Name(b.constraint))
	} else if len(b.columns) > 0 {
		target = b.eb.Expr("(?)", Names(b.columns...))
	}

	var sb strings.Builder
	sb.WriteString("CONFLICT ")

	if target == nil {
		// PostgreSQL requires a conflict target for DO UPDATE; fallback to DO NOTHING.
		sb.WriteString(ConflictDoNothing.String())
		query.On(sb.String())

		return
	}

	sb.WriteString("? ")

	args := []any{target}

	if b.targetWhere != nil {
		sb.WriteString("WHERE ? ")

		args = append(args, b.targetWhere)
	}

	sb.WriteString(b.action.String())
	query.On(sb.String(), args...)

	for _, set := range b.sets {
		query.Set("?", set)
	}

	if b.updateWhere != nil {
		query.Where("?", b.updateWhere)
	}
}
