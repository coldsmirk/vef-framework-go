package orm

import "github.com/coldsmirk/vef-framework-go/page"

// SelectQuery builds and executes SELECT queries with support for
// joins, conditions, ordering, grouping, pagination, locking, and set operations.
type SelectQuery interface {
	QueryBuilder
	SelectQueryExecutor
	DBAccessor
	CTE[SelectQuery]
	Selectable[SelectQuery]
	TableSource[SelectQuery]
	JoinOperations[SelectQuery]
	Filterable[SelectQuery]
	Orderable[SelectQuery]
	Limitable[SelectQuery]
	Applier[SelectQuery]

	// SelectAs adds a column with an alias to the SELECT clause.
	SelectAs(column, alias string) SelectQuery
	// SelectModelColumns selects the model's columns. This is the default if no select methods are called.
	SelectModelColumns() SelectQuery
	// SelectModelPKs selects only the model's primary key columns.
	SelectModelPKs() SelectQuery
	// SelectExpr adds a SQL expression to the SELECT clause with an optional alias.
	SelectExpr(builder func(ExprBuilder) any, alias ...string) SelectQuery
	// Distinct adds DISTINCT to the SELECT clause to eliminate duplicate rows.
	Distinct() SelectQuery
	// DistinctOnColumns adds DISTINCT ON for specific columns (PostgreSQL only).
	DistinctOnColumns(columns ...string) SelectQuery
	// DistinctOnExpr adds DISTINCT ON using a SQL expression (PostgreSQL only).
	DistinctOnExpr(builder func(ExprBuilder) any) SelectQuery
	// JoinRelations applies RelationSpec configurations for declarative JOIN operations.
	JoinRelations(specs ...*RelationSpec) SelectQuery
	// Relation adds a Bun relation by name with optional query customization.
	Relation(name string, apply ...func(query SelectQuery)) SelectQuery
	// GroupBy adds GROUP BY clauses for the specified columns.
	GroupBy(columns ...string) SelectQuery
	// GroupByExpr adds a GROUP BY clause using a SQL expression.
	GroupByExpr(func(ExprBuilder) any) SelectQuery
	// Having adds a HAVING condition for filtering grouped results.
	Having(func(ConditionBuilder)) SelectQuery
	// Offset sets the number of rows to skip before returning results.
	Offset(offset int) SelectQuery
	// Paginate applies LIMIT and OFFSET from a Pageable (page number + page size).
	Paginate(pageable page.Pageable) SelectQuery
	// ForShare acquires a shared lock on the selected rows (allows concurrent reads).
	ForShare(tables ...any) SelectQuery
	// ForShareNoWait acquires a shared lock, returning an error immediately if the rows are locked.
	ForShareNoWait(tables ...any) SelectQuery
	// ForShareSkipLocked acquires a shared lock, skipping rows that are currently locked.
	ForShareSkipLocked(tables ...any) SelectQuery
	// ForKeyShare acquires a key-share lock that permits concurrent SELECT FOR SHARE but blocks modifications.
	ForKeyShare(tables ...any) SelectQuery
	// ForKeyShareNoWait acquires a key-share lock, returning an error immediately if the rows are locked.
	ForKeyShareNoWait(tables ...any) SelectQuery
	// ForKeyShareSkipLocked acquires a key-share lock, skipping rows that are currently locked.
	ForKeyShareSkipLocked(tables ...any) SelectQuery
	// ForUpdate acquires an exclusive lock on the selected rows (blocks reads and writes).
	ForUpdate(tables ...any) SelectQuery
	// ForUpdateNoWait acquires an exclusive lock, returning an error immediately if the rows are locked.
	ForUpdateNoWait(tables ...any) SelectQuery
	// ForUpdateSkipLocked acquires an exclusive lock, skipping rows that are currently locked.
	ForUpdateSkipLocked(tables ...any) SelectQuery
	// ForNoKeyUpdate acquires a lock that blocks other FOR UPDATE but allows FOR KEY SHARE.
	ForNoKeyUpdate(tables ...any) SelectQuery
	// ForNoKeyUpdateNoWait acquires a no-key-update lock, returning an error immediately if the rows are locked.
	ForNoKeyUpdateNoWait(tables ...any) SelectQuery
	// ForNoKeyUpdateSkipLocked acquires a no-key-update lock, skipping rows that are currently locked.
	ForNoKeyUpdateSkipLocked(tables ...any) SelectQuery
	// Union combines results with another SELECT, eliminating duplicates.
	Union(func(query SelectQuery)) SelectQuery
	// UnionAll combines results with another SELECT, keeping all duplicates.
	UnionAll(func(query SelectQuery)) SelectQuery
	// Intersect returns only rows present in both SELECT results, eliminating duplicates.
	Intersect(func(query SelectQuery)) SelectQuery
	// IntersectAll returns only rows present in both SELECT results, keeping duplicates.
	IntersectAll(func(query SelectQuery)) SelectQuery
	// Except returns rows from this SELECT that are not in the other, eliminating duplicates.
	Except(func(query SelectQuery)) SelectQuery
	// ExceptAll returns rows from this SELECT that are not in the other, keeping duplicates.
	ExceptAll(func(query SelectQuery)) SelectQuery
}

