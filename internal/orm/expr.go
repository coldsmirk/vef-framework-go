package orm

import "github.com/uptrace/bun/schema"

// ExprBuilder provides a fluent API for building SQL expressions including
// aggregates, window functions, string/date/math functions, and conditional logic.
type ExprBuilder interface {
	// Column builds a column expression. If withTableAlias is false, skips automatic table alias.
	Column(column string, withTableAlias ...bool) schema.QueryAppender
	// TableColumns selects all columns of the current model's table. If withTableAlias is false, skips table alias prefix.
	TableColumns(withTableAlias ...bool) schema.QueryAppender
	// AllColumns selects all columns (*) optionally qualified with a table alias.
	AllColumns(tableAlias ...string) schema.QueryAppender
	// Null returns a SQL NULL literal.
	Null() schema.QueryAppender
	// IsNull tests whether the expression is NULL.
	IsNull(expr any) schema.QueryAppender
	// IsNotNull tests whether the expression is not NULL.
	IsNotNull(expr any) schema.QueryAppender
	// Literal wraps a Go value as a SQL literal with proper escaping.
	Literal(value any) schema.QueryAppender
	// Order builds an ORDER BY expression via the OrderBuilder DSL.
	Order(func(OrderBuilder)) schema.QueryAppender
	// Case builds a CASE WHEN ... THEN ... ELSE ... END expression via the CaseBuilder DSL.
	Case(func(CaseBuilder)) schema.QueryAppender
	// SubQuery builds a scalar subquery expression.
	SubQuery(func(SelectQuery)) schema.QueryAppender
	// Exists builds an EXISTS (subquery) expression.
	Exists(func(SelectQuery)) schema.QueryAppender
	// NotExists builds a NOT EXISTS (subquery) expression.
	NotExists(func(SelectQuery)) schema.QueryAppender
	// Paren wraps the expression in parentheses.
	Paren(expr any) schema.QueryAppender
	// Not negates the expression with NOT.
	Not(expr any) schema.QueryAppender
	// Any builds an ANY (subquery) expression for comparison with any row.
	Any(func(SelectQuery)) schema.QueryAppender
	// All builds an ALL (subquery) expression for comparison with all rows.
	All(func(SelectQuery)) schema.QueryAppender

	// ========== Arithmetic Operators ==========

	// Add returns left + right.
	Add(left, right any) schema.QueryAppender
	// Subtract returns left - right.
	Subtract(left, right any) schema.QueryAppender
	// Multiply returns left * right.
	Multiply(left, right any) schema.QueryAppender
	// Divide returns left / right.
	Divide(left, right any) schema.QueryAppender

	// ========== Comparison Operators ==========

	// Equals tests left = right.
	Equals(left, right any) schema.QueryAppender
	// NotEquals tests left <> right.
	NotEquals(left, right any) schema.QueryAppender
	// GreaterThan tests left > right.
	GreaterThan(left, right any) schema.QueryAppender
	// GreaterThanOrEqual tests left >= right.
	GreaterThanOrEqual(left, right any) schema.QueryAppender
	// LessThan tests left < right.
	LessThan(left, right any) schema.QueryAppender
	// LessThanOrEqual tests left <= right.
	LessThanOrEqual(left, right any) schema.QueryAppender
	// Between tests expr BETWEEN lower AND upper (inclusive).
	Between(expr, lower, upper any) schema.QueryAppender
	// NotBetween tests expr NOT BETWEEN lower AND upper.
	NotBetween(expr, lower, upper any) schema.QueryAppender
	// In tests expr IN (values...).
	In(expr any, values ...any) schema.QueryAppender
	// NotIn tests expr NOT IN (values...).
	NotIn(expr any, values ...any) schema.QueryAppender
	// IsTrue tests whether the expression evaluates to true.
	IsTrue(expr any) schema.QueryAppender
	// IsFalse tests whether the expression evaluates to false.
	IsFalse(expr any) schema.QueryAppender

	// ========== Expression Building ==========

	// Expr builds a raw SQL expression with optional argument binding (e.g., "? + 1", col).
	Expr(expr string, args ...any) schema.QueryAppender
	// Exprs concatenates multiple expressions with spaces.
	Exprs(exprs ...any) schema.QueryAppender
	// ExprsWithSep concatenates expressions with a custom separator.
	ExprsWithSep(separator any, exprs ...any) schema.QueryAppender
	// ExprByDialect selects the appropriate expression builder for the current database dialect.
	ExprByDialect(exprs DialectExprs) schema.QueryAppender
	// ExecByDialect executes a dialect-specific function based on the current database dialect.
	ExecByDialect(execs DialectExecs)
	// ExecByDialectWithErr executes a dialect-specific function that may return an error.
	ExecByDialectWithErr(execs DialectExecsWithErr) error
	// FragmentByDialect returns a dialect-specific SQL fragment as raw bytes.
	FragmentByDialect(fragments DialectFragments) ([]byte, error)

	// ========== Aggregate Functions ==========

	// Count builds a COUNT aggregate with optional DISTINCT and FILTER via the builder DSL.
	Count(func(CountBuilder)) schema.QueryAppender
	// CountColumn builds COUNT(column) with optional DISTINCT.
	CountColumn(column string, distinct ...bool) schema.QueryAppender
	// CountAll builds COUNT(*) with optional DISTINCT.
	CountAll(distinct ...bool) schema.QueryAppender
	// Sum builds a SUM aggregate via the builder DSL.
	Sum(func(SumBuilder)) schema.QueryAppender
	// SumColumn builds SUM(column) with optional DISTINCT.
	SumColumn(column string, distinct ...bool) schema.QueryAppender
	// Avg builds an AVG aggregate via the builder DSL.
	Avg(func(AvgBuilder)) schema.QueryAppender
	// AvgColumn builds AVG(column) with optional DISTINCT.
	AvgColumn(column string, distinct ...bool) schema.QueryAppender
	// Min builds a MIN aggregate via the builder DSL.
	Min(func(MinBuilder)) schema.QueryAppender
	// MinColumn builds MIN(column).
	MinColumn(column string) schema.QueryAppender
	// Max builds a MAX aggregate via the builder DSL.
	Max(func(MaxBuilder)) schema.QueryAppender
	// MaxColumn builds MAX(column).
	MaxColumn(column string) schema.QueryAppender
	// StringAgg builds a STRING_AGG (GROUP_CONCAT) aggregate via the builder DSL.
	StringAgg(func(StringAggBuilder)) schema.QueryAppender
	// ArrayAgg builds an ARRAY_AGG aggregate via the builder DSL.
	ArrayAgg(func(ArrayAggBuilder)) schema.QueryAppender
	// JSONObjectAgg builds a JSON_OBJECT_AGG aggregate via the builder DSL.
	JSONObjectAgg(func(JSONObjectAggBuilder)) schema.QueryAppender
	// JSONArrayAgg builds a JSON_ARRAY_AGG aggregate via the builder DSL.
	JSONArrayAgg(func(JSONArrayAggBuilder)) schema.QueryAppender
	// BitOr builds a BIT_OR aggregate via the builder DSL.
	BitOr(func(BitOrBuilder)) schema.QueryAppender
	// BitAnd builds a BIT_AND aggregate via the builder DSL.
	BitAnd(func(BitAndBuilder)) schema.QueryAppender
	// BoolOr builds a BOOL_OR aggregate via the builder DSL.
	BoolOr(func(BoolOrBuilder)) schema.QueryAppender
	// BoolAnd builds a BOOL_AND aggregate via the builder DSL.
	BoolAnd(func(BoolAndBuilder)) schema.QueryAppender
	// StdDev builds a STDDEV aggregate via the builder DSL.
	StdDev(func(StdDevBuilder)) schema.QueryAppender
	// Variance builds a VARIANCE aggregate via the builder DSL.
	Variance(func(VarianceBuilder)) schema.QueryAppender

	// ========== Window Functions ==========

	// RowNumber builds ROW_NUMBER() OVER (...) via the builder DSL.
	RowNumber(func(RowNumberBuilder)) schema.QueryAppender
	// Rank builds RANK() OVER (...) — rows with equal values get the same rank with gaps.
	Rank(func(RankBuilder)) schema.QueryAppender
	// DenseRank builds DENSE_RANK() OVER (...) — like Rank but without gaps in ranking sequence.
	DenseRank(func(DenseRankBuilder)) schema.QueryAppender
	// PercentRank builds PERCENT_RANK() OVER (...) — relative rank as a fraction between 0 and 1.
	PercentRank(func(PercentRankBuilder)) schema.QueryAppender
	// CumeDist builds CUME_DIST() OVER (...) — cumulative distribution of a value.
	CumeDist(func(CumeDistBuilder)) schema.QueryAppender
	// NTile builds NTILE(n) OVER (...) — divides rows into n roughly equal groups.
	NTile(func(NTileBuilder)) schema.QueryAppender
	// Lag builds LAG(column, offset, default) OVER (...) — accesses a preceding row's value.
	Lag(func(LagBuilder)) schema.QueryAppender
	// Lead builds LEAD(column, offset, default) OVER (...) — accesses a following row's value.
	Lead(func(LeadBuilder)) schema.QueryAppender
	// FirstValue builds FIRST_VALUE(column) OVER (...) — returns the first value in the window frame.
	FirstValue(func(FirstValueBuilder)) schema.QueryAppender
	// LastValue builds LAST_VALUE(column) OVER (...) — returns the last value in the window frame.
	LastValue(func(LastValueBuilder)) schema.QueryAppender
	// NthValue builds NTH_VALUE(column, n) OVER (...) — returns the nth value in the window frame.
	NthValue(func(NthValueBuilder)) schema.QueryAppender
	// WinCount builds COUNT() as a window function with OVER clause.
	WinCount(func(WindowCountBuilder)) schema.QueryAppender
	// WinSum builds SUM() as a window function with OVER clause.
	WinSum(func(WindowSumBuilder)) schema.QueryAppender
	// WinAvg builds AVG() as a window function with OVER clause.
	WinAvg(func(WindowAvgBuilder)) schema.QueryAppender
	// WinMin builds MIN() as a window function with OVER clause.
	WinMin(func(WindowMinBuilder)) schema.QueryAppender
	// WinMax builds MAX() as a window function with OVER clause.
	WinMax(func(WindowMaxBuilder)) schema.QueryAppender
	// WinStringAgg builds STRING_AGG() as a window function with OVER clause.
	WinStringAgg(func(WindowStringAggBuilder)) schema.QueryAppender
	// WinArrayAgg builds ARRAY_AGG() as a window function with OVER clause.
	WinArrayAgg(func(WindowArrayAggBuilder)) schema.QueryAppender
	// WinStdDev builds STDDEV() as a window function with OVER clause.
	WinStdDev(func(WindowStdDevBuilder)) schema.QueryAppender
	// WinVariance builds VARIANCE() as a window function with OVER clause.
	WinVariance(func(WindowVarianceBuilder)) schema.QueryAppender
	// WinJSONObjectAgg builds JSON_OBJECT_AGG() as a window function with OVER clause.
	WinJSONObjectAgg(func(WindowJSONObjectAggBuilder)) schema.QueryAppender
	// WinJSONArrayAgg builds JSON_ARRAY_AGG() as a window function with OVER clause.
	WinJSONArrayAgg(func(WindowJSONArrayAggBuilder)) schema.QueryAppender
	// WinBitOr builds BIT_OR() as a window function with OVER clause.
	WinBitOr(func(WindowBitOrBuilder)) schema.QueryAppender
	// WinBitAnd builds BIT_AND() as a window function with OVER clause.
	WinBitAnd(func(WindowBitAndBuilder)) schema.QueryAppender
	// WinBoolOr builds BOOL_OR() as a window function with OVER clause.
	WinBoolOr(func(WindowBoolOrBuilder)) schema.QueryAppender
	// WinBoolAnd builds BOOL_AND() as a window function with OVER clause.
	WinBoolAnd(func(WindowBoolAndBuilder)) schema.QueryAppender

	// ========== String Functions ==========

	// Concat concatenates multiple values into a single string.
	Concat(args ...any) schema.QueryAppender
	// ConcatWithSep concatenates values with a separator between each pair.
	ConcatWithSep(separator any, args ...any) schema.QueryAppender
	// SubString extracts a substring. start is 1-based; length is optional.
	SubString(expr, start any, length ...any) schema.QueryAppender
	// Upper converts a string to uppercase.
	Upper(expr any) schema.QueryAppender
	// Lower converts a string to lowercase.
	Lower(expr any) schema.QueryAppender
	// Trim removes leading and trailing whitespace.
	Trim(expr any) schema.QueryAppender
	// TrimLeft removes leading whitespace.
	TrimLeft(expr any) schema.QueryAppender
	// TrimRight removes trailing whitespace.
	TrimRight(expr any) schema.QueryAppender
	// Length returns the length in bytes.
	Length(expr any) schema.QueryAppender
	// CharLength returns the length in characters.
	CharLength(expr any) schema.QueryAppender
	// Position finds the position of substring in string (1-based, 0 if not found).
	Position(substring, str any) schema.QueryAppender
	// Left extracts the leftmost N characters.
	Left(expr, length any) schema.QueryAppender
	// Right extracts the rightmost N characters.
	Right(expr, length any) schema.QueryAppender
	// Repeat repeats the string N times.
	Repeat(expr, count any) schema.QueryAppender
	// Replace replaces all occurrences of search with replacement.
	Replace(expr, search, replacement any) schema.QueryAppender
	// Contains tests whether expr contains substr (case-sensitive).
	Contains(expr, substr any) schema.QueryAppender
	// StartsWith tests whether expr starts with prefix (case-sensitive).
	StartsWith(expr, prefix any) schema.QueryAppender
	// EndsWith tests whether expr ends with suffix (case-sensitive).
	EndsWith(expr, suffix any) schema.QueryAppender
	// ContainsIgnoreCase tests whether expr contains substr (case-insensitive).
	ContainsIgnoreCase(expr, substr any) schema.QueryAppender
	// StartsWithIgnoreCase tests whether expr starts with prefix (case-insensitive).
	StartsWithIgnoreCase(expr, prefix any) schema.QueryAppender
	// EndsWithIgnoreCase tests whether expr ends with suffix (case-insensitive).
	EndsWithIgnoreCase(expr, suffix any) schema.QueryAppender
	// Reverse reverses the string.
	Reverse(expr any) schema.QueryAppender

	// ========== Date and Time Functions ==========

	// CurrentDate returns the current date (without time).
	CurrentDate() schema.QueryAppender
	// CurrentTime returns the current time (without date).
	CurrentTime() schema.QueryAppender
	// CurrentTimestamp returns the current date and time with timezone.
	CurrentTimestamp() schema.QueryAppender
	// Now is an alias for CurrentTimestamp.
	Now() schema.QueryAppender
	// ExtractYear extracts the year component from a date/timestamp.
	ExtractYear(expr any) schema.QueryAppender
	// ExtractMonth extracts the month component (1-12) from a date/timestamp.
	ExtractMonth(expr any) schema.QueryAppender
	// ExtractDay extracts the day component from a date/timestamp.
	ExtractDay(expr any) schema.QueryAppender
	// ExtractHour extracts the hour component from a time/timestamp.
	ExtractHour(expr any) schema.QueryAppender
	// ExtractMinute extracts the minute component from a time/timestamp.
	ExtractMinute(expr any) schema.QueryAppender
	// ExtractSecond extracts the second component from a time/timestamp.
	ExtractSecond(expr any) schema.QueryAppender
	// DateTrunc truncates a date/timestamp to the specified precision unit.
	DateTrunc(unit DateTimeUnit, expr any) schema.QueryAppender
	// DateAdd adds an interval to a date/timestamp.
	DateAdd(expr, interval any, unit DateTimeUnit) schema.QueryAppender
	// DateSubtract subtracts an interval from a date/timestamp.
	DateSubtract(expr, interval any, unit DateTimeUnit) schema.QueryAppender
	// DateDiff computes the difference between two dates in the specified unit.
	DateDiff(start, end any, unit DateTimeUnit) schema.QueryAppender
	// Age computes the interval between two timestamps.
	Age(start, end any) schema.QueryAppender

	// ========== Math Functions ==========

	// Abs returns the absolute value.
	Abs(expr any) schema.QueryAppender
	// Ceil rounds up to the nearest integer.
	Ceil(expr any) schema.QueryAppender
	// Floor rounds down to the nearest integer.
	Floor(expr any) schema.QueryAppender
	// Round rounds to the specified precision (default 0 decimal places).
	Round(expr any, precision ...any) schema.QueryAppender
	// Trunc truncates to the specified precision without rounding.
	Trunc(expr any, precision ...any) schema.QueryAppender
	// Power returns base raised to the exponent power.
	Power(base, exponent any) schema.QueryAppender
	// Sqrt returns the square root.
	Sqrt(expr any) schema.QueryAppender
	// Exp returns e raised to the given power.
	Exp(expr any) schema.QueryAppender
	// Ln returns the natural logarithm (base e).
	Ln(expr any) schema.QueryAppender
	// Log returns the logarithm with specified base (default base 10).
	Log(expr any, base ...any) schema.QueryAppender
	// Sin returns the sine of the angle in radians.
	Sin(expr any) schema.QueryAppender
	// Cos returns the cosine of the angle in radians.
	Cos(expr any) schema.QueryAppender
	// Tan returns the tangent of the angle in radians.
	Tan(expr any) schema.QueryAppender
	// Asin returns the arc sine in radians.
	Asin(expr any) schema.QueryAppender
	// Acos returns the arc cosine in radians.
	Acos(expr any) schema.QueryAppender
	// Atan returns the arc tangent in radians.
	Atan(expr any) schema.QueryAppender
	// Pi returns the constant pi.
	Pi() schema.QueryAppender
	// Random returns a random value between 0.0 and 1.0.
	Random() schema.QueryAppender
	// Sign returns -1, 0, or 1.
	Sign(expr any) schema.QueryAppender
	// Mod returns the remainder of dividend / divisor.
	Mod(dividend, divisor any) schema.QueryAppender
	// Greatest returns the largest value among the arguments.
	Greatest(args ...any) schema.QueryAppender
	// Least returns the smallest value among the arguments.
	Least(args ...any) schema.QueryAppender

	// ========== Conditional Functions ==========

	// Coalesce returns the first non-NULL value among the arguments.
	Coalesce(args ...any) schema.QueryAppender
	// NullIf returns null if the two arguments are equal, otherwise the first argument.
	NullIf(expr1, expr2 any) schema.QueryAppender
	// IfNull returns defaultValue if expr is null, otherwise expr.
	IfNull(expr, defaultValue any) schema.QueryAppender

	// ========== Type Conversion Functions ==========

	// ToString casts the expression to a text/varchar type.
	ToString(expr any) schema.QueryAppender
	// ToInteger casts the expression to an integer type.
	ToInteger(expr any) schema.QueryAppender
	// ToDecimal casts the expression to a decimal/numeric type with optional precision.
	ToDecimal(expr any, precision ...any) schema.QueryAppender
	// ToFloat casts the expression to a floating-point type.
	ToFloat(expr any) schema.QueryAppender
	// ToBool casts the expression to a boolean type.
	ToBool(expr any) schema.QueryAppender
	// ToDate casts the expression to a date type with optional format pattern.
	ToDate(expr any, format ...any) schema.QueryAppender
	// ToTime casts the expression to a time type with optional format pattern.
	ToTime(expr any, format ...any) schema.QueryAppender
	// ToTimestamp casts the expression to a timestamp type with optional format pattern.
	ToTimestamp(expr any, format ...any) schema.QueryAppender
	// ToJSON casts the expression to a JSON/JSONB type.
	ToJSON(expr any) schema.QueryAppender

	// ========== JSON Functions ==========

	// JSONExtract extracts a value from a JSON document at the given path.
	JSONExtract(json, path any) schema.QueryAppender
	// JSONUnquote removes JSON quoting from a string value.
	JSONUnquote(expr any) schema.QueryAppender
	// JSONArray constructs a JSON array from the given values.
	JSONArray(args ...any) schema.QueryAppender
	// JSONObject constructs a JSON object from alternating key-value pairs.
	JSONObject(keyValues ...any) schema.QueryAppender
	// JSONContains tests whether a JSON document contains a specific value.
	JSONContains(json, value any) schema.QueryAppender
	// JSONContainsPath tests whether a JSON document contains data at the given path.
	JSONContainsPath(json, path any) schema.QueryAppender
	// JSONKeys returns the keys of a JSON object, optionally at the given path.
	JSONKeys(json any, path ...any) schema.QueryAppender
	// JSONLength returns the number of elements in a JSON array or keys in a JSON object.
	JSONLength(json any, path ...any) schema.QueryAppender
	// JSONType returns the type of a JSON value (e.g., "object", "array", "string").
	JSONType(json any, path ...any) schema.QueryAppender
	// JSONValid tests whether a string is valid JSON.
	JSONValid(expr any) schema.QueryAppender
	// JSONSet sets value at path (insert or update).
	JSONSet(json, path, value any) schema.QueryAppender
	// JSONInsert inserts value at path only if path doesn't exist.
	JSONInsert(json, path, value any) schema.QueryAppender
	// JSONReplace replaces value at path only if path exists.
	JSONReplace(json, path, value any) schema.QueryAppender
	// JSONArrayAppend appends a value to the JSON array at the given path.
	JSONArrayAppend(json, path, value any) schema.QueryAppender

	// ========== Utility Functions ==========

	// Decode implements Oracle-style case expression.
	// Usage: Decode(expr, search1, result1, search2, result2, ..., defaultResult)
	Decode(args ...any) schema.QueryAppender
}
