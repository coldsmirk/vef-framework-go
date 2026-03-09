package orm

import (
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

type CriteriaBuilder struct {
	qb    QueryBuilder
	eb    ExprBuilder
	and   func(query string, args ...any)
	or    func(query string, args ...any)
	group func(sep string, builder func(ConditionBuilder))
}

func (cb *CriteriaBuilder) Apply(fns ...ApplyFunc[ConditionBuilder]) ConditionBuilder {
	for _, fn := range fns {
		if fn != nil {
			fn(cb)
		}
	}

	return cb
}

func (cb *CriteriaBuilder) ApplyIf(condition bool, fns ...ApplyFunc[ConditionBuilder]) ConditionBuilder {
	if condition {
		return cb.Apply(fns...)
	}

	return cb
}

func (cb *CriteriaBuilder) Equals(column string, value any) ConditionBuilder {
	cb.and("? = ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) OrEquals(column string, value any) ConditionBuilder {
	cb.or("? = ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) EqualsColumn(column1, column2 string) ConditionBuilder {
	cb.and("? = ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) OrEqualsColumn(column1, column2 string) ConditionBuilder {
	cb.or("? = ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) EqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? = (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrEqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? = (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) EqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? = ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrEqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? = ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) EqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? = ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrEqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? = ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) EqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? = ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrEqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? = ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) NotEquals(column string, value any) ConditionBuilder {
	cb.and("? <> ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) OrNotEquals(column string, value any) ConditionBuilder {
	cb.or("? <> ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) NotEqualsColumn(column1, column2 string) ConditionBuilder {
	cb.and("? <> ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) OrNotEqualsColumn(column1, column2 string) ConditionBuilder {
	cb.or("? <> ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) NotEqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? <> (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrNotEqualsSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? <> (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) NotEqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? <> ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrNotEqualsAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? <> ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) NotEqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? <> ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrNotEqualsAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? <> ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) NotEqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? <> ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrNotEqualsExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? <> ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) GreaterThan(column string, value any) ConditionBuilder {
	cb.and("? > ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThan(column string, value any) ConditionBuilder {
	cb.or("? > ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) GreaterThanColumn(column1, column2 string) ConditionBuilder {
	cb.and("? > ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanColumn(column1, column2 string) ConditionBuilder {
	cb.or("? > ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? > (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? > (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? > ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? > ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? > ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? > ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? > ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? > ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanOrEqual(column string, value any) ConditionBuilder {
	cb.and("? >= ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanOrEqual(column string, value any) ConditionBuilder {
	cb.or("? >= ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) GreaterThanOrEqualColumn(column1, column2 string) ConditionBuilder {
	cb.and("? >= ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanOrEqualColumn(column1, column2 string) ConditionBuilder {
	cb.or("? >= ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? >= (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? >= (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? >= ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? >= ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? >= ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? >= ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) GreaterThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? >= ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrGreaterThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? >= ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) LessThan(column string, value any) ConditionBuilder {
	cb.and("? < ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) OrLessThan(column string, value any) ConditionBuilder {
	cb.or("? < ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) LessThanColumn(column1, column2 string) ConditionBuilder {
	cb.and("? < ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanColumn(column1, column2 string) ConditionBuilder {
	cb.or("? < ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) LessThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? < (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? < (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) LessThanAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? < ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? < ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) LessThanAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? < ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? < ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) LessThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? < ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? < ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) LessThanOrEqual(column string, value any) ConditionBuilder {
	cb.and("? <= ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) OrLessThanOrEqual(column string, value any) ConditionBuilder {
	cb.or("? <= ?", cb.eb.Column(column), value)

	return cb
}

func (cb *CriteriaBuilder) LessThanOrEqualColumn(column1, column2 string) ConditionBuilder {
	cb.and("? <= ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanOrEqualColumn(column1, column2 string) ConditionBuilder {
	cb.or("? <= ?", cb.eb.Column(column1), cb.eb.Column(column2))

	return cb
}

func (cb *CriteriaBuilder) LessThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? <= (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanOrEqualSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? <= (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) LessThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? <= ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanOrEqualAny(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? <= ANY (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) LessThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? <= ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanOrEqualAll(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? <= ALL (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) LessThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? <= ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrLessThanOrEqualExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? <= ?", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) Between(column string, start, end any) ConditionBuilder {
	cb.and("? BETWEEN ? AND ?", cb.eb.Column(column), start, end)

	return cb
}

func (cb *CriteriaBuilder) OrBetween(column string, start, end any) ConditionBuilder {
	cb.or("? BETWEEN ? AND ?", cb.eb.Column(column), start, end)

	return cb
}

func (cb *CriteriaBuilder) BetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder {
	cb.and("? BETWEEN ? AND ?", cb.eb.Column(column), startB(cb.eb), endB(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrBetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder {
	cb.or("? BETWEEN ? AND ?", cb.eb.Column(column), startB(cb.eb), endB(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) NotBetween(column string, start, end any) ConditionBuilder {
	cb.and("? NOT BETWEEN ? AND ?", cb.eb.Column(column), start, end)

	return cb
}

func (cb *CriteriaBuilder) OrNotBetween(column string, start, end any) ConditionBuilder {
	cb.or("? NOT BETWEEN ? AND ?", cb.eb.Column(column), start, end)

	return cb
}

func (cb *CriteriaBuilder) NotBetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder {
	cb.and("? NOT BETWEEN ? AND ?", cb.eb.Column(column), startB(cb.eb), endB(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrNotBetweenExpr(column string, startB, endB func(ExprBuilder) any) ConditionBuilder {
	cb.or("? NOT BETWEEN ? AND ?", cb.eb.Column(column), startB(cb.eb), endB(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) In(column string, values any) ConditionBuilder {
	cb.and("? IN ?", cb.eb.Column(column), bun.Tuple(values))

	return cb
}

func (cb *CriteriaBuilder) OrIn(column string, values any) ConditionBuilder {
	cb.or("? IN ?", cb.eb.Column(column), bun.Tuple(values))

	return cb
}

func (cb *CriteriaBuilder) InSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? IN (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrInSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? IN (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) InExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? IN (?)", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrInExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? IN (?)", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) NotIn(column string, values any) ConditionBuilder {
	cb.and("? NOT IN ?", cb.eb.Column(column), bun.Tuple(values))

	return cb
}

func (cb *CriteriaBuilder) OrNotIn(column string, values any) ConditionBuilder {
	cb.or("? NOT IN ?", cb.eb.Column(column), bun.Tuple(values))

	return cb
}

func (cb *CriteriaBuilder) NotInSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.and("? NOT IN (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrNotInSubQuery(column string, builder func(query SelectQuery)) ConditionBuilder {
	cb.or("? NOT IN (?)", cb.eb.Column(column), cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) NotInExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("? NOT IN (?)", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrNotInExpr(column string, builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("? NOT IN (?)", cb.eb.Column(column), builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) IsNull(column string) ConditionBuilder {
	cb.and("? IS NULL", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) OrIsNull(column string) ConditionBuilder {
	cb.or("? IS NULL", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) IsNullSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.and("(?) IS NULL", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrIsNullSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.or("(?) IS NULL", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) IsNullExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("(?) IS NULL", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrIsNullExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("(?) IS NULL", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) IsNotNull(column string) ConditionBuilder {
	cb.and("? IS NOT NULL", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) OrIsNotNull(column string) ConditionBuilder {
	cb.or("? IS NOT NULL", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) IsNotNullSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.and("(?) IS NOT NULL", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrIsNotNullSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.or("(?) IS NOT NULL", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) IsNotNullExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("(?) IS NOT NULL", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrIsNotNullExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("(?) IS NOT NULL", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) IsTrue(column string) ConditionBuilder {
	cb.and("? IS TRUE", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) OrIsTrue(column string) ConditionBuilder {
	cb.or("? IS TRUE", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) IsTrueSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.and("(?) IS TRUE", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrIsTrueSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.or("(?) IS TRUE", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) IsTrueExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("(?) IS TRUE", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrIsTrueExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("(?) IS TRUE", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) IsFalse(column string) ConditionBuilder {
	cb.and("? IS FALSE", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) OrIsFalse(column string) ConditionBuilder {
	cb.or("? IS FALSE", cb.eb.Column(column))

	return cb
}

func (cb *CriteriaBuilder) IsFalseSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.and("(?) IS FALSE", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) OrIsFalseSubQuery(builder func(query SelectQuery)) ConditionBuilder {
	cb.or("(?) IS FALSE", cb.qb.BuildSubQuery(builder))

	return cb
}

func (cb *CriteriaBuilder) IsFalseExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("(?) IS FALSE", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrIsFalseExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("(?) IS FALSE", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) Contains(column, value string) ConditionBuilder {
	cb.and("? LIKE ?", cb.eb.Column(column), FuzzyContains.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) OrContains(column, value string) ConditionBuilder {
	cb.or("? LIKE ?", cb.eb.Column(column), FuzzyContains.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) ContainsAny(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrContains(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrContainsAny(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrContains(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) ContainsIgnoreCase(column, value string) ConditionBuilder {
	cb.and("?", cb.buildLikeIgnoreCase(column, FuzzyContains.BuildPattern(value), false))

	return cb
}

func (cb *CriteriaBuilder) OrContainsIgnoreCase(column, value string) ConditionBuilder {
	cb.or("?", cb.buildLikeIgnoreCase(column, FuzzyContains.BuildPattern(value), false))

	return cb
}

func (cb *CriteriaBuilder) ContainsAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrContainsIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrContainsAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrContainsIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) NotContains(column, value string) ConditionBuilder {
	cb.and("? NOT LIKE ?", cb.eb.Column(column), FuzzyContains.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) OrNotContains(column, value string) ConditionBuilder {
	cb.or("? NOT LIKE ?", cb.eb.Column(column), FuzzyContains.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) NotContainsAny(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotContains(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrNotContainsAny(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotContains(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) NotContainsIgnoreCase(column, value string) ConditionBuilder {
	cb.and("?", cb.buildLikeIgnoreCase(column, FuzzyContains.BuildPattern(value), true))

	return cb
}

func (cb *CriteriaBuilder) OrNotContainsIgnoreCase(column, value string) ConditionBuilder {
	cb.or("?", cb.buildLikeIgnoreCase(column, FuzzyContains.BuildPattern(value), true))

	return cb
}

func (cb *CriteriaBuilder) NotContainsAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotContainsIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrNotContainsAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotContainsIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) StartsWith(column, value string) ConditionBuilder {
	cb.and("? LIKE ?", cb.eb.Column(column), FuzzyStarts.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) OrStartsWith(column, value string) ConditionBuilder {
	cb.or("? LIKE ?", cb.eb.Column(column), FuzzyStarts.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) StartsWithAny(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrStartsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrStartsWithAny(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrStartsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) StartsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.and("?", cb.buildLikeIgnoreCase(column, FuzzyStarts.BuildPattern(value), false))

	return cb
}

func (cb *CriteriaBuilder) OrStartsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.or("?", cb.buildLikeIgnoreCase(column, FuzzyStarts.BuildPattern(value), false))

	return cb
}

func (cb *CriteriaBuilder) StartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrStartsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrStartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrStartsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) NotStartsWith(column, value string) ConditionBuilder {
	cb.and("? NOT LIKE ?", cb.eb.Column(column), FuzzyStarts.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) OrNotStartsWith(column, value string) ConditionBuilder {
	cb.or("? NOT LIKE ?", cb.eb.Column(column), FuzzyStarts.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) NotStartsWithAny(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotStartsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrNotStartsWithAny(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotStartsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) NotStartsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.and("?", cb.buildLikeIgnoreCase(column, FuzzyStarts.BuildPattern(value), true))

	return cb
}

func (cb *CriteriaBuilder) OrNotStartsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.or("?", cb.buildLikeIgnoreCase(column, FuzzyStarts.BuildPattern(value), true))

	return cb
}

func (cb *CriteriaBuilder) NotStartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotStartsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrNotStartsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotStartsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) EndsWith(column, value string) ConditionBuilder {
	cb.and("? LIKE ?", cb.eb.Column(column), FuzzyEnds.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) OrEndsWith(column, value string) ConditionBuilder {
	cb.or("? LIKE ?", cb.eb.Column(column), FuzzyEnds.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) EndsWithAny(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrEndsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrEndsWithAny(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrEndsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) EndsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.and("?", cb.buildLikeIgnoreCase(column, FuzzyEnds.BuildPattern(value), false))

	return cb
}

func (cb *CriteriaBuilder) OrEndsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.or("?", cb.buildLikeIgnoreCase(column, FuzzyEnds.BuildPattern(value), false))

	return cb
}

func (cb *CriteriaBuilder) EndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrEndsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrEndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.OrEndsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) NotEndsWith(column, value string) ConditionBuilder {
	cb.and("? NOT LIKE ?", cb.eb.Column(column), FuzzyEnds.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) OrNotEndsWith(column, value string) ConditionBuilder {
	cb.or("? NOT LIKE ?", cb.eb.Column(column), FuzzyEnds.BuildPattern(value))

	return cb
}

func (cb *CriteriaBuilder) NotEndsWithAny(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotEndsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrNotEndsWithAny(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotEndsWith(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) NotEndsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.and("?", cb.buildLikeIgnoreCase(column, FuzzyEnds.BuildPattern(value), true))

	return cb
}

func (cb *CriteriaBuilder) OrNotEndsWithIgnoreCase(column, value string) ConditionBuilder {
	cb.or("?", cb.buildLikeIgnoreCase(column, FuzzyEnds.BuildPattern(value), true))

	return cb
}

func (cb *CriteriaBuilder) NotEndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.Group(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotEndsWithIgnoreCase(column, value)
		}
	})

	return cb
}

func (cb *CriteriaBuilder) OrNotEndsWithAnyIgnoreCase(column string, values []string) ConditionBuilder {
	cb.OrGroup(func(cb ConditionBuilder) {
		for _, value := range values {
			cb.NotEndsWithIgnoreCase(column, value)
		}
	})

	return cb
}

// buildLikeIgnoreCase builds a dialect-aware case-insensitive LIKE/NOT LIKE expression.
// Uses ILIKE/NOT ILIKE on Postgres; falls back to LOWER(column) LIKE/NOT LIKE LOWER(pattern) on other dialects.
func (cb *CriteriaBuilder) buildLikeIgnoreCase(column string, pattern any, negate bool) schema.QueryAppender {
	pgOp, defaultOp := "ILIKE", "LIKE"
	if negate {
		pgOp, defaultOp = "NOT ILIKE", "NOT LIKE"
	}

	return cb.eb.ExprByDialect(DialectExprs{
		Postgres: func() schema.QueryAppender {
			return cb.eb.Expr("? "+pgOp+" ?", cb.eb.Column(column), pattern)
		},
		Default: func() schema.QueryAppender {
			return cb.eb.Expr("? "+defaultOp+" ?", cb.eb.Lower(cb.eb.Column(column)), cb.eb.Lower(pattern))
		},
	})
}

func (cb *CriteriaBuilder) Expr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.and("?", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) OrExpr(builder func(ExprBuilder) any) ConditionBuilder {
	cb.or("?", builder(cb.eb))

	return cb
}

func (cb *CriteriaBuilder) Group(builder func(ConditionBuilder)) ConditionBuilder {
	cb.group(separatorAnd, builder)

	return cb
}

func (cb *CriteriaBuilder) OrGroup(builder func(ConditionBuilder)) ConditionBuilder {
	cb.group(separatorOr, builder)

	return cb
}

// auditUserCompare adds a comparison condition for an audit user column.
func (*CriteriaBuilder) auditUserCompare(addFn func(string, ...any), op, col, value string, alias ...string) {
	addFn("? ? ?", buildColumnExpr(col, alias...), bun.Safe(op), value)
}

// auditUserCompareSubQuery adds a comparison condition with a subquery for an audit user column.
func (cb *CriteriaBuilder) auditUserCompareSubQuery(addFn func(string, ...any), op, col string, builder func(SelectQuery), alias ...string) {
	addFn("? ? (?)", buildColumnExpr(col, alias...), bun.Safe(op), cb.qb.BuildSubQuery(builder))
}

// auditUserCompareCurrent adds a comparison condition against the current operator for an audit user column.
func (*CriteriaBuilder) auditUserCompareCurrent(addFn func(string, ...any), op, col string, alias ...string) {
	addFn("? ? ?Operator", buildColumnExpr(col, alias...), bun.Safe(op))
}

// auditUserIn adds an IN/NOT IN condition for an audit user column.
func (*CriteriaBuilder) auditUserIn(addFn func(string, ...any), op, col string, values []string, alias ...string) {
	addFn("? ? ?", buildColumnExpr(col, alias...), bun.Safe(op), bun.Tuple(values))
}

// auditUserInSubQuery adds an IN/NOT IN subquery condition for an audit user column.
func (cb *CriteriaBuilder) auditUserInSubQuery(addFn func(string, ...any), op, col string, builder func(SelectQuery), alias ...string) {
	addFn("? ? (?)", buildColumnExpr(col, alias...), bun.Safe(op), cb.qb.BuildSubQuery(builder))
}

func (cb *CriteriaBuilder) CreatedByEquals(createdBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.and, "=", ColumnCreatedBy, createdBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByEquals(createdBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.or, "=", ColumnCreatedBy, createdBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "=", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "=", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "= ANY", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "= ANY", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "= ALL", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "= ALL", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.and, "=", ColumnCreatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.or, "=", ColumnCreatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotEquals(createdBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.and, "<>", ColumnCreatedBy, createdBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotEquals(createdBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.or, "<>", ColumnCreatedBy, createdBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "<>", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "<>", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "<> ANY", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "<> ANY", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "<> ALL", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "<> ALL", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.and, "<>", ColumnCreatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.or, "<>", ColumnCreatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByIn(createdBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.and, "IN", ColumnCreatedBy, createdBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByIn(createdBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.or, "IN", ColumnCreatedBy, createdBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.and, "IN", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.or, "IN", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotIn(createdBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.and, "NOT IN", ColumnCreatedBy, createdBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotIn(createdBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.or, "NOT IN", ColumnCreatedBy, createdBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedByNotInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.and, "NOT IN", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedByNotInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.or, "NOT IN", ColumnCreatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByEquals(updatedBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.and, "=", ColumnUpdatedBy, updatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByEquals(updatedBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.or, "=", ColumnUpdatedBy, updatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "=", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "=", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "= ANY", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "= ANY", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "= ALL", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "= ALL", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.and, "=", ColumnUpdatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.or, "=", ColumnUpdatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotEquals(updatedBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.and, "<>", ColumnUpdatedBy, updatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotEquals(updatedBy string, alias ...string) ConditionBuilder {
	cb.auditUserCompare(cb.or, "<>", ColumnUpdatedBy, updatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "<>", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotEqualsSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "<>", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "<> ANY", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotEqualsAny(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "<> ANY", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.and, "<> ALL", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotEqualsAll(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserCompareSubQuery(cb.or, "<> ALL", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.and, "<>", ColumnUpdatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotEqualsCurrent(alias ...string) ConditionBuilder {
	cb.auditUserCompareCurrent(cb.or, "<>", ColumnUpdatedBy, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByIn(updatedBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.and, "IN", ColumnUpdatedBy, updatedBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByIn(updatedBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.or, "IN", ColumnUpdatedBy, updatedBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.and, "IN", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.or, "IN", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotIn(updatedBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.and, "NOT IN", ColumnUpdatedBy, updatedBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotIn(updatedBys []string, alias ...string) ConditionBuilder {
	cb.auditUserIn(cb.or, "NOT IN", ColumnUpdatedBy, updatedBys, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedByNotInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.and, "NOT IN", ColumnUpdatedBy, builder, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedByNotInSubQuery(builder func(SelectQuery), alias ...string) ConditionBuilder {
	cb.auditUserInSubQuery(cb.or, "NOT IN", ColumnUpdatedBy, builder, alias...)

	return cb
}

// auditTimestampCompare adds a comparison condition for an audit timestamp column.
func (*CriteriaBuilder) auditTimestampCompare(addFn func(string, ...any), op, col string, t time.Time, alias ...string) {
	addFn("? "+op+" ?", buildColumnExpr(col, alias...), t)
}

// auditTimestampBetween adds a BETWEEN/NOT BETWEEN condition for an audit timestamp column.
func (*CriteriaBuilder) auditTimestampBetween(addFn func(string, ...any), negate bool, col string, start, end time.Time, alias ...string) {
	op := "BETWEEN"
	if negate {
		op = "NOT BETWEEN"
	}

	addFn("? "+op+" ? AND ?", buildColumnExpr(col, alias...), start, end)
}

func (cb *CriteriaBuilder) CreatedAtGreaterThan(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, ">", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedAtGreaterThan(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, ">", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedAtGreaterThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, ">=", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedAtGreaterThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, ">=", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedAtLessThan(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, "<", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedAtLessThan(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, "<", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedAtLessThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, "<=", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedAtLessThanOrEqual(createdAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, "<=", ColumnCreatedAt, createdAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.and, false, ColumnCreatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.or, false, ColumnCreatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) CreatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.and, true, ColumnCreatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrCreatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.or, true, ColumnCreatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedAtGreaterThan(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, ">", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedAtGreaterThan(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, ">", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedAtGreaterThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, ">=", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedAtGreaterThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, ">=", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedAtLessThan(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, "<", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedAtLessThan(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, "<", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedAtLessThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.and, "<=", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedAtLessThanOrEqual(updatedAt time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampCompare(cb.or, "<=", ColumnUpdatedAt, updatedAt, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.and, false, ColumnUpdatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedAtBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.or, false, ColumnUpdatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) UpdatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.and, true, ColumnUpdatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) OrUpdatedAtNotBetween(start, end time.Time, alias ...string) ConditionBuilder {
	cb.auditTimestampBetween(cb.or, true, ColumnUpdatedAt, start, end, alias...)

	return cb
}

func (cb *CriteriaBuilder) PKEquals(pk any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("PKEquals", cb.qb.GetTable(), pk, alias...)
	cb.and("? = ?", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) OrPKEquals(pk any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("OrPKEquals", cb.qb.GetTable(), pk, alias...)
	cb.or("? = ?", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) PKNotEquals(pk any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("PKNotEquals", cb.qb.GetTable(), pk, alias...)
	cb.and("? <> ?", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) OrPKNotEquals(pk any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("OrPKNotEquals", cb.qb.GetTable(), pk, alias...)
	cb.or("? <> ?", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) PKIn(pks any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("PKIn", cb.qb.GetTable(), pks, alias...)
	cb.and("? IN (?)", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) OrPKIn(pks any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("OrPKIn", cb.qb.GetTable(), pks, alias...)
	cb.or("? IN (?)", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) PKNotIn(pks any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("PKNotIn", cb.qb.GetTable(), pks, alias...)
	cb.and("? NOT IN (?)", pc, pv)

	return cb
}

func (cb *CriteriaBuilder) OrPKNotIn(pks any, alias ...string) ConditionBuilder {
	pc, pv := parsePKColumnsAndValues("OrPKNotIn", cb.qb.GetTable(), pks, alias...)
	cb.or("? NOT IN (?)", pc, pv)

	return cb
}
