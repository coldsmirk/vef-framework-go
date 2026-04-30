package tabular

import (
	"reflect"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/reflectx"
	"github.com/coldsmirk/vef-framework-go/strx"
)

// parseStruct parses the tabular columns from a struct using visitor pattern.
func parseStruct(t reflect.Type) []*Column {
	rootType := reflectx.Indirect(t)
	if rootType.Kind() != reflect.Struct {
		logger.Warnf("Invalid value type, expected struct, got %s", rootType.Name())

		return nil
	}

	columns := make([]*Column, 0)
	columnOrder := 0

	visitor := reflectx.TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) reflectx.VisitAction {
			tag, hasTag := field.Tag.Lookup(TagTabular)
			if !hasTag {
				if field.Anonymous {
					return reflectx.SkipChildren
				}

				column := buildColumn(field, make(map[string]string), columnOrder)
				column.Key = buildFieldKey(rootType, field.Index)
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
			column.Key = buildFieldKey(rootType, field.Index)
			columns = append(columns, column)
			columnOrder++

			return reflectx.SkipChildren
		},
	}

	reflectx.VisitType(
		rootType, visitor,
		reflectx.WithDiveTag(TagTabular, AttrDive),
		reflectx.WithTraversalMode(reflectx.DepthFirst),
	)

	return columns
}

// buildColumn builds a Column from a struct field and attributes. Key is left
// empty here and is populated by parseStruct using the full field path so that
// nested dive fields produce unique keys.
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
		Name:      name,
		Type:      field.Type,
		Order:     order,
		Width:     width,
		Default:   attrs[AttrDefault],
		Format:    attrs[AttrFormat],
		Formatter: attrs[AttrFormatter],
		Parser:    attrs[AttrParser],
		Index:     field.Index,
	}
}

// buildFieldKey assembles the dotted path of struct field names for the given
// index path. Top-level fields collapse to a single segment so the common case
// matches the field name verbatim.
func buildFieldKey(rootType reflect.Type, indexPath []int) string {
	if len(indexPath) == 0 {
		return ""
	}

	parts := make([]string, len(indexPath))
	t := rootType

	for i, idx := range indexPath {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}

		f := t.Field(idx)
		parts[i] = f.Name
		t = f.Type
	}

	if len(parts) == 1 {
		return parts[0]
	}

	return strings.Join(parts, ".")
}
