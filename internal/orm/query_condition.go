package orm

import (
	"github.com/uptrace/bun"
)

// QueryConditionBuilder is a builder for building query conditions.
type QueryConditionBuilder struct {
	*CriteriaBuilder
}

// newQueryConditionBuilder creates a new query builder.
func newQueryConditionBuilder(builder bun.QueryBuilder, qb QueryBuilder) *QueryConditionBuilder {
	return &QueryConditionBuilder{
		CriteriaBuilder: &CriteriaBuilder{
			qb: qb,
			eb: qb.ExprBuilder(),
			and: func(query string, args ...any) {
				builder.Where(query, args...)
			},
			or: func(query string, args ...any) {
				builder.WhereOr(query, args...)
			},
			group: func(sep string, cb func(ConditionBuilder)) {
				builder.WhereGroup(
					sep,
					func(builder bun.QueryBuilder) bun.QueryBuilder {
						cb(newQueryConditionBuilder(builder, qb))

						return builder
					},
				)
			},
		},
	}
}
