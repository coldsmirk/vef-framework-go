package orm

import (
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/schema"
)

// QueryExprBuilder implements the ExprBuilder interface, providing methods to build various SQL expressions.
type QueryExprBuilder struct {
	qb QueryBuilder
}

func (b *QueryExprBuilder) Column(column string, withTableAlias ...bool) schema.QueryAppender {
	needTableAlias := len(withTableAlias) == 0 || withTableAlias[0]

	before, after, ok := strings.Cut(column, ".")
	if ok {
		alias, name := before, after
		if strings.HasPrefix(alias, "?") {
			return b.Expr(alias+".?", bun.Name(name))
		}

		return b.Expr("?.?", bun.Name(alias), bun.Name(name))
	}

	if needTableAlias && b.qb.GetTable() != nil {
		return b.Expr("?TableAlias.?", bun.Name(column))
	}

	return b.Expr("?", bun.Name(column))
}

func (b *QueryExprBuilder) TableColumns(withTableAlias ...bool) schema.QueryAppender {
	needTableAlias := len(withTableAlias) == 0 || withTableAlias[0]

	if needTableAlias {
		return b.Expr(ExprTableColumns)
	}

	return b.Expr(ExprColumns)
}

func (b *QueryExprBuilder) AllColumns(tableAlias ...string) schema.QueryAppender {
	if len(tableAlias) > 0 && tableAlias[0] != "" {
		return b.Expr("?.*", bun.Name(tableAlias[0]))
	}

	if b.qb.GetTable() != nil {
		return b.Expr("?TableAlias.*")
	}

	return bun.Safe("*")
}

func (*QueryExprBuilder) Null() schema.QueryAppender {
	return bun.Safe(sqlNull)
}

func (b *QueryExprBuilder) IsNull(expr any) schema.QueryAppender {
	return b.Expr("? IS NULL", expr)
}

func (b *QueryExprBuilder) IsNotNull(expr any) schema.QueryAppender {
	return b.Expr("? IS NOT NULL", expr)
}

func (b *QueryExprBuilder) Literal(value any) schema.QueryAppender {
	return b.Expr("?", value)
}

func (b *QueryExprBuilder) Order(builder func(OrderBuilder)) schema.QueryAppender {
	ob := newOrderExpr(b)
	builder(ob)

	return ob
}

func (b *QueryExprBuilder) Case(builder func(CaseBuilder)) schema.QueryAppender {
	cb := newCaseExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) SubQuery(builder func(SelectQuery)) schema.QueryAppender {
	return b.Expr("(?)", b.qb.BuildSubQuery(builder))
}

func (b *QueryExprBuilder) Exists(builder func(SelectQuery)) schema.QueryAppender {
	return b.Expr("EXISTS (?)", b.qb.BuildSubQuery(builder))
}

func (b *QueryExprBuilder) NotExists(builder func(SelectQuery)) schema.QueryAppender {
	return b.Expr("NOT EXISTS (?)", b.qb.BuildSubQuery(builder))
}

func (b *QueryExprBuilder) Paren(expr any) schema.QueryAppender {
	return b.Expr("(?)", expr)
}

func (b *QueryExprBuilder) Not(expr any) schema.QueryAppender {
	return b.Expr("NOT (?)", expr)
}

func (b *QueryExprBuilder) Any(builder func(SelectQuery)) schema.QueryAppender {
	return b.Expr("ANY (?)", b.qb.BuildSubQuery(builder))
}

func (b *QueryExprBuilder) All(builder func(SelectQuery)) schema.QueryAppender {
	return b.Expr("ALL (?)", b.qb.BuildSubQuery(builder))
}

// ========== Arithmetic Operators ==========

func (b *QueryExprBuilder) Add(left, right any) schema.QueryAppender {
	return b.Expr("? + ?", left, right)
}

func (b *QueryExprBuilder) Subtract(left, right any) schema.QueryAppender {
	return b.Expr("? - ?", left, right)
}

func (b *QueryExprBuilder) Multiply(left, right any) schema.QueryAppender {
	return b.Expr("? * ?", left, right)
}

// Divide creates a division expression (left / right).
// To ensure consistent float results across all databases, we cast to REAL/DOUBLE/NUMERIC.
// This prevents integer division behavior in SQLite and PostgreSQL.
func (b *QueryExprBuilder) Divide(left, right any) schema.QueryAppender {
	decimalDivide := func() schema.QueryAppender {
		return b.Expr("? / ?", b.ToDecimal(left), b.ToDecimal(right))
	}

	return b.ExprByDialect(DialectExprs{
		SQLite:   decimalDivide,
		Postgres: decimalDivide,
		Default: func() schema.QueryAppender {
			return b.Expr("? / ?", left, right)
		},
	})
}

// ========== Comparison Operators ==========

func (b *QueryExprBuilder) Equals(left, right any) schema.QueryAppender {
	return b.Expr("? = ?", left, right)
}

func (b *QueryExprBuilder) NotEquals(left, right any) schema.QueryAppender {
	return b.Expr("? <> ?", left, right)
}

func (b *QueryExprBuilder) GreaterThan(left, right any) schema.QueryAppender {
	return b.Expr("? > ?", left, right)
}

func (b *QueryExprBuilder) GreaterThanOrEqual(left, right any) schema.QueryAppender {
	return b.Expr("? >= ?", left, right)
}

func (b *QueryExprBuilder) LessThan(left, right any) schema.QueryAppender {
	return b.Expr("? < ?", left, right)
}

func (b *QueryExprBuilder) LessThanOrEqual(left, right any) schema.QueryAppender {
	return b.Expr("? <= ?", left, right)
}

func (b *QueryExprBuilder) Between(expr, lower, upper any) schema.QueryAppender {
	return b.Expr("? BETWEEN ? AND ?", expr, lower, upper)
}

func (b *QueryExprBuilder) NotBetween(expr, lower, upper any) schema.QueryAppender {
	return b.Expr("? NOT BETWEEN ? AND ?", expr, lower, upper)
}

func (b *QueryExprBuilder) In(expr any, values ...any) schema.QueryAppender {
	return b.Expr("? IN ?", expr, bun.Tuple(values))
}

func (b *QueryExprBuilder) NotIn(expr any, values ...any) schema.QueryAppender {
	return b.Expr("? NOT IN ?", expr, bun.Tuple(values))
}

func (b *QueryExprBuilder) IsTrue(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Equals(expr, 1)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("? IS TRUE", expr)
		},
	})
}

func (b *QueryExprBuilder) IsFalse(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Equals(expr, 0)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("? IS FALSE", expr)
		},
	})
}

// ========== Expression Building ==========

func (*QueryExprBuilder) Expr(expr string, args ...any) schema.QueryAppender {
	return bun.SafeQuery(expr, args...)
}

func (*QueryExprBuilder) Exprs(exprs ...any) schema.QueryAppender {
	return newExpressions(", ", exprs...)
}

func (*QueryExprBuilder) ExprsWithSep(sep any, exprs ...any) schema.QueryAppender {
	return newExpressions(sep, exprs...)
}

