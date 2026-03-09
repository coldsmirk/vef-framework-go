package search

import (
	"reflect"
	"strings"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/strx"
)

var apiInType = reflect.TypeFor[api.P]()

func New(typ reflect.Type) Search {
	typ = reflectx.Indirect(typ)
	if typ.Kind() != reflect.Struct {
		logger.Warnf("Invalid value type, expected struct, got %s", typ.Name())

		return Search{}
	}

	return Search{conditions: parseStruct(typ)}
}

func NewFor[T any]() Search {
	return New(reflect.TypeFor[T]())
}

func parseStruct(t reflect.Type) []Condition {
	conditions := make([]Condition, 0)

	visitor := reflectx.TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) reflectx.VisitAction {
			if field.Anonymous && field.Type == apiInType {
				return reflectx.SkipChildren
			}

			tag, hasTag := field.Tag.Lookup(TagSearch)

			if hasTag {
				if tag == IgnoreField {
					return reflectx.SkipChildren
				}

				if tag == AttrDive {
					return reflectx.Continue
				}

				attrs := strx.ParseTag(tag)
				conditions = append(conditions, buildCondition(field, attrs))
			} else {
				if field.Anonymous {
					return reflectx.SkipChildren
				}

				// Default to eq operator with snake_case column when no tag specified
				conditions = append(conditions, buildCondition(field, make(map[string]string)))
			}

			return reflectx.SkipChildren
		},
	}

	reflectx.VisitType(
		t, visitor,
		reflectx.WithDiveTag(TagSearch, AttrDive),
		reflectx.WithTraversalMode(reflectx.DepthFirst),
	)

	return conditions
}

func buildCondition(field reflect.StructField, attrs map[string]string) Condition {
	column := attrs[AttrColumn]
	columns := lo.Ternary(
		column == "",
		[]string{lo.SnakeCase(field.Name)},
		strings.Split(column, "|"),
	)

	operator := lo.CoalesceOrEmpty(attrs[AttrOperator], attrs[strx.DefaultKey], string(Equals))

	params := make(map[string]string)
	if attrs[AttrParams] != "" {
		params = strx.ParseTag(attrs[AttrParams],
			strx.WithSpacePairDelimiter(),
			strx.WithValueDelimiter(':'),
		)
	}

	return Condition{
		Index:    field.Index,
		Alias:    attrs[AttrAlias],
		Columns:  columns,
		Operator: Operator(operator),
		Params:   params,
	}
}
