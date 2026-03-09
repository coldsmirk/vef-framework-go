package search

import (
	"reflect"
	"strings"

	"github.com/coldsmirk/go-streams"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/dbx"
	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/monad"
	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var (
	logger    = log.Named("search")
	rangeType = reflect.TypeFor[monad.Range[int]]()
)

type Search struct {
	conditions []Condition
}

type Condition struct {
	Index    []int
	Alias    string
	Columns  []string
	Operator Operator
	Params   map[string]string
}

func (f Search) Apply(cb orm.ConditionBuilder, target any, defaultAlias ...string) {
	value := reflect.Indirect(reflect.ValueOf(target))
	if value.Kind() != reflect.Struct {
		logger.Warnf("Invalid target type, expected struct, got %s", value.Type().Name())

		return
	}

	for _, c := range f.conditions {
		field := value.FieldByIndex(c.Index)
		if field.Kind() == reflect.Pointer && field.IsNil() {
			continue
		}

		fieldValue, valid := extractFieldValue(field.Interface())
		if !valid {
			continue
		}

		alias := getColumnAlias(c.Alias, defaultAlias...)
		columns := streams.MapTo(
			streams.FromSlice(c.Columns),
			func(column string) string { return dbx.ColumnWithAlias(column, alias) },
		).Collect()

		applyCondition(cb, c, columns, fieldValue)
	}
}

func extractFieldValue(fieldValue any) (any, bool) {
	switch nv := fieldValue.(type) {
	case null.String:
		return nv.ValueOrZero(), nv.Valid
	case null.Int:
		return nv.ValueOrZero(), nv.Valid
	case null.Int16:
		return nv.ValueOrZero(), nv.Valid
	case null.Int32:
		return nv.ValueOrZero(), nv.Valid
	case null.Float:
		return nv.ValueOrZero(), nv.Valid
	case null.Bool:
		return nv.ValueOrZero(), nv.Valid
	case null.Byte:
		return nv.ValueOrZero(), nv.Valid
	case null.DateTime:
		return nv.ValueOrZero(), nv.Valid
	case null.Date:
		return nv.ValueOrZero(), nv.Valid
	case null.Time:
		return nv.ValueOrZero(), nv.Valid
	case null.Decimal:
		return nv.ValueOrZero(), nv.Valid
	default:
		return fieldValue, true
	}
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

func applyCondition(cb orm.ConditionBuilder, c Condition, columns []string, value any) {
	switch c.Operator {
	case Equals, NotEquals, GreaterThan, GreaterThanOrEqual, LessThan, LessThanOrEqual:
		applyComparisonCondition(cb, columns[0], c.Operator, value)
	case Between, NotBetween:
		applyBetweenCondition(cb, columns[0], c.Operator, value, c.Params)
	case In, NotIn:
		applyInCondition(cb, columns[0], value, c.Operator, c.Params)
	case IsNull, IsNotNull:
		applyNullCondition(cb, columns[0], value, c.Operator)
	case Contains, NotContains, StartsWith, NotStartsWith, EndsWith, NotEndsWith,
		ContainsIgnoreCase, NotContainsIgnoreCase, StartsWithIgnoreCase, NotStartsWithIgnoreCase,
		EndsWithIgnoreCase, NotEndsWithIgnoreCase:
		applyLikeCondition(cb, columns, value, c.Operator)
	default:
		logger.Warnf("Unknown operator %q for columns %v, condition ignored", c.Operator, columns)
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

	switch v := fieldValue.(type) {
	case string:
		values = parseStringInCondition(v, conditionParams)
	case *string:
		values = parseStringInCondition(*v, conditionParams)
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
	var shouldApply bool
	switch value := fieldValue.(type) {
	case bool:
		shouldApply = value
	case *bool:
		shouldApply = *value
	}

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
	var content string
	switch value := fieldValue.(type) {
	case string:
		content = value
	case *string:
		content = *value
	}

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
