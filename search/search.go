package search

import (
	"reflect"
	"strings"

	"github.com/coldsmirk/go-streams"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/dbx"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var logger = logx.Named("search")

// Search is a compiled set of search conditions parsed from a struct's
// `search` tags. Apply it against a populated instance of that struct to
// translate non-empty fields into ORM query conditions.
type Search struct {
	conditions []condition
}

type condition struct {
	index    []int
	alias    string
	columns  []string
	operator Operator
	params   map[string]string
}

// Apply adds a query condition for every non-empty field of target, which must
// be the (optionally pointer-wrapped) struct the Search was built from. The
// optional defaultAlias qualifies columns that do not declare their own alias.
// A non-struct target is a programming error; it is logged and Apply becomes a
// no-op, adding no conditions, so callers must not rely on it to enforce filtering.
func (f Search) Apply(cb orm.ConditionBuilder, target any, defaultAlias ...string) {
	value := reflect.Indirect(reflect.ValueOf(target))
	if value.Kind() != reflect.Struct {
		logger.Warnf("Invalid target type, expected struct, got %s", value.Type().Name())

		return
	}

	for _, c := range f.conditions {
		field := value.FieldByIndex(c.index)
		if field.Kind() == reflect.Pointer && field.IsNil() {
			continue
		}

		fieldValue := extractFieldValue(field.Interface())

		alias := getColumnAlias(c.alias, defaultAlias...)
		columns := streams.MapTo(
			streams.FromSlice(c.columns),
			func(column string) string { return dbx.ColumnWithAlias(column, alias) },
		).Collect()

		applyCondition(cb, c, columns, fieldValue)
	}
}

func extractFieldValue(fieldValue any) any {
	rv := reflect.ValueOf(fieldValue)
	if rv.Kind() == reflect.Pointer {
		return rv.Elem().Interface()
	}

	return fieldValue
}

func getColumnAlias(alias string, defaultAlias ...string) string {
	if alias != "" {
		return alias
	}

	if len(defaultAlias) > 0 {
		return defaultAlias[0]
	}

	return ""
}

func applyCondition(cb orm.ConditionBuilder, c condition, columns []string, value any) {
	switch c.operator {
	case Equals, NotEquals, GreaterThan, GreaterThanOrEqual, LessThan, LessThanOrEqual:
		applyComparisonCondition(cb, columns[0], c.operator, value)
	case Between, NotBetween:
		applyBetweenCondition(cb, columns[0], c.operator, value, c.params)
	case In, NotIn:
		applyInCondition(cb, columns[0], value, c.operator, c.params)
	case IsNull, IsNotNull:
		applyNullCondition(cb, columns[0], value, c.operator)
	case Contains, NotContains, StartsWith, NotStartsWith, EndsWith, NotEndsWith,
		ContainsIgnoreCase, NotContainsIgnoreCase, StartsWithIgnoreCase, NotStartsWithIgnoreCase,
		EndsWithIgnoreCase, NotEndsWithIgnoreCase:
		applyLikeCondition(cb, columns, value, c.operator)
	default:
		logger.Warnf("Unknown operator %q for columns %v, condition ignored", c.operator, columns)
	}
}

func applyComparisonCondition(cb orm.ConditionBuilder, column string, operator Operator, value any) {
	switch operator {
	case Equals:
		cb.Equals(column, value)
	case NotEquals:
		cb.NotEquals(column, value)
	case GreaterThan:
		cb.GreaterThan(column, value)
	case GreaterThanOrEqual:
		cb.GreaterThanOrEqual(column, value)
	case LessThan:
		cb.LessThan(column, value)
	case LessThanOrEqual:
		cb.LessThanOrEqual(column, value)
	}
}

func applyBetweenCondition(cb orm.ConditionBuilder, column string, operator Operator, value any, conditionParams map[string]string) {
	start, end, ok := getRangeValue(value, conditionParams)
	if !ok {
		return
	}

	switch operator {
	case Between:
		cb.Between(column, start, end)
	case NotBetween:
		cb.NotBetween(column, start, end)
	}
}

func applyInCondition(cb orm.ConditionBuilder, column string, fieldValue any, operator Operator, conditionParams map[string]string) {
	var values []any

	if s, ok := fieldValue.(string); ok {
		values = parseStringInCondition(s, conditionParams)
	}

	// Handle slice types
	rv := reflect.Indirect(reflect.ValueOf(fieldValue))
	if rv.Kind() == reflect.Slice {
		for i := range rv.Len() {
			values = append(values, rv.Index(i).Interface())
		}
	}

	if len(values) == 0 {
		return
	}

	switch operator {
	case In:
		cb.In(column, values)
	case NotIn:
		cb.NotIn(column, values)
	}
}

func parseStringInCondition(slice string, conditionParams map[string]string) []any {
	var values []any
	if slice == "" {
		return values
	}

	delimiter := lo.CoalesceOrEmpty(conditionParams[ParamDelimiter], ",")
	for value := range strings.SplitSeq(slice, delimiter) {
		switch conditionParams[ParamType] {
		case TypeInt:
			values = append(values, cast.ToInt(value))
		default:
			values = append(values, value)
		}
	}

	return values
}

// applyNullCondition only applies condition when value is boolean true.
func applyNullCondition(cb orm.ConditionBuilder, column string, fieldValue any, operator Operator) {
	shouldApply, _ := fieldValue.(bool)

	switch operator {
	case IsNull:
		cb.ApplyIf(shouldApply, func(cb orm.ConditionBuilder) {
			cb.IsNull(column)
		})
	case IsNotNull:
		cb.ApplyIf(shouldApply, func(cb orm.ConditionBuilder) {
			cb.IsNotNull(column)
		})
	}
}

func applyLikeCondition(cb orm.ConditionBuilder, columns []string, fieldValue any, operator Operator) {
	content, _ := fieldValue.(string)
	if content == "" {
		return
	}

	if len(columns) > 1 {
		cb.Group(func(cb orm.ConditionBuilder) {
			for _, col := range columns {
				applyLikeOperation(cb, col, content, operator, true)
			}
		})

		return
	}

	applyLikeOperation(cb, columns[0], content, operator, false)
}

func applyLikeOperation(cb orm.ConditionBuilder, column, content string, operator Operator, useOr bool) {
	switch operator {
	case Contains:
		applyLikeMethod(useOr, cb.OrContains, cb.Contains, column, content)
	case ContainsIgnoreCase:
		applyLikeMethod(useOr, cb.OrContainsIgnoreCase, cb.ContainsIgnoreCase, column, content)
	case NotContains:
		applyLikeMethod(useOr, cb.OrNotContains, cb.NotContains, column, content)
	case NotContainsIgnoreCase:
		applyLikeMethod(useOr, cb.OrNotContainsIgnoreCase, cb.NotContainsIgnoreCase, column, content)
	case StartsWith:
		applyLikeMethod(useOr, cb.OrStartsWith, cb.StartsWith, column, content)
	case StartsWithIgnoreCase:
		applyLikeMethod(useOr, cb.OrStartsWithIgnoreCase, cb.StartsWithIgnoreCase, column, content)
	case NotStartsWith:
		applyLikeMethod(useOr, cb.OrNotStartsWith, cb.NotStartsWith, column, content)
	case NotStartsWithIgnoreCase:
		applyLikeMethod(useOr, cb.OrNotStartsWithIgnoreCase, cb.NotStartsWithIgnoreCase, column, content)
	case EndsWith:
		applyLikeMethod(useOr, cb.OrEndsWith, cb.EndsWith, column, content)
	case EndsWithIgnoreCase:
		applyLikeMethod(useOr, cb.OrEndsWithIgnoreCase, cb.EndsWithIgnoreCase, column, content)
	case NotEndsWith:
		applyLikeMethod(useOr, cb.OrNotEndsWith, cb.NotEndsWith, column, content)
	case NotEndsWithIgnoreCase:
		applyLikeMethod(useOr, cb.OrNotEndsWithIgnoreCase, cb.NotEndsWithIgnoreCase, column, content)
	}
}

func applyLikeMethod(
	useOr bool,
	orMethod, andMethod func(string, string) orm.ConditionBuilder,
	column, content string,
) {
	if useOr {
		orMethod(column, content)
	} else {
		andMethod(column, content)
	}
}
