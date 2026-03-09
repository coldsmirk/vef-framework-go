package orm

import (
	"context"
	"database/sql"
)

// DBAccessor provides access to the underlying DB instance.
type DBAccessor interface {
	// DB returns the DB that created this query.
	DB() DB
}

// Executor defines the Exec method shared by all DML and DDL queries.
type Executor interface {
	// Exec executes the query and optionally scans the result into dest.
	Exec(ctx context.Context, dest ...any) (sql.Result, error)
}

// QueryExecutor extends Executor with Scan for reading result rows.
type QueryExecutor interface {
	Executor

	// Scan executes the query and scans result rows into dest.
	Scan(ctx context.Context, dest ...any) error
}

// SelectQueryExecutor extends QueryExecutor with SELECT-specific operations.
type SelectQueryExecutor interface {
	QueryExecutor

	// Rows executes the query and returns the raw sql.Rows iterator.
	Rows(ctx context.Context) (*sql.Rows, error)
	// ScanAndCount executes the query, scans result rows into dest, and returns the total count (ignoring LIMIT/OFFSET).
	ScanAndCount(ctx context.Context, dest ...any) (int64, error)
	// Count executes a COUNT(*) query, ignoring any column selections.
	Count(ctx context.Context) (int64, error)
	// Exists returns true if the query yields at least one row.
	Exists(ctx context.Context) (bool, error)
}

// CTE defines methods for creating Common Table Expressions.
type CTE[T Executor] interface {
	// With adds a named CTE built from a SELECT subquery.
	With(name string, builder func(query SelectQuery)) T
	// WithValues adds a named CTE from a model's values (useful for bulk operations).
	WithValues(name string, model any, withOrder ...bool) T
	// WithRecursive adds a recursive CTE built from a SELECT subquery.
	WithRecursive(name string, builder func(query SelectQuery)) T
}

// Selectable defines column selection methods (include/exclude).
type Selectable[T Executor] interface {
	// SelectAll selects all columns (SELECT *).
	SelectAll() T
	// Select adds specific columns to the SELECT clause with automatic table alias resolution.
	Select(columns ...string) T
	// Exclude removes specific columns from the model's default column set.
	Exclude(columns ...string) T
	// ExcludeAll removes all model columns from the SELECT clause, useful when only selecting expressions.
	ExcludeAll() T
}

// TableSource defines methods for specifying table sources.
// Supports model-based and raw table references with optional aliases.
type TableSource[T Executor] interface {
	// Model sets the primary table with automatic name and alias resolution from model structure.
	Model(model any) T
	// ModelTable overrides the table name and alias resolved by Model. Must be called after Model.
	ModelTable(name string, alias ...string) T
	// Table sets the table by raw name with optional alias.
	Table(name string, alias ...string) T
	// TableFrom auto-resolves table name and alias from model. The alias parameter takes precedence.
	TableFrom(model any, alias ...string) T
	// TableExpr sets the table source to a SQL expression with optional alias.
	TableExpr(builder func(ExprBuilder) any, alias ...string) T
	// TableSubQuery sets the table source to a subquery with optional alias.
	TableSubQuery(builder func(query SelectQuery), alias ...string) T
}