// ExprByDialect creates a cross-database compatible expression.
// It selects the appropriate expression builder based on the current database dialect.
func (b *QueryExprBuilder) ExprByDialect(exprs DialectExprs) schema.QueryAppender {
	switch b.qb.Dialect().Name() {
	case dialect.Oracle:
		if exprs.Oracle != nil {
			return exprs.Oracle()
		}
	case dialect.MSSQL:
		if exprs.SQLServer != nil {
			return exprs.SQLServer()
		}
	case dialect.PG:
		if exprs.Postgres != nil {
			return exprs.Postgres()
		}
	case dialect.MySQL:
		if exprs.MySQL != nil {
			return exprs.MySQL()
		}
	case dialect.SQLite:
		if exprs.SQLite != nil {
			return exprs.SQLite()
		}
	}

	if exprs.Default != nil {
		return exprs.Default()
	}

	return b.Null()
}

// ExecByDialect executes database-specific side-effect callbacks based on the current dialect.
func (b *QueryExprBuilder) ExecByDialect(execs DialectExecs) {
	switch b.qb.Dialect().Name() {
	case dialect.Oracle:
		if execs.Oracle != nil {
			execs.Oracle()

			return
		}

	case dialect.MSSQL:
		if execs.SQLServer != nil {
			execs.SQLServer()

			return
		}

	case dialect.PG:
		if execs.Postgres != nil {
			execs.Postgres()

			return
		}

	case dialect.MySQL:
		if execs.MySQL != nil {
			execs.MySQL()

			return
		}

	case dialect.SQLite:
		if execs.SQLite != nil {
			execs.SQLite()

			return
		}
	}

	if execs.Default != nil {
		execs.Default()
	}
}

// ExecByDialectWithErr executes database-specific callbacks that can return an error.
func (b *QueryExprBuilder) ExecByDialectWithErr(execs DialectExecsWithErr) error {
	switch b.qb.Dialect().Name() {
	case dialect.Oracle:
		if execs.Oracle != nil {
			return execs.Oracle()
		}
	case dialect.MSSQL:
		if execs.SQLServer != nil {
			return execs.SQLServer()
		}
	case dialect.PG:
		if execs.Postgres != nil {
			return execs.Postgres()
		}
	case dialect.MySQL:
		if execs.MySQL != nil {
			return execs.MySQL()
		}
	case dialect.SQLite:
		if execs.SQLite != nil {
			return execs.SQLite()
		}
	}

	if execs.Default != nil {
		return execs.Default()
	}

	return ErrDialectHandlerMissing
}

// FragmentByDialect executes database-specific callbacks that return query fragments.
func (b *QueryExprBuilder) FragmentByDialect(fragments DialectFragments) ([]byte, error) {
	switch b.qb.Dialect().Name() {
	case dialect.Oracle:
		if fragments.Oracle != nil {
			return fragments.Oracle()
		}
	case dialect.MSSQL:
		if fragments.SQLServer != nil {
			return fragments.SQLServer()
		}
	case dialect.PG:
		if fragments.Postgres != nil {
			return fragments.Postgres()
		}
	case dialect.MySQL:
		if fragments.MySQL != nil {
			return fragments.MySQL()
		}
	case dialect.SQLite:
		if fragments.SQLite != nil {
			return fragments.SQLite()
		}
	}

	if fragments.Default != nil {
		return fragments.Default()
	}

	return nil, ErrDialectHandlerMissing
}

// ========== Aggregate Functions ==========

func (b *QueryExprBuilder) Count(builder func(CountBuilder)) schema.QueryAppender {
	cb := newCountExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) CountColumn(column string, distinct ...bool) schema.QueryAppender {
	return b.Count(func(cb CountBuilder) {
		if len(distinct) > 0 && distinct[0] {
			cb.Distinct()
		}

		cb.Column(column)
	})
}

func (b *QueryExprBuilder) CountAll(distinct ...bool) schema.QueryAppender {
	return b.Count(func(cb CountBuilder) {
		if len(distinct) > 0 && distinct[0] {
			cb.Distinct()
		}

		cb.All()
	})
}

func (b *QueryExprBuilder) Sum(builder func(SumBuilder)) schema.QueryAppender {
	cb := newSumExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) SumColumn(column string, distinct ...bool) schema.QueryAppender {
	return b.Sum(func(cb SumBuilder) {
		if len(distinct) > 0 && distinct[0] {
			cb.Distinct()
		}

		cb.Column(column)
	})
}

func (b *QueryExprBuilder) Avg(builder func(AvgBuilder)) schema.QueryAppender {
	cb := newAvgExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) AvgColumn(column string, distinct ...bool) schema.QueryAppender {
	return b.Avg(func(cb AvgBuilder) {
		if len(distinct) > 0 && distinct[0] {
			cb.Distinct()
		}

		cb.Column(column)
	})
}

func (b *QueryExprBuilder) Min(builder func(MinBuilder)) schema.QueryAppender {
	cb := newMinExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) MinColumn(column string) schema.QueryAppender {
	return b.Min(func(cb MinBuilder) {
		cb.Column(column)
	})
}

func (b *QueryExprBuilder) Max(builder func(MaxBuilder)) schema.QueryAppender {
	cb := newMaxExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) MaxColumn(column string) schema.QueryAppender {
	return b.Max(func(cb MaxBuilder) {
		cb.Column(column)
	})
}

func (b *QueryExprBuilder) StringAgg(builder func(StringAggBuilder)) schema.QueryAppender {
	cb := newStringAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) ArrayAgg(builder func(ArrayAggBuilder)) schema.QueryAppender {
	cb := newArrayAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) StdDev(builder func(StdDevBuilder)) schema.QueryAppender {
	cb := newStdDevExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) Variance(builder func(VarianceBuilder)) schema.QueryAppender {
	cb := newVarianceExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) JSONObjectAgg(builder func(JSONObjectAggBuilder)) schema.QueryAppender {
	cb := newJSONObjectAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) JSONArrayAgg(builder func(JSONArrayAggBuilder)) schema.QueryAppender {
	cb := newJSONArrayAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) BitOr(builder func(BitOrBuilder)) schema.QueryAppender {
	cb := newBitOrExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) BitAnd(builder func(BitAndBuilder)) schema.QueryAppender {
	cb := newBitAndExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) BoolOr(builder func(BoolOrBuilder)) schema.QueryAppender {
	cb := newBoolOrExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) BoolAnd(builder func(BoolAndBuilder)) schema.QueryAppender {
	cb := newBoolAndExpr(b.qb)
	builder(cb)

	return cb
}

// ========== Window Functions ==========

