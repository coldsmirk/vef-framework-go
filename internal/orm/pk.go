package orm

import (
	"fmt"
	"reflect"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/schema"
)

// PKField describes a model's primary key field with common aliases.
// It provides helpers to get/set the PK value on a concrete model instance.
type PKField struct {
	// Field is the Go struct field name, e.g. "UserID".
	Field string
	// Column is the database column name as defined in schema, e.g. "user_id".
	Column string
	// Name is the lower camel-case alias, usually used in params or API payloads, e.g. "userID".
	Name string

	f *schema.Field
}

// Value returns the primary key value from the given model instance.
// The model must be a pointer to struct; otherwise an error is returned.
// It leverages bun's schema.Field to read the concrete field value safely.
func (p *PKField) Value(model any) (any, error) {
	value, err := p.validateModel(model)
	if err != nil {
		return nil, err
	}

	return p.f.Value(value).Interface(), nil
}

// Set writes the provided value into the model's primary key field.
// The model must be a pointer to struct. Basic kinds supported:
// - string (and *string)
// - int/int32/int64 (and their pointer forms)
// For unsupported kinds, an error is returned.
func (p *PKField) Set(model, value any) error {
	modelValue, err := p.validateModel(model)
	if err != nil {
		return err
	}

	pkValue := p.f.Value(modelValue)
	switch kind := p.f.IndirectType.Kind(); kind {
	case reflect.String:
		v, err := cast.ToStringE(value)
		if err != nil {
			return err
		}

		if p.f.IsPtr {
			pkValue.Set(reflect.ValueOf(&v))
		} else {
			pkValue.SetString(v)
		}

	case reflect.Int, reflect.Int32, reflect.Int64:
		v, err := cast.ToInt64E(value)
		if err != nil {
			return err
		}

		if p.f.IsPtr {
			pkValue.Set(reflect.ValueOf(&v))
		} else {
			pkValue.SetInt(v)
		}

	default:
		return fmt.Errorf("%w: %s", ErrPrimaryKeyUnsupportedType, kind)
	}

	return nil
}

func (*PKField) validateModel(model any) (reflect.Value, error) {
	value, ok := model.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(model)
	}

	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	} else if !value.CanAddr() {
		return reflect.Value{}, ErrModelMustBePointerToStruct
	}

	if value.Kind() != reflect.Struct {
		return reflect.Value{}, ErrModelMustBePointerToStruct
	}

	return value, nil
}

// NewPKField constructs a PKField helper from a bun schema.Field.
// Field is the Go struct field name; Column is the DB column name;
// Name is a lower-camel alias commonly used in params or API payloads.
func NewPKField(field *schema.Field) *PKField {
	return &PKField{
		Field:  field.GoName,
		Column: field.Name,
		Name:   lo.CamelCase(field.Name),
		f:      field,
	}
}

// parsePKColumnsAndValues parses the primary key columns and values from the given table and primary key value.
//
// For single-column PKs: pk is a scalar for equals, or a slice for IN operations.
// For composite PKs: pk is a slice matching the PK column count for equals (e.g. []any{"uid", "pid"}),
// or a slice of slices for IN operations (e.g. [][]any{{"uid1", "pid1"}, {"uid2", "pid2"}}).
func parsePKColumnsAndValues(method string, table *schema.Table, pk any, alias ...string) (*pkColumns, *pkValues) {
	if table == nil {
		panic(fmt.Sprintf(
			"method %q failed: table schema is nil. "+
				"This usually happens when: "+
				"1) Model() method was not called before %q, "+
				"2) Model() was called with plain nil or a value that is not a struct pointer (or slice pointer of struct), "+
				"3) Table()/TableExpr() was used without binding a model via Model(). "+
				"Please ensure you call Model() with a valid struct pointer (or slice pointer of struct) before using %q.",
			method, method, method,
		))
	}

	pks := table.PKs
	if len(pks) == 0 {
		panic(fmt.Sprintf("table %q has no primary key", table.Name))
	}

	tableAlias := table.SQLAlias
	if len(alias) > 0 {
		tableAlias = bun.Safe(alias[0])
	}

	columns := make([]bun.Safe, len(pks))
	for i, p := range pks {
		columns[i] = p.SQLName
	}

	values := parsePKValues(method, pk, len(pks))

	return &pkColumns{alias: tableAlias, columns: columns}, &pkValues{values: values}
}

// parsePKValues converts a primary key value into a normalized slice of values.
//
// For single-column PKs (pkCount == 1):
//   - Scalar → []any{scalar}
//   - Slice  → expanded element-by-element (each element is one PK value for IN/NOT IN)
//
// For composite PKs (pkCount > 1):
//   - A slice whose length matches pkCount and whose first element is NOT a slice
//     is treated as a single composite-PK tuple → []any{tuple}
//   - A slice of slices is expanded element-by-element (each element is a tuple)
func parsePKValues(method string, pk any, pkCount int) []any {
	pkv := reflect.ValueOf(pk)
	if pkv.Kind() != reflect.Slice {
		return []any{pk}
	}

	n := pkv.Len()
	if n == 0 {
		panic(fmt.Sprintf("method %q failed: primary key value is empty", method))
	}

	// For composite PKs only: a slice whose length matches the PK column count
	// and whose first element is not itself a slice represents a single tuple.
	if pkCount > 1 && n == pkCount && pkv.Index(0).Kind() != reflect.Slice {
		return []any{pk}
	}

	values := make([]any, n)
	for i := range n {
		values[i] = pkv.Index(i).Interface()
	}

	return values
}

type pkColumns struct {
	alias   bun.Safe
	columns []bun.Safe
}

func (p *pkColumns) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	cLen := len(p.columns)
	if cLen == 0 {
		return dialect.AppendNull(b), nil
	}

	if cLen > 1 {
		b = append(b, '(')
	}

	for i, column := range p.columns {
		if i > 0 {
			b = append(b, ", "...)
		}

		if b, err = p.alias.AppendQuery(gen, b); err != nil {
			return
		}

		b = append(b, '.')

		if b, err = column.AppendQuery(gen, b); err != nil {
			return
		}
	}

	if cLen > 1 {
		b = append(b, ')')
	}

	return b, nil
}

type pkValues struct {
	values []any
}

func (p *pkValues) AppendQuery(gen schema.QueryGen, b []byte) (_ []byte, err error) {
	for i, v := range p.values {
		if i > 0 {
			b = append(b, ", "...)
		}

		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice {
			// Composite PK tuple: render as (val1, val2, ...)
			if b, err = bun.Tuple(v).AppendQuery(gen, b); err != nil {
				return nil, err
			}
		} else {
			b = gen.AppendValue(b, rv)
		}
	}

	return b, nil
}
