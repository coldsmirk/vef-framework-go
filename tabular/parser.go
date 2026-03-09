package tabular

import (
	"reflect"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/strx"
)

var baseModelType = reflect.TypeFor[schema.BaseModel]()

// parseStruct parses the tabular columns from a struct using visitor pattern.
func parseStruct(t reflect.Type) []*Column {
	if t = reflectx.Indirect(t); t.Kind() != reflect.Struct {
		logger.Warnf("Invalid value type, expected struct, got %s", t.Name())

		return nil
	}

	columns := make([]*Column, 0)
	columnOrder := 0

	visitor := reflectx.TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) reflectx.VisitAction {
			if field.Anonymous && field.Type == baseModelType {
				return reflectx.SkipChildren
			}

			tag, hasTag := field.Tag.Lookup(TagTabular)
			if !hasTag {
				if field.Anonymous {
					return reflectx.SkipChildren
				}

				column := buildColumn(field, make(map[string]string), columnOrder)
				columns = append(columns, column)
				columnOrder++

				return reflectx.SkipChildren
			}

			if tag == IgnoreField {
				return reflectx.SkipChildren
			}

			if tag == AttrDive {
				return reflectx.Continue
			}

			attrs := strx.ParseTag(tag)
			column := buildColumn(field, attrs, columnOrder)
			columns = append(columns, column)
			columnOrder++

			return reflectx.SkipChildren
		},
	}

	reflectx.VisitType(
		t, visitor,
		reflectx.WithDiveTag(TagTabular, AttrDive),
		reflectx.WithTraversalMode(reflectx.DepthFirst),
	)

	return columns
}

// buildColumn builds a Column from a struct field and attributes.
func buildColumn(field reflect.StructField, attrs map[string]string, autoOrder int) *Column {
	name := attrs[AttrName]
	if name == "" {
		name = lo.CoalesceOrEmpty(attrs[strx.DefaultKey], field.Name)
	}

	var width float64
	if widthStr := attrs[AttrWidth]; widthStr != "" {
		width = cast.ToFloat64(widthStr)
	}

	order := autoOrder
	if orderStr := attrs[AttrOrder]; orderStr != "" {
		order = cast.ToInt(orderStr)
	}

	return &Column{
		Index:     field.Index,
		Name:      name,
		Width:     width,
		Order:     order,
		Default:   attrs[AttrDefault],
		Format:    attrs[AttrFormat],
		Formatter: attrs[AttrFormatter],
		Parser:    attrs[AttrParser],
	}
}