func (b *QueryExprBuilder) RowNumber(builder func(RowNumberBuilder)) schema.QueryAppender {
	cb := newRowNumberExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) Rank(builder func(RankBuilder)) schema.QueryAppender {
	cb := newRankExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) DenseRank(builder func(DenseRankBuilder)) schema.QueryAppender {
	cb := newDenseRankExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) PercentRank(builder func(PercentRankBuilder)) schema.QueryAppender {
	cb := newPercentRankExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) CumeDist(builder func(CumeDistBuilder)) schema.QueryAppender {
	cb := newCumeDistExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) NTile(builder func(NTileBuilder)) schema.QueryAppender {
	cb := newNTileExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) Lag(builder func(LagBuilder)) schema.QueryAppender {
	cb := newLagExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) Lead(builder func(LeadBuilder)) schema.QueryAppender {
	cb := newLeadExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) FirstValue(builder func(FirstValueBuilder)) schema.QueryAppender {
	cb := newFirstValueExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) LastValue(builder func(LastValueBuilder)) schema.QueryAppender {
	cb := newLastValueExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) NthValue(builder func(NthValueBuilder)) schema.QueryAppender {
	cb := newNthValueExpr(b)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinCount(builder func(WindowCountBuilder)) schema.QueryAppender {
	cb := newWindowCountExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinSum(builder func(WindowSumBuilder)) schema.QueryAppender {
	cb := newWindowSumExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinAvg(builder func(WindowAvgBuilder)) schema.QueryAppender {
	cb := newWindowAvgExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinMin(builder func(WindowMinBuilder)) schema.QueryAppender {
	cb := newWindowMinExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinMax(builder func(WindowMaxBuilder)) schema.QueryAppender {
	cb := newWindowMaxExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinStringAgg(builder func(WindowStringAggBuilder)) schema.QueryAppender {
	cb := newWindowStringAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinArrayAgg(builder func(WindowArrayAggBuilder)) schema.QueryAppender {
	cb := newWindowArrayAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinStdDev(builder func(WindowStdDevBuilder)) schema.QueryAppender {
	cb := newWindowStdDevExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinVariance(builder func(WindowVarianceBuilder)) schema.QueryAppender {
	cb := newWindowVarianceExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinJSONObjectAgg(builder func(WindowJSONObjectAggBuilder)) schema.QueryAppender {
	cb := newWindowJSONObjectAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinJSONArrayAgg(builder func(WindowJSONArrayAggBuilder)) schema.QueryAppender {
	cb := newWindowJSONArrayAggExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinBitOr(builder func(WindowBitOrBuilder)) schema.QueryAppender {
	cb := newWindowBitOrExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinBitAnd(builder func(WindowBitAndBuilder)) schema.QueryAppender {
	cb := newWindowBitAndExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinBoolOr(builder func(WindowBoolOrBuilder)) schema.QueryAppender {
	cb := newWindowBoolOrExpr(b.qb)
	builder(cb)

	return cb
}

func (b *QueryExprBuilder) WinBoolAnd(builder func(WindowBoolAndBuilder)) schema.QueryAppender {
	cb := newWindowBoolAndExpr(b.qb)
	builder(cb)

	return cb
}

// ========== String Functions ==========

func (b *QueryExprBuilder) Concat(args ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			if len(args) == 0 {
				return b.Expr("?", "")
			}

			if len(args) == 1 {
				return b.Expr("?", args[0])
			}

			return b.ExprsWithSep(" || ", args...)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CONCAT(?)", b.Exprs(args...))
		},
	})
}

func (b *QueryExprBuilder) ConcatWithSep(separator any, args ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			if len(args) == 0 {
				return b.Expr("?", "")
			}

			if len(args) == 1 {
				return b.Expr("?", args[0])
			}

			var parts []any

			for i, arg := range args {
				if i > 0 {
					parts = append(parts, separator)
				}

				parts = append(parts, arg)
			}

			return b.Concat(parts...)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CONCAT_WS(?, ?)", separator, b.Exprs(args...))
		},
	})
}

func (b *QueryExprBuilder) SubString(expr, start any, length ...any) schema.QueryAppender {
	substringFn := func(fnName string) schema.QueryAppender {
		if len(length) > 0 {
			return b.Expr("?(?, ?, ?)", bun.Safe(fnName), expr, start, length[0])
		}

		return b.Expr("?(?, ?)", bun.Safe(fnName), expr, start)
	}

	return b.ExprByDialect(DialectExprs{
		SQLite:  func() schema.QueryAppender { return substringFn("SUBSTR") },
		Default: func() schema.QueryAppender { return substringFn("SUBSTRING") },
	})
}

func (b *QueryExprBuilder) Upper(expr any) schema.QueryAppender {
	return b.Expr("UPPER(?)", expr)
}

func (b *QueryExprBuilder) Lower(expr any) schema.QueryAppender {
	return b.Expr("LOWER(?)", expr)
}

func (b *QueryExprBuilder) Trim(expr any) schema.QueryAppender {
	return b.Expr("TRIM(?)", expr)
}

func (b *QueryExprBuilder) TrimLeft(expr any) schema.QueryAppender {
	return b.Expr("LTRIM(?)", expr)
}

func (b *QueryExprBuilder) TrimRight(expr any) schema.QueryAppender {
	return b.Expr("RTRIM(?)", expr)
}

func (b *QueryExprBuilder) Length(expr any) schema.QueryAppender {
	return b.Expr("LENGTH(?)", expr)
}

func (b *QueryExprBuilder) CharLength(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Length(expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CHAR_LENGTH(?)", expr)
		},
	})
}

func (b *QueryExprBuilder) Position(substring, str any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("INSTR(?, ?)", str, substring)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("POSITION(? IN ?)", substring, str)
		},
	})
}

func (b *QueryExprBuilder) Left(expr, length any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.SubString(expr, 1, length)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("LEFT(?, ?)", expr, length)
		},
	})
}

func (b *QueryExprBuilder) Right(expr, length any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("SUBSTR(?, -?)", expr, length)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("RIGHT(?, ?)", expr, length)
		},
	})
}

func (b *QueryExprBuilder) Repeat(expr, count any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Replace(
				b.SubString(b.Expr("QUOTE(ZEROBLOB(?))", b.Divide(b.Paren(b.Add(count, 1)), 2)), 3, count),
				"0",
				expr,
			)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("REPEAT(?, ?)", expr, count)
		},
	})
}

func (b *QueryExprBuilder) Replace(expr, search, replacement any) schema.QueryAppender {
	return b.Expr("REPLACE(?, ?, ?)", expr, search, replacement)
}

// fuzzyMatch builds a fuzzy LIKE expression based on FuzzyKind.
// It optimizes for string literals by pre-building the LIKE pattern in Go code,
// avoiding runtime string concatenation in the database.
func (b *QueryExprBuilder) fuzzyMatch(expr, pattern any, kind FuzzyKind, ignoreCase bool) schema.QueryAppender {
	// Optimize for string literals: build pattern in Go
	if strPattern, ok := pattern.(string); ok {
		likePattern := kind.BuildPattern(strPattern)
		if ignoreCase {
			return b.ExprByDialect(DialectExprs{
				Postgres: func() schema.QueryAppender {
					return b.Expr("? ILIKE ?", expr, likePattern)
				},
				Default: func() schema.QueryAppender {
					return b.Expr("? LIKE ?", b.Lower(expr), strings.ToLower(likePattern))
				},
			})
		}

		return b.Expr("? LIKE ?", expr, likePattern)
	}

	// For dynamic expressions, build pattern using b.Concat()
	var likePattern any
	switch kind {
	case FuzzyContains:
		likePattern = b.Concat("%", pattern, "%")
	case FuzzyStarts:
		likePattern = b.Concat(pattern, "%")
	case FuzzyEnds:
		likePattern = b.Concat("%", pattern)
	default:
		likePattern = pattern
	}

	if ignoreCase {
		return b.ExprByDialect(DialectExprs{
			Postgres: func() schema.QueryAppender {
				return b.Expr("? ILIKE ?", expr, likePattern)
			},
			Default: func() schema.QueryAppender {
				return b.Expr("? LIKE ?", b.Lower(expr), b.Lower(likePattern))
			},
		})
	}

	return b.Expr("? LIKE ?", expr, likePattern)
}

func (b *QueryExprBuilder) Contains(expr, substr any) schema.QueryAppender {
	return b.fuzzyMatch(expr, substr, FuzzyContains, false)
}

func (b *QueryExprBuilder) StartsWith(expr, prefix any) schema.QueryAppender {
	return b.fuzzyMatch(expr, prefix, FuzzyStarts, false)
}