// JoinOperations defines all standard SQL join types (INNER, LEFT, RIGHT, FULL, CROSS)
// with model, table name, subquery, and expression source variants.
type JoinOperations[T any] interface {
	// Join performs an INNER JOIN using a model as the table source.
	Join(model any, builder func(ConditionBuilder), alias ...string) T
	// JoinTable performs an INNER JOIN using a raw table name.
	JoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	// JoinSubQuery performs an INNER JOIN using a subquery as the table source.
	JoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	// JoinExpr performs an INNER JOIN using a SQL expression as the table source.
	JoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	// LeftJoin performs a LEFT OUTER JOIN using a model as the table source.
	LeftJoin(model any, builder func(ConditionBuilder), alias ...string) T
	// LeftJoinTable performs a LEFT OUTER JOIN using a raw table name.
	LeftJoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	// LeftJoinSubQuery performs a LEFT OUTER JOIN using a subquery as the table source.
	LeftJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	// LeftJoinExpr performs a LEFT OUTER JOIN using a SQL expression as the table source.
	LeftJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	// RightJoin performs a RIGHT OUTER JOIN using a model as the table source.
	RightJoin(model any, builder func(ConditionBuilder), alias ...string) T
	// RightJoinTable performs a RIGHT OUTER JOIN using a raw table name.
	RightJoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	// RightJoinSubQuery performs a RIGHT OUTER JOIN using a subquery as the table source.
	RightJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	// RightJoinExpr performs a RIGHT OUTER JOIN using a SQL expression as the table source.
	RightJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	// FullJoin performs a FULL OUTER JOIN using a model as the table source.
	FullJoin(model any, builder func(ConditionBuilder), alias ...string) T
	// FullJoinTable performs a FULL OUTER JOIN using a raw table name.
	FullJoinTable(name string, builder func(ConditionBuilder), alias ...string) T
	// FullJoinSubQuery performs a FULL OUTER JOIN using a subquery as the table source.
	FullJoinSubQuery(sqBuilder func(query SelectQuery), cBuilder func(ConditionBuilder), alias ...string) T
	// FullJoinExpr performs a FULL OUTER JOIN using a SQL expression as the table source.
	FullJoinExpr(eBuilder func(ExprBuilder) any, cBuilder func(ConditionBuilder), alias ...string) T

	// CrossJoin performs a CROSS JOIN using a model as the table source (no join condition).
	CrossJoin(model any, alias ...string) T
	// CrossJoinTable performs a CROSS JOIN using a raw table name.
	CrossJoinTable(name string, alias ...string) T
	// CrossJoinSubQuery performs a CROSS JOIN using a subquery as the table source.
	CrossJoinSubQuery(sqBuilder func(query SelectQuery), alias ...string) T
	// CrossJoinExpr performs a CROSS JOIN using a SQL expression as the table source.
	CrossJoinExpr(eBuilder func(ExprBuilder) any, alias ...string) T
}

// Filterable defines WHERE clause methods including soft delete support.
type Filterable[T Executor] interface {
	// Where adds WHERE conditions using the ConditionBuilder DSL.
	Where(func(ConditionBuilder)) T
	// WherePK adds a WHERE clause matching the model's primary key columns.
	WherePK(columns ...string) T
	// WhereDeleted restricts results to soft-deleted rows only.
	WhereDeleted() T
	// IncludeDeleted disables the automatic soft delete filter to include deleted rows.
	IncludeDeleted() T
}

// Orderable defines ORDER BY methods for query results.
type Orderable[T Executor] interface {
	// OrderBy adds ascending ORDER BY clauses for the specified columns.
	OrderBy(columns ...string) T
	// OrderByDesc adds descending ORDER BY clauses for the specified columns.
	OrderByDesc(columns ...string) T
	// OrderByExpr adds an ORDER BY clause using a custom expression with full control over direction and nulls ordering.
	OrderByExpr(func(ExprBuilder) any) T
}

// Limitable defines the LIMIT clause for result set size control.
type Limitable[T Executor] interface {
	// Limit restricts the maximum number of rows returned.
	Limit(limit int) T
}

// ColumnUpdatable defines methods for setting column values in queries.
type ColumnUpdatable[T Executor] interface {
	// Column sets a column to a literal value.
	Column(name string, value any) T
	// ColumnExpr sets a column to a SQL expression.
	ColumnExpr(name string, builder func(ExprBuilder) any) T
}

// Returnable defines RETURNING clause methods for INSERT, UPDATE, and DELETE queries.
type Returnable[T Executor] interface {
	// Returning adds specific columns to the RETURNING clause.
	Returning(columns ...string) T
	// ReturningAll adds all columns to the RETURNING clause (RETURNING *).
	ReturningAll() T
	// ReturningNone removes all RETURNING clauses, suppressing any auto-generated ones.
	ReturningNone() T
}

// ApplyFunc is a reusable query modification function.
type ApplyFunc[T any] func(T)

// Applier enables applying reusable query modifications, optionally conditioned on a boolean.
type Applier[T any] interface {
	// Apply executes the given modification functions on the query.
	Apply(fns ...ApplyFunc[T]) T
	// ApplyIf executes the modification functions only when condition is true.
	ApplyIf(condition bool, fns ...ApplyFunc[T]) T
}

// TableTarget defines table targeting methods for DDL queries.
type TableTarget[T Executor] interface {
	// Model sets the target table from a model struct with automatic name resolution.
	Model(model any) T
	// Table sets the target table by raw name(s).
	Table(tables ...string) T
}
