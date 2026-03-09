package search

import (
	"reflect"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/monad"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

var (
	rangeStartFieldIndex []int
	rangeEndFieldIndex   []int
)

func init() {
	field, ok := reflect.TypeFor[monad.Range[int]]().FieldByName("Start")
	if !ok {
		panic("mo.Range[int] struct must have a 'Start' field for range operations to work properly")
	}

	rangeStartFieldIndex = field.Index

	field, ok = reflect.TypeFor[monad.Range[int]]().FieldByName("End")
	if !ok {
		panic("mo.Range[int] struct must have an 'End' field for range operations to work properly")
	}

	rangeEndFieldIndex = field.Index
}

func getRangeValue(fieldValue any, conditionParams map[string]string) (start, end any, _ bool) {
	value := reflect.Indirect(reflect.ValueOf(fieldValue))
	valueType := value.Type()
	kind := valueType.Kind()

	if kind == reflect.Struct && reflectx.IsSimilarType(valueType, rangeType) {
		return value.FieldByIndex(rangeStartFieldIndex).Interface(), value.FieldByIndex(rangeEndFieldIndex).Interface(), true
	} else if kind == reflect.String {
		return parseStringRange(value.String(), conditionParams)
	} else if kind == reflect.Slice {
		return parseSliceRange(value)
	}

	return nil, nil, false
}

func parseStringRange(value string, conditionParams map[string]string) (start, end any, _ bool) {
	if value == "" {
		return nil, nil, false
	}

	delimiter := lo.CoalesceOrEmpty(conditionParams[ParamDelimiter], ",")

	values := strings.SplitN(value, delimiter, 2)
	if len(values) != 2 {
		logger.Warnf("Invalid range value, expected value delimited by %q, got %q", delimiter, value)

		return nil, nil, false
	}

	parserMap := map[string]func([]string) (any, any, bool){
		TypeInt:      parseIntRange,
		TypeDecimal:  parseDecimalRange,
		TypeDate:     parseDateRange,
		TypeTime:     parseTimeRange,
		TypeDateTime: parseDateTimeRange,
	}

	if parser, exists := parserMap[conditionParams[ParamType]]; exists {
		return parser(values)
	}

	return nil, nil, false
}

func parseSliceRange(value reflect.Value) (start, end any, _ bool) {
	if value.Len() == 0 {
		return nil, nil, false
	}

	if value.Len() != 2 {
		logger.Warnf("Invalid range value, expected slice of length 2, got %v", value.Interface())

		return nil, nil, false
	}

	return value.Index(0).Interface(), value.Index(1).Interface(), true
}

func parseIntRange(values []string) (start, end any, _ bool) {
	var err error
	if start, err = cast.ToIntE(values[0]); err != nil {
		logger.Warnf("Invalid range value, expected int, got %v", values[0])

		return nil, nil, false
	}

	if end, err = cast.ToIntE(values[1]); err != nil {
		logger.Warnf("Invalid range value, expected int, got %v", values[1])

		return nil, nil, false
	}

	return start, end, true
}

func parseDecimalRange(values []string) (start, end any, _ bool) {
	var err error
	if start, err = decimal.NewFromString(values[0]); err != nil {
		logger.Warnf("Invalid range value, expected decimal, got %v", values[0])

		return nil, nil, false
	}

	if end, err = decimal.NewFromString(values[1]); err != nil {
		logger.Warnf("Invalid range value, expected decimal, got %v", values[1])

		return nil, nil, false
	}

	return start, end, true
}

func parseDateRange(values []string) (start, end any, _ bool) {
	return parseTimeRangeWithLayout(values, time.DateOnly, "date")
}

func parseTimeRange(values []string) (start, end any, _ bool) {
	return parseTimeRangeWithLayout(values, time.TimeOnly, "time")
}

func parseDateTimeRange(values []string) (start, end any, _ bool) {
	return parseTimeRangeWithLayout(values, time.DateTime, "datetime")
}

func parseTimeRangeWithLayout(values []string, layout, typeName string) (start, end any, _ bool) {
	var err error
	if start, err = time.ParseInLocation(layout, values[0], time.Local); err != nil {
		logger.Warnf("Invalid range value, expected %s, got %v", typeName, values[0])

		return nil, nil, false
	}

	if end, err = time.ParseInLocation(layout, values[1], time.Local); err != nil {
		logger.Warnf("Invalid range value, expected %s, got %v", typeName, values[1])

		return nil, nil, false
	}

	return start, end, true
}