func (b *QueryExprBuilder) EndsWith(expr, suffix any) schema.QueryAppender {
	return b.fuzzyMatch(expr, suffix, FuzzyEnds, false)
}

func (b *QueryExprBuilder) ContainsIgnoreCase(expr, substr any) schema.QueryAppender {
	return b.fuzzyMatch(expr, substr, FuzzyContains, true)
}

func (b *QueryExprBuilder) StartsWithIgnoreCase(expr, prefix any) schema.QueryAppender {
	return b.fuzzyMatch(expr, prefix, FuzzyStarts, true)
}

func (b *QueryExprBuilder) EndsWithIgnoreCase(expr, suffix any) schema.QueryAppender {
	return b.fuzzyMatch(expr, suffix, FuzzyEnds, true)
}

func (b *QueryExprBuilder) Reverse(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("?", expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("REVERSE(?)", expr)
		},
	})
}

// ========== Date and Time Functions ==========

func (b *QueryExprBuilder) CurrentDate() schema.QueryAppender {
	return b.Expr("CURRENT_DATE")
}

func (b *QueryExprBuilder) CurrentTime() schema.QueryAppender {
	return b.Expr("CURRENT_TIME")
}

func (b *QueryExprBuilder) CurrentTimestamp() schema.QueryAppender {
	return b.Expr("CURRENT_TIMESTAMP")
}

func (b *QueryExprBuilder) Now() schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("DATETIME('now')")
		},
		Default: func() schema.QueryAppender {
			return b.Expr("NOW()")
		},
	})
}

// sqliteNormalizeTS converts the Go driver's timestamp format
// "YYYY-MM-DD HH:MM:SS +HHMM TZ" into ISO-8601 "YYYY-MM-DD HH:MM:SS+HH:MM"
// that SQLite date/time functions can parse. Short values (≤19 chars) pass through unchanged.
// Use this for arithmetic operations (DateTrunc, DateAdd, DateSubtract, DateDiff, Age, ToDate, ToTime, ToTimestamp)
// that require correct absolute-time semantics.
func (b *QueryExprBuilder) sqliteNormalizeTS(expr any) schema.QueryAppender {
	return b.Case(func(cb CaseBuilder) {
		cb.WhenExpr(b.GreaterThan(b.Length(expr), 19)).
			Then(b.Concat(
				b.SubString(expr, 1, 19),
				b.SubString(expr, 21, 3),
				":",
				b.SubString(expr, 24, 2),
			))
		cb.Else(expr)
	})
}

// sqliteLocalTS strips the timezone suffix from the Go driver's timestamp
// format, preserving the wall-clock datetime portion ("YYYY-MM-DD HH:MM:SS").
// Use this for extraction functions (ExtractYear/Month/Day/Hour/Minute/Second)
// where the caller expects local-time component values matching Go's time.Time accessors.
func (b *QueryExprBuilder) sqliteLocalTS(expr any) schema.QueryAppender {
	return b.SubString(expr, 1, 19)
}

// extractPart builds a dialect-aware EXTRACT expression.
// For SQLite, it uses STRFTIME with the given format code and casts the result to INTEGER.
// For other databases, it uses EXTRACT(part FROM expr).
func (b *QueryExprBuilder) extractPart(sqlPart, sqliteFormat string, expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.ToInteger(
				b.Expr("STRFTIME(?, ?)", sqliteFormat, b.sqliteLocalTS(expr)),
			)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("EXTRACT("+sqlPart+" FROM ?)", expr)
		},
	})
}

func (b *QueryExprBuilder) ExtractYear(expr any) schema.QueryAppender {
	return b.extractPart("YEAR", "%Y", expr)
}

func (b *QueryExprBuilder) ExtractMonth(expr any) schema.QueryAppender {
	return b.extractPart("MONTH", "%m", expr)
}

func (b *QueryExprBuilder) ExtractDay(expr any) schema.QueryAppender {
	return b.extractPart("DAY", "%d", expr)
}

func (b *QueryExprBuilder) ExtractHour(expr any) schema.QueryAppender {
	return b.extractPart("HOUR", "%H", expr)
}

func (b *QueryExprBuilder) ExtractMinute(expr any) schema.QueryAppender {
	return b.extractPart("MINUTE", "%M", expr)
}

func (b *QueryExprBuilder) ExtractSecond(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		MySQL: func() schema.QueryAppender {
			return b.ToDecimal(
				b.Expr("EXTRACT(SECOND FROM ?)", expr),
				10, 6,
			)
		},
		SQLite: func() schema.QueryAppender {
			return b.ToDecimal(
				b.Expr("STRFTIME(?, ?)", "%S", b.sqliteLocalTS(expr)),
				10, 6,
			)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("EXTRACT(SECOND FROM ?)", expr)
		},
	})
}

func (b *QueryExprBuilder) DateTrunc(unit DateTimeUnit, expr any) schema.QueryAppender {
	precision := unit.ForDateTrunc()

	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("DATE_TRUNC(?, ?)", precision, expr)
		},
		MySQL: func() schema.QueryAppender {
			switch unit {
			case UnitYear:
				return b.Expr("DATE_FORMAT(?, ?)", expr, "%Y-01-01")
			case UnitMonth:
				return b.Expr("DATE_FORMAT(?, ?)", expr, "%Y-%m-01")
			case UnitHour:
				return b.Expr("DATE_FORMAT(?, ?)", expr, "%Y-%m-%d %H:00:00")
			case UnitMinute:
				return b.Expr("DATE_FORMAT(?, ?)", expr, "%Y-%m-%d %H:%i:00")
			case UnitDay:
				fallthrough
			default:
				return b.Expr("DATE(?)", expr)
			}
		},
		SQLite: func() schema.QueryAppender {
			nExpr := b.sqliteNormalizeTS(expr)
			switch unit {
			case UnitYear:
				return b.Expr("STRFTIME(?, ?)", "%Y-01-01", nExpr)
			case UnitMonth:
				return b.Expr("STRFTIME(?, ?)", "%Y-%m-01", nExpr)
			case UnitHour:
				return b.Expr("STRFTIME(?, ?)", "%Y-%m-%d %H:00:00", nExpr)
			case UnitMinute:
				return b.Expr("STRFTIME(?, ?)", "%Y-%m-%d %H:%M:00", nExpr)
			case UnitDay:
				fallthrough
			default:
				return b.Expr("DATE(?)", nExpr)
			}
		},
		Default: func() schema.QueryAppender {
			return b.Expr("DATE_TRUNC(?, ?)", precision, expr)
		},
	})
}

func (b *QueryExprBuilder) DateAdd(expr, interval any, unit DateTimeUnit) schema.QueryAppender {
	// Dynamic expression (e.g., column reference): use multiplication-based approach
	// because PostgreSQL's INTERVAL '...' and SQLite's modifiers require string literals.
	if _, isDynamic := interval.(schema.QueryAppender); isDynamic {
		return b.dateArith("+", expr, interval, unit)
	}

	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("? + INTERVAL '? ?'", expr, interval, b.Expr(unit.ForPostgres()))
		},
		MySQL: func() schema.QueryAppender {
			return b.Expr("DATE_ADD(?, INTERVAL ? ?)", expr, interval, b.Expr(unit.ForMySQL()))
		},
		SQLite: func() schema.QueryAppender {
			return b.Expr("DATETIME(?, '+? ?')", b.sqliteNormalizeTS(expr), interval, b.Expr(unit.ForSQLite()))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("DATE_ADD(?, INTERVAL ? ?)", expr, interval, b.Expr(unit.String()))
		},
	})
}

