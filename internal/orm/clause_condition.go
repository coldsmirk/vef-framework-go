package orm

import (
	"github.com/uptrace/bun/schema"
)

// ClauseConditionBuilder is responsible for collecting and grouping condition clauses, and rendering them.
type ClauseConditionBuilder struct {
	*CriteriaBuilder

	conditions []schema.QueryWithSep
}

func newConditionBuilder(qb QueryBuilder) *ClauseConditionBuilder {
	builder := &ClauseConditionBuilder{
		CriteriaBuilder: &CriteriaBuilder{
			qb: qb,
			eb: qb.ExprBuilder(),
		},
	}

	builder.and = builder.And
	builder.or = builder.Or
	builder.group = builder.BuildGroup

	return builder
}

func (cb *ClauseConditionBuilder) AppendConditions(conditions ...schema.QueryWithSep) {
	cb.conditions = append(cb.conditions, conditions...)
}

func (cb *ClauseConditionBuilder) And(query string, args ...any) {
	cb.AppendConditions(schema.SafeQueryWithSep(query, args, separatorAnd))
}

func (cb *ClauseConditionBuilder) Or(query string, args ...any) {
	cb.AppendConditions(schema.SafeQueryWithSep(query, args, separatorOr))
}

func (cb *ClauseConditionBuilder) BuildGroup(sep string, builder func(ConditionBuilder)) {
	saved := cb.conditions
	cb.conditions = nil

	builder(cb)

	on := cb.conditions
	cb.conditions = saved
	cb.AppendGroup(sep, on)
}

func (cb *ClauseConditionBuilder) AppendGroup(sep string, conditions []schema.QueryWithSep) {
	if len(conditions) == 0 {
		return
	}

	cb.AppendConditions(schema.SafeQueryWithSep("", nil, sep))
	cb.AppendConditions(schema.SafeQueryWithSep("", nil, "("))

	conditions[0].Sep = ""
	cb.AppendConditions(conditions...)
	cb.AppendConditions(schema.SafeQueryWithSep("", nil, ")"))
}

func (cb *ClauseConditionBuilder) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	if len(cb.conditions) == 0 {
		return b, nil
	}

	for i, condition := range cb.conditions {
		if i > 0 {
			b = append(b, condition.Sep...)
		}

		b, err = condition.AppendQuery(gen, b)
		if err != nil {
			return nil, err
		}
	}

	return b, nil
}