// RawQuery executes raw SQL queries with parameter binding.
type RawQuery interface {
	QueryExecutor
}

// InsertQuery builds and executes INSERT queries with conflict resolution and RETURNING support.
type InsertQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[InsertQuery]
	TableSource[InsertQuery]
	Selectable[InsertQuery]
	ColumnUpdatable[InsertQuery]
	Returnable[InsertQuery]
	Applier[InsertQuery]

	// OnConflict configures conflict handling (UPSERT).
	OnConflict(func(ConflictBuilder)) InsertQuery
}

// UpdateQuery builds and executes UPDATE queries.
// Does not inherit JoinOperations; use From methods for PostgreSQL-style FROM clause.
type UpdateQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[UpdateQuery]
	TableSource[UpdateQuery]
	Selectable[UpdateQuery]
	Filterable[UpdateQuery]
	Orderable[UpdateQuery]
	Limitable[UpdateQuery]
	ColumnUpdatable[UpdateQuery]
	Returnable[UpdateQuery]
	Applier[UpdateQuery]

	// Set is an alias for Column.
	Set(name string, value any) UpdateQuery
	// SetExpr is an alias for ColumnExpr.
	SetExpr(name string, builder func(ExprBuilder) any) UpdateQuery
	// OmitZero skips zero-value fields when building SET clauses from the model.
	OmitZero() UpdateQuery
	// Bulk enables bulk update mode, generating a single query to update multiple rows via CTE.
	Bulk() UpdateQuery
}

// DeleteQuery builds and executes DELETE queries with soft delete support.
type DeleteQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[DeleteQuery]
	TableSource[DeleteQuery]
	Filterable[DeleteQuery]
	Orderable[DeleteQuery]
	Limitable[DeleteQuery]
	Returnable[DeleteQuery]
	Applier[DeleteQuery]

	// ForceDelete bypasses soft delete and performs a hard delete.
	ForceDelete() DeleteQuery
}

// MergeQuery builds and executes MERGE (UPSERT) queries with conditional match/no-match actions.
type MergeQuery interface {
	QueryBuilder
	QueryExecutor
	DBAccessor
	CTE[MergeQuery]
	TableSource[MergeQuery]
	Returnable[MergeQuery]
	Applier[MergeQuery]

	// Using specifies the source table for the merge using a model.
	Using(model any, alias ...string) MergeQuery
	// UsingTable specifies the source table for the merge by raw name.
	UsingTable(table string, alias ...string) MergeQuery
	// UsingExpr specifies the source for the merge using a SQL expression.
	UsingExpr(builder func(ExprBuilder) any, alias ...string) MergeQuery
	// UsingSubQuery specifies the source for the merge using a subquery.
	UsingSubQuery(builder func(SelectQuery), alias ...string) MergeQuery

	// On specifies the merge condition that determines matches between target and source.
	On(func(ConditionBuilder)) MergeQuery

	// WhenMatched configures actions for rows that match the ON condition.
	WhenMatched(builder ...func(ConditionBuilder)) MergeWhenBuilder
	// WhenNotMatched configures actions for source rows with no target match.
	WhenNotMatched(builder ...func(ConditionBuilder)) MergeWhenBuilder
	// WhenNotMatchedByTarget is an alias for WhenNotMatched (SQL Server syntax).
	WhenNotMatchedByTarget(builder ...func(ConditionBuilder)) MergeWhenBuilder
	// WhenNotMatchedBySource configures actions for target rows with no source match.
	WhenNotMatchedBySource(builder ...func(ConditionBuilder)) MergeWhenBuilder
}