func (b *QueryExprBuilder) DateSubtract(expr, interval any, unit DateTimeUnit) schema.QueryAppender {
	// Dynamic expression (e.g., column reference): use multiplication-based approach
	// because PostgreSQL's INTERVAL '...' and SQLite's modifiers require string literals.
	if _, isDynamic := interval.(schema.QueryAppender); isDynamic {
		return b.dateArith("-", expr, interval, unit)
	}

	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("? - INTERVAL '? ?'", expr, interval, b.Expr(unit.ForPostgres()))
		},
		MySQL: func() schema.QueryAppender {
			return b.Expr("DATE_SUB(?, INTERVAL ? ?)", expr, interval, b.Expr(unit.ForMySQL()))
		},
		SQLite: func() schema.QueryAppender {
			return b.Expr("DATETIME(?, '-? ?')", b.sqliteNormalizeTS(expr), interval, b.Expr(unit.ForSQLite()))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("DATE_SUB(?, INTERVAL ? ?)", expr, interval, b.Expr(unit.String()))
		},
	})
}

// dateArith handles date add/subtract when interval is a dynamic expression (column, subquery, etc.).
//   - PostgreSQL: expr +/- interval * INTERVAL '1 unit'
//   - MySQL: DATE_ADD/DATE_SUB(expr, INTERVAL interval unit) — natively supports expressions
//   - SQLite: DATETIME(expr, '+'/'-' || interval || ' unit') — modifier built via string concatenation
func (b *QueryExprBuilder) dateArith(op string, expr, interval any, unit DateTimeUnit) schema.QueryAppender {
	unitInterval := b.Multiply(interval, b.Expr("INTERVAL '1 ?'", b.Expr(unit.ForPostgres())))

	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("? ? ?", expr, b.Expr(op), unitInterval)
		},
		MySQL: func() schema.QueryAppender {
			fn := "DATE_ADD"
			if op == "-" {
				fn = "DATE_SUB"
			}

			return b.Expr(fn+"(?, INTERVAL ? ?)", expr, interval, b.Expr(unit.ForMySQL()))
		},
		SQLite: func() schema.QueryAppender {
			modifier := b.Concat(op, b.Paren(interval), " ", unit.ForSQLite())

			return b.Expr("DATETIME(?, ?)", b.sqliteNormalizeTS(expr), modifier)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("? ? ?", expr, b.Expr(op), unitInterval)
		},
	})
}

func (b *QueryExprBuilder) DateDiff(start, end any, unit DateTimeUnit) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			// PostgreSQL: use EXTRACT(EPOCH FROM ...) for accurate calculations
			// EPOCH returns total seconds, then divide by unit
			epochDiff := b.Expr("EXTRACT(EPOCH FROM ?)", b.Subtract(end, start))

			switch unit {
			case UnitSecond:
				return epochDiff
			case UnitMinute:
				return b.Divide(epochDiff, 60)
			case UnitHour:
				return b.Divide(epochDiff, 3600)
			case UnitMonth:
				// Approximate: 30.44 days per month on average
				return b.Divide(epochDiff, 2629800)
			case UnitYear:
				// Approximate: 365.25 days per year on average
				return b.Divide(epochDiff, 31557600)
			case UnitDay:
				fallthrough
			default:
				return b.Divide(epochDiff, 86400) // default to days
			}
		},
		MySQL: func() schema.QueryAppender {
			switch unit {
			case UnitDay:
				return b.ToDecimal(b.Expr("DATEDIFF(?, ?)", end, start), 20, 6)
			default:
				return b.ToDecimal(b.Expr("TIMESTAMPDIFF(?, ?, ?)", b.Expr(unit.ForMySQL()), start, end), 20, 6)
			}
		},
		SQLite: func() schema.QueryAppender {
			// Compute Julian day difference once; scale by unit-specific factor
			dayDiff := b.Subtract(
				b.Expr("JULIANDAY(?)", b.sqliteNormalizeTS(end)),
				b.Expr("JULIANDAY(?)", b.sqliteNormalizeTS(start)),
			)

			switch unit {
			case UnitSecond:
				return b.Multiply(b.Paren(dayDiff), 86400)
			case UnitMinute:
				return b.Multiply(b.Paren(dayDiff), 1440)
			case UnitHour:
				return b.Multiply(b.Paren(dayDiff), 24)
			case UnitMonth:
				return b.Divide(b.Paren(dayDiff), 30.44)
			case UnitYear:
				return b.Divide(b.Paren(dayDiff), 365.25)
			case UnitDay:
				fallthrough
			default:
				return dayDiff
			}
		},
		Default: func() schema.QueryAppender {
			return b.Expr("DATEDIFF(?, ?, ?)", b.Expr(unit.String()), end, start)
		},
	})
}

// Age returns the age (interval) between two timestamps.
// Returns a PostgreSQL-compatible interval string in format: "X years Y mons Z days"
// This provides a symbolic result using field-by-field subtraction with adjustments.
func (b *QueryExprBuilder) Age(start, end any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			// PostgreSQL's AGE() returns simplified format for small intervals (e.g., "1 mon" instead of "0 years 1 mons 0 days")
			// We need to ensure consistent formatting by extracting years, months, and days components explicitly
			ageInterval := b.Expr("AGE(?, ?)", end, start)

			years := b.ExtractYear(ageInterval)
			months := b.ExtractMonth(ageInterval)
			days := b.ExtractDay(ageInterval)

			return b.Concat(
				b.ToString(years), " years ",
				b.ToString(months), " mons ",
				b.ToString(days), " days",
			)
		},
		MySQL: func() schema.QueryAppender {
			years := b.ToInteger(b.DateDiff(start, end, UnitYear))
			// Dynamic value, can't use DateAdd
			startPlusYears := b.DateAdd(start, years, UnitYear)
			// Calculate remaining months
			months := b.ToInteger(b.DateDiff(startPlusYears, end, UnitMonth))
			// Dynamic value, can't use DateAdd
			startPlusYearsMonths := b.DateAdd(startPlusYears, months, UnitMonth)
			// Calculate remaining days
			days := b.ToInteger(b.DateDiff(startPlusYearsMonths, end, UnitDay))

			return b.Concat(
				years, " years ",
				months, " mons ",
				days, " days",
			)
		},
		SQLite: func() schema.QueryAppender {
			// SQLite doesn't have AGE function, so we emulate it
			nEnd := b.sqliteNormalizeTS(end)

			approxYears := b.Subtract(
				b.ExtractYear(end),
				b.ExtractYear(start),
			)

			startPlusYears := b.DateAdd(start, approxYears, UnitYear)
			actualYears := b.Case(func(cb CaseBuilder) {
				cb.WhenExpr(b.GreaterThan(startPlusYears, nEnd)).
					Then(b.Subtract(approxYears, 1)).
					Else(approxYears)
			})
			startPlusActualYears := b.DateAdd(start, actualYears, UnitYear)

			approxMonths := b.Add(
				b.Multiply(
					b.Paren(b.Subtract(
						b.ExtractYear(end),
						b.ExtractYear(startPlusActualYears),
					)),
					12,
				),
				b.Subtract(
					b.ExtractMonth(end),
					b.ExtractMonth(startPlusActualYears),
				),
			)

			startPlusYearsMonths := b.DateAdd(startPlusActualYears, approxMonths, UnitMonth)
			actualMonths := b.Case(func(cb CaseBuilder) {
				cb.WhenExpr(b.GreaterThan(startPlusYearsMonths, nEnd)).
					Then(b.Subtract(approxMonths, 1)).
					Else(approxMonths)
			})
			startPlusActualYearsMonths := b.DateAdd(
				b.DateAdd(start, actualYears, UnitYear),
				actualMonths,
				UnitMonth,
			)

			days := b.Subtract(
				b.Expr("JULIANDAY(?)", nEnd),
				b.Expr("JULIANDAY(?)", startPlusActualYearsMonths),
			)

			return b.Concat(
				b.ToString(actualYears), " years ",
				b.ToString(actualMonths), " mons ",
				b.ToString(days), " days",
			)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("AGE(?, ?)", end, start)
		},
	})
}

