package orm

import "time"

// AuditConditionBuilder is a builder for audit conditions.
type AuditConditionBuilder interface {
	// CreatedByEquals is a condition that checks if the created by column is equal to a value.
	CreatedByEquals(createdBy string, alias ...string) ConditionBuilder
	// OrCreatedByEquals is an OR condition that checks if the created by column is equal to a value.
	OrCreatedByEquals(createdBy string, alias ...string) ConditionBuilder
	// CreatedByEqualsSubQuery is a condition that checks if the created by column is equal to a subquery.
	CreatedByEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByEqualsSubQuery is a condition that checks if the created by column is equal to a subquery.
	OrCreatedByEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByEqualsAny is a condition that checks if the created by column is equal to any value returned by a subquery.
	CreatedByEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByEqualsAny is a condition that checks if the created by column is equal to any value returned by a subquery.
	OrCreatedByEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByEqualsAll is a condition that checks if the created by column is equal to all values returned by a subquery.
	CreatedByEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByEqualsAll is a condition that checks if the created by column is equal to all values returned by a subquery.
	OrCreatedByEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByEqualsCurrent is a condition that checks if the created by column is equal to the current user.
	CreatedByEqualsCurrent(alias ...string) ConditionBuilder
	// OrCreatedByEqualsCurrent is a condition that checks if the created by column is equal to the current user.
	OrCreatedByEqualsCurrent(alias ...string) ConditionBuilder
	// CreatedByNotEquals is a condition that checks if the created by column is not equal to a value.
	CreatedByNotEquals(createdBy string, alias ...string) ConditionBuilder
	// OrCreatedByNotEquals is a condition that checks if the created by column is not equal to a value.
	OrCreatedByNotEquals(createdBy string, alias ...string) ConditionBuilder
	// CreatedByNotEqualsSubQuery is a condition that checks if the created by column is not equal to a subquery.
	CreatedByNotEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByNotEqualsSubQuery is a condition that checks if the created by column is not equal to a subquery.
	OrCreatedByNotEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByNotEqualsAny is a condition that checks if the created by column is not equal to any value returned by a subquery.
	CreatedByNotEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByNotEqualsAny is a condition that checks if the created by column is not equal to any value returned by a subquery.
	OrCreatedByNotEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByNotEqualsAll is a condition that checks if the created by column is not equal to all values returned by a subquery.
	CreatedByNotEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByNotEqualsAll is a condition that checks if the created by column is not equal to all values returned by a subquery.
	OrCreatedByNotEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByNotEqualsCurrent is a condition that checks if the created by column is not equal to the current user.
	CreatedByNotEqualsCurrent(alias ...string) ConditionBuilder
	// OrCreatedByNotEqualsCurrent is a condition that checks if the created by column is not equal to the current user.
	OrCreatedByNotEqualsCurrent(alias ...string) ConditionBuilder
	// CreatedByIn is a condition that checks if the created by column is in a list of values.
	CreatedByIn(createdBys []string, alias ...string) ConditionBuilder
	// OrCreatedByIn is a condition that checks if the created by column is in a list of values.
	OrCreatedByIn(createdBys []string, alias ...string) ConditionBuilder
	// CreatedByInSubQuery is a condition that checks if the created by column is in a subquery.
	CreatedByInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByInSubQuery is an OR condition that checks if the created by column is in a subquery.
	OrCreatedByInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedByNotIn is a condition that checks if the created by column is not in a list of values.
	CreatedByNotIn(createdBys []string, alias ...string) ConditionBuilder
	// OrCreatedByNotIn is a condition that checks if the created by column is not in a list of values.
	OrCreatedByNotIn(createdBys []string, alias ...string) ConditionBuilder
	// CreatedByNotInSubQuery is a condition that checks if the created by column is not in a subquery.
	CreatedByNotInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrCreatedByNotInSubQuery is a condition that checks if the created by column is not in a subquery.
	OrCreatedByNotInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByEquals is a condition that checks if the updated by column is equal to a value.
	UpdatedByEquals(updatedBy string, alias ...string) ConditionBuilder
	// OrUpdatedByEquals is a condition that checks if the updated by column is equal to a value.
	OrUpdatedByEquals(updatedBy string, alias ...string) ConditionBuilder
	// UpdatedByEqualsSubQuery is a condition that checks if the updated by column is equal to a subquery.
	UpdatedByEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByEqualsSubQuery is a condition that checks if the updated by column is equal to a subquery.
	OrUpdatedByEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByEqualsAny is a condition that checks if the updated by column is equal to any value returned by a subquery.
	UpdatedByEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByEqualsAny is a condition that checks if the updated by column is equal to any value returned by a subquery.
	OrUpdatedByEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByEqualsAll is a condition that checks if the updated by column is equal to all values returned by a subquery.
	UpdatedByEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByEqualsAll is a condition that checks if the updated by column is equal to all values returned by a subquery.
	OrUpdatedByEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByEqualsCurrent is a condition that checks if the updated by column is equal to the current user.
	UpdatedByEqualsCurrent(alias ...string) ConditionBuilder
	// OrUpdatedByEqualsCurrent is a condition that checks if the updated by column is equal to the current user.
	OrUpdatedByEqualsCurrent(alias ...string) ConditionBuilder
	// UpdatedByNotEquals is a condition that checks if the updated by column is not equal to a value.
	UpdatedByNotEquals(updatedBy string, alias ...string) ConditionBuilder
	// OrUpdatedByNotEquals is an OR condition that checks if the updated by column is not equal to a value.
	OrUpdatedByNotEquals(updatedBy string, alias ...string) ConditionBuilder
	// UpdatedByNotEqualsSubQuery is a condition that checks if the updated by column is not equal to a subquery.
	UpdatedByNotEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByNotEqualsSubQuery is a condition that checks if the updated by column is not equal to a subquery.
	OrUpdatedByNotEqualsSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByNotEqualsAny is a condition that checks if the updated by column is not equal to any value returned by a subquery.
	UpdatedByNotEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByNotEqualsAny is a condition that checks if the updated by column is not equal to any value returned by a subquery.
	OrUpdatedByNotEqualsAny(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByNotEqualsAll is a condition that checks if the updated by column is not equal to all values returned by a subquery.
	UpdatedByNotEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByNotEqualsAll is a condition that checks if the updated by column is not equal to all values returned by a subquery.
	OrUpdatedByNotEqualsAll(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByNotEqualsCurrent is a condition that checks if the updated by column is not equal to the current user.
	UpdatedByNotEqualsCurrent(alias ...string) ConditionBuilder
	// OrUpdatedByNotEqualsCurrent is a condition that checks if the updated by column is not equal to the current user.
	OrUpdatedByNotEqualsCurrent(alias ...string) ConditionBuilder
	// UpdatedByIn is a condition that checks if the updated by column is in a list of values.
	UpdatedByIn(updatedBys []string, alias ...string) ConditionBuilder
	// OrUpdatedByIn is an OR condition that checks if the updated by column is in a list of values.
	OrUpdatedByIn(updatedBys []string, alias ...string) ConditionBuilder
	// UpdatedByInSubQuery is a condition that checks if the updated by column is in a subquery.
	UpdatedByInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByInSubQuery is a condition that checks if the updated by column is in a subquery.
	OrUpdatedByInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// UpdatedByNotIn is a condition that checks if the updated by column is not in a list of values.
	UpdatedByNotIn(updatedBys []string, alias ...string) ConditionBuilder
	// OrUpdatedByNotIn is a condition that checks if the updated by column is not in a list of values.
	OrUpdatedByNotIn(updatedBys []string, alias ...string) ConditionBuilder
	// UpdatedByNotInSubQuery is a condition that checks if the updated by column is not in a subquery.
	UpdatedByNotInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// OrUpdatedByNotInSubQuery is a condition that checks if the updated by column is not in a subquery.
	OrUpdatedByNotInSubQuery(builder func(query SelectQuery), alias ...string) ConditionBuilder
	// CreatedAtGreaterThan is a condition that checks if the created at column is greater than a value.
	CreatedAtGreaterThan(createdAt time.Time, alias ...string) ConditionBuilder
	// OrCreatedAtGreaterThan is a condition that checks if the created at column is greater than a value.
	OrCreatedAtGreaterThan(createdAt time.Time, alias ...string) ConditionBuilder
	// CreatedAtGreaterThanOrEqual is a condition that checks if the created at column is greater than or equal to a value.
	CreatedAtGreaterThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder
	// OrCreatedAtGreaterThanOrEqual is a condition that checks if the created at column is greater than or equal to a value.
	OrCreatedAtGreaterThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder
	// CreatedAtLessThan is a condition that checks if the created at column is less than a value.
	CreatedAtLessThan(createdAt time.Time, alias ...string) ConditionBuilder
	// OrCreatedAtLessThan is a condition that checks if the created at column is less than a value.
	OrCreatedAtLessThan(createdAt time.Time, alias ...string) ConditionBuilder
	// CreatedAtLessThanOrEqual is a condition that checks if the created at column is less than or equal to a value.
	CreatedAtLessThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder
	// OrCreatedAtLessThanOrEqual is a condition that checks if the created at column is less than or equal to a value.
	OrCreatedAtLessThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder
	// CreatedAtBetween is a condition that checks if the created at column is between two values.
	CreatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder
	// OrCreatedAtBetween is a condition that checks if the created at column is between two values.
	OrCreatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder
	// CreatedAtNotBetween is a condition that checks if the created at column is not between two values.
	CreatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder
	// OrCreatedAtNotBetween is a condition that checks if the created at column is not between two values.
	OrCreatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder
	// UpdatedAtGreaterThan is a condition that checks if the updated at column is greater than a value.
	UpdatedAtGreaterThan(updatedAt time.Time, alias ...string) ConditionBuilder
	// OrUpdatedAtGreaterThan is a condition that checks if the updated at column is greater than a value.
	OrUpdatedAtGreaterThan(updatedAt time.Time, alias ...string) ConditionBuilder
	// UpdatedAtGreaterThanOrEqual is a condition that checks if the updated at column is greater than or equal to a value.
	UpdatedAtGreaterThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder
	// OrUpdatedAtGreaterThanOrEqual is a condition that checks if the updated at column is greater than or equal to a value.
	OrUpdatedAtGreaterThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder
	// UpdatedAtLessThan is a condition that checks if the updated at column is less than a value.
	UpdatedAtLessThan(updatedAt time.Time, alias ...string) ConditionBuilder
	// OrUpdatedAtLessThan is a condition that checks if the updated at column is less than a value.
	OrUpdatedAtLessThan(updatedAt time.Time, alias ...string) ConditionBuilder
	// UpdatedAtLessThanOrEqual is a condition that checks if the updated at column is less than or equal to a value.
	UpdatedAtLessThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder
	// OrUpdatedAtLessThanOrEqual is a condition that checks if the updated at column is less than or equal to a value.
	OrUpdatedAtLessThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder
	// UpdatedAtBetween is a condition that checks if the updated at column is between two values.
	UpdatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder
	// OrUpdatedAtBetween is a condition that checks if the updated at column is between two values.
	OrUpdatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder
	// UpdatedAtNotBetween is a condition that checks if the updated at column is not between two values.
	UpdatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder
	// OrUpdatedAtNotBetween is a condition that checks if the updated at column is not between two values.
	OrUpdatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder
}

// PKConditionBuilder is a builder for primary key conditions.
type PKConditionBuilder interface {
	// PKEquals is a condition that checks if the primary key is equal to a value.
	PKEquals(pk any, alias ...string) ConditionBuilder
	// OrPKEquals is a condition that checks if the primary key is equal to a value.
	OrPKEquals(pk any, alias ...string) ConditionBuilder
	// PKNotEquals is a condition that checks if the primary key is not equal to a value.
	PKNotEquals(pk any, alias ...string) ConditionBuilder
	// OrPKNotEquals is a condition that checks if the primary key is not equal to a value.
	OrPKNotEquals(pk any, alias ...string) ConditionBuilder
	// PKIn is a condition that checks if the primary key is in a list of values.
	PKIn(pks any, alias ...string) ConditionBuilder
	// OrPKIn is a condition that checks if the primary key is in a list of values.
	OrPKIn(pks any, alias ...string) ConditionBuilder
	// PKNotIn is a condition that checks if the primary key is not in a list of values.
	PKNotIn(pks any, alias ...string) ConditionBuilder
	// OrPKNotIn is a condition that checks if the primary key is not in a list of values.
	OrPKNotIn(pks any, alias ...string) ConditionBuilder
}

// ConditionBuilder is a builder for conditions.
type ConditionBuilder interface {
	Applier[ConditionBuilder]
	AuditConditionBuilder
	PKConditionBuilder
	// Equals is a condition that checks if a column is equal to a value.
	Equals(column string, value any) ConditionBuilder
	// OrEquals is a condition that checks if a column is equal to a value.
	OrEquals(column string, value any) ConditionBuilder
	// EqualsColumn is a condition that checks if a column is equal to another column.
	EqualsColumn(column1, column2 string) ConditionBuilder
	// OrEqualsColumn is a condition that checks if a column is equal to another column.
	OrEqualsColumn(column1, column2 string) ConditionBuilder
	// EqualsSubQuery is a condition that checks if a column is equal to a subquery.
	EqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrEqualsSubQuery is a condition that checks if a column is equal to a subquery.
	OrEqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// EqualsAny is a condition that checks if a column is equal to any value returned by a subquery.
	EqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrEqualsAny is a condition that checks if a column is equal to any value returned by a subquery.
	OrEqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// EqualsAll is a condition that checks if a column is equal to all values returned by a subquery.
	EqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrEqualsAll is a condition that checks if a column is equal to all values returned by a subquery.
	OrEqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// EqualsExpr is a condition that checks if a column is equal to an expression.
	EqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrEqualsExpr is a condition that checks if a column is equal to an expression.
	OrEqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// NotEquals is a condition that checks if a column is not equal to a value.
	NotEquals(column string, value any) ConditionBuilder
	// OrNotEquals is a condition that checks if a column is not equal to a value.
	OrNotEquals(column string, value any) ConditionBuilder
	// NotEqualsColumn is a condition that checks if a column is not equal to another column.
	NotEqualsColumn(column1, column2 string) ConditionBuilder
	// OrNotEqualsColumn is a condition that checks if a column is not equal to another column.
	OrNotEqualsColumn(column1, column2 string) ConditionBuilder
	// NotEqualsSubQuery is a condition that checks if a column is not equal to a subquery.
	NotEqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrNotEqualsSubQuery is a condition that checks if a column is not equal to a subquery.
	OrNotEqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// NotEqualsAny is a condition that checks if a column is not equal to any value returned by a subquery.
	NotEqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrNotEqualsAny is a condition that checks if a column is not equal to any value returned by a subquery.
	OrNotEqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// NotEqualsAll is a condition that checks if a column is not equal to all values returned by a subquery.
	NotEqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrNotEqualsAll is a condition that checks if a column is not equal to all values returned by a subquery.
	OrNotEqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// NotEqualsExpr is a condition that checks if a column is not equal to an expression.
	NotEqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrNotEqualsExpr is a condition that checks if a column is not equal to an expression.
	OrNotEqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// GreaterThan is a condition that checks if a column is greater than a value.
	GreaterThan(column string, value any) ConditionBuilder
	// OrGreaterThan is a condition that checks if a column is greater than a value.
	OrGreaterThan(column string, value any) ConditionBuilder
	// GreaterThanColumn is a condition that checks if a column is greater than another column.
	GreaterThanColumn(column1, column2 string) ConditionBuilder
	// OrGreaterThanColumn is a condition that checks if a column is greater than another column.
	OrGreaterThanColumn(column1, column2 string) ConditionBuilder
	// GreaterThanSubQuery is a condition that checks if a column is greater than a subquery.
	GreaterThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrGreaterThanSubQuery is a condition that checks if a column is greater than a subquery.
	OrGreaterThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// GreaterThanAny is a condition that checks if a column is greater than any value returned by a subquery.
	GreaterThanAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrGreaterThanAny is a condition that checks if a column is greater than any value returned by a subquery.
	OrGreaterThanAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// GreaterThanAll is a condition that checks if a column is greater than all values returned by a subquery.
	GreaterThanAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrGreaterThanAll is a condition that checks if a column is greater than all values returned by a subquery.
	OrGreaterThanAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// GreaterThanExpr is a condition that checks if a column is greater than an expression.
	GreaterThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrGreaterThanExpr is a condition that checks if a column is greater than an expression.
	OrGreaterThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// GreaterThanOrEqual is a condition that checks if a column is greater than or equal to a value.
	GreaterThanOrEqual(column string, value any) ConditionBuilder
	// OrGreaterThanOrEqual is a condition that checks if a column is greater than or equal to a value.
	OrGreaterThanOrEqual(column string, value any) ConditionBuilder
	// GreaterThanOrEqualColumn is a condition that checks if a column is greater than or equal to another column.
	GreaterThanOrEqualColumn(column1, column2 string) ConditionBuilder
	// OrGreaterThanOrEqualColumn is a condition that checks if a column is greater than or equal to another column.
	OrGreaterThanOrEqualColumn(column1, column2 string) ConditionBuilder
	// GreaterThanOrEqualSubQuery is a condition that checks if a column is greater than or equal to a subquery.
	GreaterThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrGreaterThanOrEqualSubQuery is a condition that checks if a column is greater than or equal to a subquery.
	OrGreaterThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// GreaterThanOrEqualAny is a condition that checks if a column is greater than or equal to any value returned by a subquery.
	GreaterThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrGreaterThanOrEqualAny is a condition that checks if a column is greater than or equal to any value returned by a subquery.
	OrGreaterThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// GreaterThanOrEqualAll is a condition that checks if a column is greater than or equal to all values returned by a subquery.
	GreaterThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrGreaterThanOrEqualAll is a condition that checks if a column is greater than or equal to all values returned by a subquery.
	OrGreaterThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// GreaterThanOrEqualExpr is a condition that checks if a column is greater than or equal to an expression.
	GreaterThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrGreaterThanOrEqualExpr is a condition that checks if a column is greater than or equal to an expression.
	OrGreaterThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// LessThan is a condition that checks if a column is less than a value.
	LessThan(column string, value any) ConditionBuilder
	// OrLessThan is a condition that checks if a column is less than a value.
	OrLessThan(column string, value any) ConditionBuilder
	// LessThanColumn is a condition that checks if a column is less than another column.
	LessThanColumn(column1, column2 string) ConditionBuilder
	// OrLessThanColumn is a condition that checks if a column is less than another column.
	OrLessThanColumn(column1, column2 string) ConditionBuilder
	// LessThanSubQuery is a condition that checks if a column is less than a subquery.
	LessThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrLessThanSubQuery is a condition that checks if a column is less than a subquery.
	OrLessThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// LessThanAny is a condition that checks if a column is less than any value returned by a subquery.
	LessThanAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrLessThanAny is a condition that checks if a column is less than any value returned by a subquery.
	OrLessThanAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// LessThanAll is a condition that checks if a column is less than all values returned by a subquery.
	LessThanAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrLessThanAll is a condition that checks if a column is less than all values returned by a subquery.
	OrLessThanAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// LessThanExpr is a condition that checks if a column is less than an expression.
	LessThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrLessThanExpr is a condition that checks if a column is less than an expression.
	OrLessThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// LessThanOrEqual is a condition that checks if a column is less than or equal to a value.
	LessThanOrEqual(column string, value any) ConditionBuilder
	// OrLessThanOrEqual is a condition that checks if a column is less than or equal to a value.
	OrLessThanOrEqual(column string, value any) ConditionBuilder
	// LessThanOrEqualColumn is a condition that checks if a column is less than or equal to another column.
	LessThanOrEqualColumn(column1, column2 string) ConditionBuilder
	// OrLessThanOrEqualColumn is a condition that checks if a column is less than or equal to another column.
	OrLessThanOrEqualColumn(column1, column2 string) ConditionBuilder
	// LessThanOrEqualSubQuery is a condition that checks if a column is less than or equal to a subquery.
	LessThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrLessThanOrEqualSubQuery is a condition that checks if a column is less than or equal to a subquery.
	OrLessThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// LessThanOrEqualAny is a condition that checks if a column is less than or equal to any value returned by a subquery.
	LessThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrLessThanOrEqualAny is a condition that checks if a column is less than or equal to any value returned by a subquery.
	OrLessThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder
	// LessThanOrEqualAll is a condition that checks if a column is less than or equal to all values returned by a subquery.
	LessThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrLessThanOrEqualAll is a condition that checks if a column is less than or equal to all values returned by a subquery.
	OrLessThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder
	// LessThanOrEqualExpr is a condition that checks if a column is less than or equal to an expression.
	LessThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrLessThanOrEqualExpr is a condition that checks if a column is less than or equal to an expression.
	OrLessThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// Between is a condition that checks if a column is between two values.
	Between(column string, start, end any) ConditionBuilder
	// OrBetween is a condition that checks if a column is between two values.
	OrBetween(column string, start, end any) ConditionBuilder
	// BetweenExpr is a condition that checks if a column is between an expression and a value.
	BetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder
	// OrBetweenExpr is a condition that checks if a column is between an expression and a value.
	OrBetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder
	// NotBetween is a condition that checks if a column is not between two values.
	NotBetween(column string, start, end any) ConditionBuilder
	// OrNotBetween is a condition that checks if a column is not between two values.
	OrNotBetween(column string, start, end any) ConditionBuilder
	// NotBetweenExpr is a condition that checks if a column is not between an expression and a value.
	NotBetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder
	// OrNotBetweenExpr is a condition that checks if a column is not between an expression and a value.
	OrNotBetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder
	// In is a condition that checks if a column is in a list of values.
	In(column string, values any) ConditionBuilder
	// OrIn is a condition that checks if a column is in a list of values.
	OrIn(column string, values any) ConditionBuilder
	// InSubQuery is a condition that checks if a column is in a subquery.
	InSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrInSubQuery is a condition that checks if a column is in a subquery.
	OrInSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// InExpr is a condition that checks if a column is in an expression.
	InExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrInExpr is a condition that checks if a column is in an expression.
	OrInExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// NotIn is a condition that checks if a column is not in a list of values.
	NotIn(column string, values any) ConditionBuilder
	// OrNotIn is a condition that checks if a column is not in a list of values.
	OrNotIn(column string, values any) ConditionBuilder
	// NotInSubQuery is a condition that checks if a column is not in a subquery.
	NotInSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// OrNotInSubQuery is a condition that checks if a column is not in a subquery.
	OrNotInSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder
	// NotInExpr is a condition that checks if a column is not in an expression.
	NotInExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// OrNotInExpr is a condition that checks if a column is not in an expression.
	OrNotInExpr(column string, builder func(ExprBuilder) any) ConditionBuilder
	// IsNull is a condition that checks if a column is null.
	IsNull(column string) ConditionBuilder
	// OrIsNull is a condition that checks if a column is null.
	OrIsNull(column string) ConditionBuilder
	// IsNullSubQuery is a condition that checks if a column is null.
	IsNullSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// OrIsNullSubQuery is a condition that checks if a column is null.
	OrIsNullSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// IsNullExpr is a condition that checks if a column is null.
	IsNullExpr(builder func(ExprBuilder) any) ConditionBuilder
	// OrIsNullExpr is a condition that checks if a column is null.
	OrIsNullExpr(builder func(ExprBuilder) any) ConditionBuilder
	// IsNotNull is a condition that checks if a column is not null.
	IsNotNull(column string) ConditionBuilder
	// OrIsNotNull is a condition that checks if a column is not null.
	OrIsNotNull(column string) ConditionBuilder
	// IsNotNullSubQuery is a condition that checks if a column is not null.
	IsNotNullSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// OrIsNotNullSubQuery is a condition that checks if a column is not null.
	OrIsNotNullSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// IsNotNullExpr is a condition that checks if a column is not null.
	IsNotNullExpr(builder func(ExprBuilder) any) ConditionBuilder
	// OrIsNotNullExpr is a condition that checks if a column is not null.
	OrIsNotNullExpr(builder func(ExprBuilder) any) ConditionBuilder
	// IsTrue is a condition that checks if a column is true.
	IsTrue(column string) ConditionBuilder
	// OrIsTrue is a condition that checks if a column is true.
	OrIsTrue(column string) ConditionBuilder
	// IsTrueSubQuery is a condition that checks if a column is true.
	IsTrueSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// OrIsTrueSubQuery is a condition that checks if a column is true.
	OrIsTrueSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// IsTrueExpr is a condition that checks if a column is true.
	IsTrueExpr(builder func(ExprBuilder) any) ConditionBuilder
	// OrIsTrueExpr is a condition that checks if a column is true.
	OrIsTrueExpr(builder func(ExprBuilder) any) ConditionBuilder
	// IsFalse is a condition that checks if a column is false.
	IsFalse(column string) ConditionBuilder
	// OrIsFalse is a condition that checks if a column is false.
	OrIsFalse(column string) ConditionBuilder
	// IsFalseSubQuery is a condition that checks if a column is false.
	IsFalseSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// OrIsFalseSubQuery is a condition that checks if a column is false.
	OrIsFalseSubQuery(builder func(query SelectQuery)) ConditionBuilder
	// IsFalseExpr is a condition that checks if a column is false.
	IsFalseExpr(builder func(ExprBuilder) any) ConditionBuilder
	// OrIsFalseExpr is a condition that checks if a column is false.
	OrIsFalseExpr(builder func(ExprBuilder) any) ConditionBuilder
	// Contains is a condition that checks if a column contains a value.
	Contains(column, value string) ConditionBuilder
	// OrContains is a condition that checks if a column contains a value.
	OrContains(column, value string) ConditionBuilder
	// ContainsAny is a condition that checks if a column contains any of the values.
	ContainsAny(column string, values []string) ConditionBuilder
	// OrContainsAny is a condition that checks if a column contains any of the values.
	OrContainsAny(column string, values []string) ConditionBuilder
	// ContainsIgnoreCase is a condition that checks if a column contains a value, ignoring case.
	ContainsIgnoreCase(column, value string) ConditionBuilder
	// OrContainsIgnoreCase is a condition that checks if a column contains a value, ignoring case.
	OrContainsIgnoreCase(column, value string) ConditionBuilder
	// ContainsAnyIgnoreCase is a condition that checks if a column contains any of the values, ignoring case.
	ContainsAnyIgnoreCase(column string, values []string) ConditionBuilder
	// OrContainsAnyIgnoreCase is a condition that checks if a column contains any of the values, ignoring case.
	OrContainsAnyIgnoreCase(column string, values []string) ConditionBuilder
	// NotContains is a condition that checks if a column does not contain a value.
	NotContains(column, value string) ConditionBuilder
	// OrNotContains is a condition that checks if a column does not contain a value.
	OrNotContains(column, value string) ConditionBuilder
	// NotContainsAny is a condition that checks if a column does not contain any of the values.
	NotContainsAny(column string, values []string) ConditionBuilder
	// OrNotContainsAny is a condition that checks if a column does not contain any of the values.
	OrNotContainsAny(column string, values []string) ConditionBuilder
	// NotContainsIgnoreCase is a condition that checks if a column does not contain a value, ignoring case.
	NotContainsIgnoreCase(column, value string) ConditionBuilder
	// OrNotContainsIgnoreCase is a condition that checks if a column does not contain a value, ignoring case.
	OrNotContainsIgnoreCase(column, value string) ConditionBuilder
	// NotContainsAnyIgnoreCase is a condition that checks if a column does not contain any of the values, ignoring case.
	NotContainsAnyIgnoreCase(column string, values []string) ConditionBuilder
	// OrNotContainsAnyIgnoreCase is a condition that checks if a column does not contain any of the values, ignoring case.
	OrNotContainsAnyIgnoreCase(column string, values []string) ConditionBuilder
	// StartsWith is a condition that checks if a column starts with a value.
	StartsWith(column, value string) ConditionBuilder
	// OrStartsWith is a condition that checks if a column starts with a value.
	OrStartsWith(column, value string) ConditionBuilder
	// StartsWithAny is a condition that checks if a column starts with any of the values.
	StartsWithAny(column string, values []string) ConditionBuilder
	// OrStartsWithAny is a condition that checks if a column starts with any of the values.
	OrStartsWithAny(column string, values []string) ConditionBuilder
	// StartsWithIgnoreCase is a condition that checks if a column starts with a value, ignoring case.
	StartsWithIgnoreCase(column, value string) ConditionBuilder
	// OrStartsWithIgnoreCase is a condition that checks if a column starts with a value, ignoring case.
	OrStartsWithIgnoreCase(column, value string) ConditionBuilder
	// StartsWithAnyIgnoreCase is a condition that checks if a column starts with any of the values, ignoring case.
	StartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// OrStartsWithAnyIgnoreCase is a condition that checks if a column starts with any of the values, ignoring case.
	OrStartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// NotStartsWith is a condition that checks if a column does not start with a value.
	NotStartsWith(column, value string) ConditionBuilder
	// OrNotStartsWith is a condition that checks if a column does not start with a value.
	OrNotStartsWith(column, value string) ConditionBuilder
	// NotStartsWithAny is a condition that checks if a column does not start with any of the values.
	NotStartsWithAny(column string, values []string) ConditionBuilder
	// OrNotStartsWithAny is a condition that checks if a column does not start with any of the values.
	OrNotStartsWithAny(column string, values []string) ConditionBuilder
	// NotStartsWithIgnoreCase is a condition that checks if a column does not start with a value, ignoring case.
	NotStartsWithIgnoreCase(column, value string) ConditionBuilder
	// OrNotStartsWithIgnoreCase is a condition that checks if a column does not start with a value, ignoring case.
	OrNotStartsWithIgnoreCase(column, value string) ConditionBuilder
	// NotStartsWithAnyIgnoreCase is a condition that checks if a column does not start with any of the values, ignoring case.
	NotStartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// OrNotStartsWithAnyIgnoreCase is a condition that checks if a column does not start with any of the values, ignoring case.
	OrNotStartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// EndsWith is a condition that checks if a column ends with a value.
	EndsWith(column, value string) ConditionBuilder
	// OrEndsWith is a condition that checks if a column ends with a value.
	OrEndsWith(column, value string) ConditionBuilder
	// EndsWithAny is a condition that checks if a column ends with any of the values.
	EndsWithAny(column string, values []string) ConditionBuilder
	// OrEndsWithAny is a condition that checks if a column ends with any of the values.
	OrEndsWithAny(column string, values []string) ConditionBuilder
	// EndsWithIgnoreCase is a condition that checks if a column ends with a value, ignoring case.
	EndsWithIgnoreCase(column, value string) ConditionBuilder
	// OrEndsWithIgnoreCase is a condition that checks if a column ends with a value, ignoring case.
	OrEndsWithIgnoreCase(column, value string) ConditionBuilder
	// EndsWithAnyIgnoreCase is a condition that checks if a column ends with any of the values, ignoring case.
	EndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// OrEndsWithAnyIgnoreCase is a condition that checks if a column ends with any of the values, ignoring case.
	OrEndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// NotEndsWith is a condition that checks if a column does not end with a value.
	NotEndsWith(column, value string) ConditionBuilder
	// OrNotEndsWith is a condition that checks if a column does not end with a value.
	OrNotEndsWith(column, value string) ConditionBuilder
	// NotEndsWithAny is a condition that checks if a column does not end with any of the values.
	NotEndsWithAny(column string, values []string) ConditionBuilder
	// OrNotEndsWithAny is a condition that checks if a column does not end with any of the values.
	OrNotEndsWithAny(column string, values []string) ConditionBuilder
	// NotEndsWithIgnoreCase is a condition that checks if a column does not end with a value, ignoring case.
	NotEndsWithIgnoreCase(column, value string) ConditionBuilder
	// OrNotEndsWithIgnoreCase is a condition that checks if a column does not end with a value, ignoring case.
	OrNotEndsWithIgnoreCase(column, value string) ConditionBuilder
	// NotEndsWithAnyIgnoreCase is a condition that checks if a column does not end with any of the values, ignoring case.
	NotEndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// OrNotEndsWithAnyIgnoreCase is a condition that checks if a column does not end with any of the values, ignoring case.
	OrNotEndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder
	// Expr is a condition that checks if an expression is true.
	Expr(builder func(ExprBuilder) any) ConditionBuilder
	// OrExpr is a condition that checks if an expression is true.
	OrExpr(builder func(ExprBuilder) any) ConditionBuilder
	// Group is a condition that checks if a group of conditions are true.
	Group(builder func(ConditionBuilder)) ConditionBuilder
	// OrGroup is a condition that checks if a group of conditions are true.
	OrGroup(builder func(ConditionBuilder)) ConditionBuilder
}