// ========== Math Functions ==========

func (b *QueryExprBuilder) Abs(expr any) schema.QueryAppender {
	return b.Expr("ABS(?)", expr)
}

func (b *QueryExprBuilder) Ceil(expr any) schema.QueryAppender {
	return b.Expr("CEIL(?)", expr)
}

func (b *QueryExprBuilder) Floor(expr any) schema.QueryAppender {
	return b.Expr("FLOOR(?)", expr)
}

func (b *QueryExprBuilder) Round(expr any, precision ...any) schema.QueryAppender {
	if len(precision) > 0 {
		return b.Expr("ROUND(?, ?)", expr, precision[0])
	}

	return b.Expr("ROUND(?)", expr)
}

func (b *QueryExprBuilder) Trunc(expr any, precision ...any) schema.QueryAppender {
	hasPrecision := len(precision) > 0

	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			if hasPrecision {
				return b.Round(expr, precision[0])
			}

			return b.ToInteger(expr)
		},
		MySQL: func() schema.QueryAppender {
			if hasPrecision {
				return b.Expr("TRUNCATE(?, ?)", expr, precision[0])
			}

			return b.Expr("TRUNCATE(?, 0)", expr)
		},
		Default: func() schema.QueryAppender {
			if hasPrecision {
				return b.Expr("TRUNC(?, ?)", expr, precision[0])
			}

			return b.Expr("TRUNC(?)", expr)
		},
	})
}

func (b *QueryExprBuilder) Power(base, exponent any) schema.QueryAppender {
	return b.Expr("POWER(?, ?)", base, exponent)
}

func (b *QueryExprBuilder) Sqrt(expr any) schema.QueryAppender {
	return b.Expr("SQRT(?)", expr)
}

func (b *QueryExprBuilder) Exp(expr any) schema.QueryAppender {
	return b.Expr("EXP(?)", expr)
}

func (b *QueryExprBuilder) Ln(expr any) schema.QueryAppender {
	return b.Expr("LN(?)", expr)
}

func (b *QueryExprBuilder) Log(expr any, base ...any) schema.QueryAppender {
	if len(base) > 0 {
		return b.Expr("LOG(?, ?)", base[0], expr)
	}

	return b.Expr("LOG(?)", expr)
}

func (b *QueryExprBuilder) Sin(expr any) schema.QueryAppender {
	return b.Expr("SIN(?)", expr)
}

func (b *QueryExprBuilder) Cos(expr any) schema.QueryAppender {
	return b.Expr("COS(?)", expr)
}

func (b *QueryExprBuilder) Tan(expr any) schema.QueryAppender {
	return b.Expr("TAN(?)", expr)
}

func (b *QueryExprBuilder) Asin(expr any) schema.QueryAppender {
	return b.Expr("ASIN(?)", expr)
}

func (b *QueryExprBuilder) Acos(expr any) schema.QueryAppender {
	return b.Expr("ACOS(?)", expr)
}

func (b *QueryExprBuilder) Atan(expr any) schema.QueryAppender {
	return b.Expr("ATAN(?)", expr)
}

func (b *QueryExprBuilder) Pi() schema.QueryAppender {
	return b.Expr("PI()")
}

func (b *QueryExprBuilder) Random() schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Abs(
				b.Divide(
					b.Multiply(b.Expr("RANDOM()"), 1.0),
					9223372036854775808.0,
				),
			)
		},
		MySQL: func() schema.QueryAppender {
			return b.Expr("RAND()")
		},
		Default: func() schema.QueryAppender {
			return b.Expr("RANDOM()")
		},
	})
}

func (b *QueryExprBuilder) Sign(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.ToFloat(
				b.Case(func(cb CaseBuilder) {
					cb.WhenExpr(b.GreaterThan(expr, 0)).Then(1).
						WhenExpr(b.LessThan(expr, 0)).Then(-1).
						Else(0)
				}),
			)
		},
		MySQL: func() schema.QueryAppender {
			return b.ToDecimal(b.Expr("SIGN(?)", expr), 2, 1)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("SIGN(?)", expr)
		},
	})
}

func (b *QueryExprBuilder) Mod(dividend, divisor any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("? % ?", dividend, divisor)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("MOD(?, ?)", dividend, divisor)
		},
	})
}

func (b *QueryExprBuilder) Greatest(args ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("MAX(?)", newExpressions(", ", args...))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("GREATEST(?)", newExpressions(", ", args...))
		},
	})
}

func (b *QueryExprBuilder) Least(args ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		SQLite: func() schema.QueryAppender {
			return b.Expr("MIN(?)", newExpressions(", ", args...))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("LEAST(?)", newExpressions(", ", args...))
		},
	})
}

// ========== Conditional Functions ==========

func (b *QueryExprBuilder) Coalesce(args ...any) schema.QueryAppender {
	return b.Expr("COALESCE(?)", newExpressions(", ", args...))
}

func (b *QueryExprBuilder) NullIf(expr1, expr2 any) schema.QueryAppender {
	return b.Expr("NULLIF(?, ?)", expr1, expr2)
}

func (b *QueryExprBuilder) IfNull(expr, defaultValue any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Coalesce(expr, defaultValue)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("IFNULL(?, ?)", expr, defaultValue)
		},
	})
}

// ========== Type Conversion Functions ==========

func (b *QueryExprBuilder) ToString(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("?::TEXT", b.Paren(expr))
		},
		MySQL: func() schema.QueryAppender {
			return b.Expr("CAST(? AS CHAR)", expr)
		},
		SQLite: func() schema.QueryAppender {
			return b.Expr("CAST(? AS TEXT)", expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CAST(? AS VARCHAR)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToInteger(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("?::INTEGER", b.Paren(expr))
		},
		MySQL: func() schema.QueryAppender {
			return b.Expr("CAST(? AS SIGNED INTEGER)", expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CAST(? AS INTEGER)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToDecimal(expr any, precision ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			switch len(precision) {
			case 0:
				return b.Expr("?::NUMERIC", b.Paren(expr))
			case 1:
				return b.Expr("?::NUMERIC(?)", b.Paren(expr), precision[0])
			default:
				return b.Expr("?::NUMERIC(?, ?)", b.Paren(expr), precision[0], precision[1])
			}
		},
		SQLite: func() schema.QueryAppender {
			return b.Expr("CAST(? AS REAL)", expr)
		},
		Default: func() schema.QueryAppender {
			switch len(precision) {
			case 0:
				return b.Expr("CAST(? AS DECIMAL)", expr)
			case 1:
				return b.Expr("CAST(? AS DECIMAL(?))", expr, precision[0])
			default:
				return b.Expr("CAST(? AS DECIMAL(?, ?))", expr, precision[0], precision[1])
			}
		},
	})
}

func (b *QueryExprBuilder) ToFloat(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("?::DOUBLE PRECISION", b.Paren(expr))
		},
		SQLite: func() schema.QueryAppender {
			return b.Expr("CAST(? AS REAL)", expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CAST(? AS DOUBLE)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToBool(expr any) schema.QueryAppender {
	intNotZero := func() schema.QueryAppender {
		return b.NotEquals(b.ToInteger(expr), 0)
	}

	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("?::BOOLEAN", b.Paren(expr))
		},
		MySQL:  intNotZero,
		SQLite: intNotZero,
		Default: func() schema.QueryAppender {
			return b.Expr("CAST(? AS BOOLEAN)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToDate(expr any, format ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TO_DATE(?, ?)", expr, format[0])
			}

			return b.Expr("?::DATE", b.Paren(expr))
		},
		MySQL: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("STR_TO_DATE(?, ?)", expr, format[0])
			}

			return b.Expr("CAST(? AS DATE)", expr)
		},
		SQLite: func() schema.QueryAppender {
			nExpr := b.sqliteNormalizeTS(expr)
			if len(format) > 0 {
				return b.Expr("STRFTIME(?, ?)", "%Y-%m-%d", nExpr)
			}

			return b.Expr("DATE(?)", nExpr)
		},
		Default: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TO_DATE(?, ?)", expr, format[0])
			}

			return b.Expr("CAST(? AS DATE)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToTime(expr any, format ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TO_TIMESTAMP(?, ?)::TIME", expr, format[0])
			}

			return b.Expr("?::TIME", b.Paren(expr))
		},
		MySQL: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TIME(STR_TO_DATE(?, ?))", expr, format[0])
			}

			return b.Expr("CAST(? AS TIME)", expr)
		},
		SQLite: func() schema.QueryAppender {
			nExpr := b.sqliteNormalizeTS(expr)
			if len(format) > 0 {
				return b.Expr("STRFTIME(?, ?)", "%H:%M:%S", nExpr)
			}

			return b.Expr("TIME(?)", nExpr)
		},
		Default: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TO_TIME(?, ?)", expr, format[0])
			}

			return b.Expr("CAST(? AS TIME)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToTimestamp(expr any, format ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TO_TIMESTAMP(?, ?)", expr, format[0])
			}

			return b.Expr("?::TIMESTAMP", b.Paren(expr))
		},
		MySQL: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("STR_TO_DATE(?, ?)", expr, format[0])
			}

			return b.Expr("CAST(? AS DATETIME)", expr)
		},
		SQLite: func() schema.QueryAppender {
			nExpr := b.sqliteNormalizeTS(expr)
			if len(format) > 0 {
				return b.Expr("STRFTIME(?, ?)", "%Y-%m-%d %H:%M:%S", nExpr)
			}

			return b.Expr("DATETIME(?)", nExpr)
		},
		Default: func() schema.QueryAppender {
			if len(format) > 0 {
				return b.Expr("TO_TIMESTAMP(?, ?)", expr, format[0])
			}

			return b.Expr("CAST(? AS TIMESTAMP)", expr)
		},
	})
}

func (b *QueryExprBuilder) ToJSON(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("?::JSONB", b.Paren(expr))
		},
		MySQL: func() schema.QueryAppender {
			return b.Expr("CAST(? AS JSON)", expr)
		},
		SQLite: func() schema.QueryAppender {
			return b.Expr("?", expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("CAST(? AS JSON)", expr)
		},
	})
}

// ========== JSON Functions ==========

// processJSONPath formats the JSON path according to the dialect requirements.
// Input path is expected to be in "key1.key2" format (dot-separated keys).
func (b *QueryExprBuilder) processJSONPath(path any, dialectName dialect.Name) any {
	switch dialectName {
	case dialect.PG:
		if pathStr, ok := path.(string); ok {
			// Split by dot and join with comma
			parts := strings.Split(pathStr, ".")

			return "{" + strings.Join(parts, ",") + "}"
		}
		// For expressions, we replace . with , and wrap in {}
		// Note: This assumes the expression evaluates to a dot-separated string
		return b.Concat("{", b.Replace(path, ".", ","), "}")

	default: // MySQL, SQLite
		if pathStr, ok := path.(string); ok {
			if len(pathStr) == 0 {
				return "$"
			}

			return "$." + pathStr
		}

		return b.Concat("$.", path)
	}
}

// JSONExtract extracts value from JSON at specified path.
//
// Note: For PostgreSQL, this uses the #>> operator which returns the value as text (unquoted).
// This is compatible with b.ToJSON() which will correctly cast the text back to JSONB
// if the result is used in subsequent JSON functions (e.g. chaining JSONExtract).
func (b *QueryExprBuilder) JSONExtract(json, path any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("? #>> ?::text[]", b.ToJSON(json), b.processJSONPath(path, dialect.PG))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_EXTRACT(?, ?)", json, b.processJSONPath(path, dialect.MySQL))
		},
	})
}

func (b *QueryExprBuilder) JSONUnquote(expr any) schema.QueryAppender {
	// PostgreSQL #>> already returns unquoted text; SQLite JSON_EXTRACT returns unquoted scalars.
	// Only MySQL requires explicit JSON_UNQUOTE.
	return b.ExprByDialect(DialectExprs{
		MySQL: func() schema.QueryAppender {
			return b.Expr("JSON_UNQUOTE(?)", expr)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("?", expr)
		},
	})
}

func (b *QueryExprBuilder) JSONArray(args ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(args) == 0 {
				return b.ToJSON("[]")
			}

			return b.Expr("JSONB_BUILD_ARRAY(?)", b.Exprs(args...))
		},
		Default: func() schema.QueryAppender {
			if len(args) == 0 {
				return b.Expr("JSON_ARRAY()")
			}

			return b.Expr("JSON_ARRAY(?)", b.Exprs(args...))
		},
	})
}

func (b *QueryExprBuilder) JSONObject(keyValues ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(keyValues) == 0 {
				return b.ToJSON("{}")
			}

			return b.Expr("JSONB_BUILD_OBJECT(?)", b.Exprs(keyValues...))
		},
		Default: func() schema.QueryAppender {
			if len(keyValues) == 0 {
				return b.Expr("JSON_OBJECT()")
			}

			return b.Expr("JSON_OBJECT(?)", b.Exprs(keyValues...))
		},
	})
}

func (b *QueryExprBuilder) JSONContains(json, value any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("? @> ?", json, b.ToJSON(value))
		},
		SQLite: func() schema.QueryAppender {
			return b.Exists(func(sq SelectQuery) {
				sq.SelectExpr(func(eb ExprBuilder) any { return eb.Literal(1) }).
					Where(func(cb ConditionBuilder) {
						cb.Expr(func(eb ExprBuilder) any {
							return eb.Equals(
								eb.JSONExtract(json, "$[*]"),
								value,
							)
						})
					})
			})
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_CONTAINS(?, ?)", json, value)
		},
	})
}

func (b *QueryExprBuilder) JSONContainsPath(json, path any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			// PostgreSQL uses jsonb_path_exists which requires JSONPath syntax ($.key)
			return b.Expr("JSONB_PATH_EXISTS(?, ?::jsonpath)", b.ToJSON(json), b.processJSONPath(path, dialect.MySQL))
		},
		SQLite: func() schema.QueryAppender {
			return b.IsNotNull(b.JSONExtract(json, path))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_CONTAINS_PATH(?, ?, ?)", json, "one", b.processJSONPath(path, dialect.MySQL))
		},
	})
}

func (b *QueryExprBuilder) JSONKeys(json any, path ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(path) > 0 {
				return b.Expr("JSONB_OBJECT_KEYS(? #> ?::text[])", b.ToJSON(json), b.processJSONPath(path[0], dialect.PG))
			}

			return b.Expr("JSONB_OBJECT_KEYS(?)", b.ToJSON(json))
		},
		SQLite: func() schema.QueryAppender {
			buildKeyArray := func(source any) schema.QueryAppender {
				return b.SubQuery(func(sq SelectQuery) {
					sq.TableExpr(
						func(eb ExprBuilder) any {
							return eb.Expr("JSON_EACH(?)", source)
						}).
						SelectExpr(func(eb ExprBuilder) any {
							return eb.Expr("JSON_GROUP_ARRAY(key)")
						})
				})
			}

			if len(path) > 0 {
				return buildKeyArray(b.JSONExtract(json, path[0]))
			}

			return buildKeyArray(json)
		},
		Default: func() schema.QueryAppender {
			if len(path) > 0 {
				return b.Expr("JSON_KEYS(?, ?)", json, b.processJSONPath(path[0], dialect.MySQL))
			}

			return b.Expr("JSON_KEYS(?)", json)
		},
	})
}

func (b *QueryExprBuilder) JSONLength(json any, path ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			var jsonExpr schema.QueryAppender
			if len(path) > 0 {
				jsonExpr = b.Expr("? #> ?::text[]", b.ToJSON(json), b.processJSONPath(path[0], dialect.PG))
			} else {
				jsonExpr = b.ToJSON(json)
			}

			return b.Case(func(cb CaseBuilder) {
				cb.Case(b.Expr("JSONB_TYPEOF(?)", jsonExpr)).
					WhenExpr("array").
					Then(b.Expr("JSONB_ARRAY_LENGTH(?)", jsonExpr)).
					WhenExpr("object").
					ThenSubQuery(func(query SelectQuery) {
						query.SelectExpr(func(eb ExprBuilder) any { return eb.CountAll() }).
							TableExpr(func(eb ExprBuilder) any { return eb.Expr("JSONB_OBJECT_KEYS(?)", jsonExpr) })
					}).
					Else(0)
			})
		},
		SQLite: func() schema.QueryAppender {
			valueExpr := json
			if len(path) > 0 {
				valueExpr = b.JSONExtract(json, path[0])
			}

			return b.Case(func(cb CaseBuilder) {
				cb.Case(b.JSONType(json, path...)).
					WhenExpr("array").
					Then(b.Expr("JSON_ARRAY_LENGTH(?)", valueExpr)).
					WhenExpr("object").
					ThenSubQuery(func(query SelectQuery) {
						query.SelectExpr(func(eb ExprBuilder) any { return eb.CountAll() }).
							TableExpr(func(eb ExprBuilder) any {
								return eb.Expr("JSON_EACH(?)", valueExpr)
							})
					}).
					Else(0)
			})
		},
		Default: func() schema.QueryAppender {
			if len(path) > 0 {
				return b.Expr("JSON_LENGTH(?, ?)", json, b.processJSONPath(path[0], dialect.MySQL))
			}

			return b.Expr("JSON_LENGTH(?)", json)
		},
	})
}

func (b *QueryExprBuilder) JSONType(json any, path ...any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if len(path) > 0 {
				return b.Expr("JSONB_TYPEOF(? #> ?::text[])", b.ToJSON(json), b.processJSONPath(path[0], dialect.PG))
			}

			return b.Expr("JSONB_TYPEOF(?)", b.ToJSON(json))
		},
		Default: func() schema.QueryAppender {
			if len(path) > 0 {
				return b.Expr("JSON_TYPE(?, ?)", json, b.processJSONPath(path[0], dialect.MySQL))
			}

			return b.Expr("JSON_TYPE(?)", json)
		},
	})
}

func (b *QueryExprBuilder) JSONValid(expr any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("? AND ?", b.IsNotNull(expr), b.IsNotNull(b.ToJSON(b.ToString(expr))))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_VALID(?)", expr)
		},
	})
}

func (b *QueryExprBuilder) JSONSet(json, path, value any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("JSONB_SET(?, ?::text[], ?, TRUE)", b.ToJSON(json), b.processJSONPath(path, dialect.PG), b.ToJSON(value))
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_SET(?, ?, ?)", json, b.processJSONPath(path, dialect.MySQL), value)
		},
	})
}

func (b *QueryExprBuilder) JSONInsert(json, path, value any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("JSONB_INSERT(?, ?::text[], TO_JSONB(?), FALSE)", b.ToJSON(json), b.processJSONPath(path, dialect.PG), value)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_INSERT(?, ?, ?)", json, b.processJSONPath(path, dialect.MySQL), value)
		},
	})
}

func (b *QueryExprBuilder) JSONReplace(json, path, value any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return b.Expr("JSONB_SET(?, ?::text[], TO_JSONB(?), FALSE)", b.ToJSON(json), b.processJSONPath(path, dialect.PG), value)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_REPLACE(?, ?, ?)", json, b.processJSONPath(path, dialect.MySQL), value)
		},
	})
}

func (b *QueryExprBuilder) JSONArrayAppend(json, path, value any) schema.QueryAppender {
	return b.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			if pathStr, ok := path.(string); ok && pathStr == "$" {
				return b.Expr("(? || ?)", b.ToJSON(json), b.JSONArray(value))
			}

			pgPath := b.processJSONPath(path, dialect.PG)

			return b.Expr(
				"JSONB_SET(?, ?::text[], (? || ?))",
				b.ToJSON(json),
				pgPath,
				b.Coalesce(
					b.Expr("? #> ?::text[]", b.ToJSON(json), pgPath),
					b.ToJSON("[]"),
				),
				b.JSONArray(value),
			)
		},
		SQLite: func() schema.QueryAppender {
			return b.JSONSet(
				json,
				path,
				b.JSONInsert(
					b.Coalesce(
						b.JSONExtract(json, path),
						"[]",
					),
					"$[#]",
					value,
				),
			)
		},
		Default: func() schema.QueryAppender {
			return b.Expr("JSON_ARRAY_APPEND(?, ?, ?)", json, b.processJSONPath(path, dialect.MySQL), value)
		},
	})
}

// ========== Utility Functions ==========

func (b *QueryExprBuilder) Decode(args ...any) schema.QueryAppender {
	if len(args) < 3 {
		return b.Null()
	}

	// Most databases don't support DECODE natively; Oracle does.
	// For PostgreSQL, MySQL, SQLite: convert to CASE WHEN expression.
	return b.ExprByDialect(DialectExprs{
		Oracle: func() schema.QueryAppender {
			return b.Expr("DECODE(?)", newExpressions(", ", args...))
		},
		Default: func() schema.QueryAppender {
			return b.convertDecodeToCase(args...)
		},
	})
}

// convertDecodeToCase converts DECODE syntax to CASE WHEN expression using the existing Case builder.
func (b *QueryExprBuilder) convertDecodeToCase(args ...any) schema.QueryAppender {
	if len(args) < 3 {
		return b.Null()
	}

	return b.Case(func(cb CaseBuilder) {
		cb.Case(args[0])

		i := 1
		for i+1 < len(args) {
			search := args[i]
			result := args[i+1]
			cb.WhenExpr(search).Then(result)

			i += 2
		}

		if i < len(args) {
			cb.Else(args[i])
		}
	})
}
