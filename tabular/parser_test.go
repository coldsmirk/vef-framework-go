package tabular

import (
	"reflect"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseStruct exercises parseStruct, the helper that converts a struct
// reflect.Type into the slice of *Column descriptors used by Schema.
func TestParseStruct(t *testing.T) {
	t.Run("StructShapes", func(t *testing.T) {
		tests := []struct {
			name     string
			typ      reflect.Type
			expected []*Column
		}{
			{
				name: "BasicStructWithNames",
				typ: reflect.TypeFor[struct {
					Name  string `tabular:"用户名"`
					Email string `tabular:"邮箱"`
					Age   int    `tabular:"年龄"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "用户名", Order: 0},
					{Index: []int{1}, Name: "邮箱", Order: 1},
					{Index: []int{2}, Name: "年龄", Order: 2},
				},
			},
			{
				name: "StructWithNameAttribute",
				typ: reflect.TypeFor[struct {
					UserName string `tabular:"name=用户名称"`
					UserAge  int    `tabular:"name=用户年龄"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "用户名称", Order: 0},
					{Index: []int{1}, Name: "用户年龄", Order: 1},
				},
			},
			{
				name: "StructWithWidth",
				typ: reflect.TypeFor[struct {
					Name        string `tabular:"姓名,width=20"`
					Description string `tabular:"描述,width=50.5"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "姓名", Width: 20, Order: 0},
					{Index: []int{1}, Name: "描述", Width: 50.5, Order: 1},
				},
			},
			{
				name: "StructWithOrder",
				typ: reflect.TypeFor[struct {
					Field1 string `tabular:"字段1,order=2"`
					Field2 string `tabular:"字段2,order=1"`
					Field3 string `tabular:"字段3,order=0"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "字段1", Order: 2},
					{Index: []int{1}, Name: "字段2", Order: 1},
					{Index: []int{2}, Name: "字段3", Order: 0},
				},
			},
			{
				name: "StructWithDefault",
				typ: reflect.TypeFor[struct {
					Status string `tabular:"状态,default=active"`
					Type   string `tabular:"类型,default=normal"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "状态", Default: "active", Order: 0},
					{Index: []int{1}, Name: "类型", Default: "normal", Order: 1},
				},
			},
			{
				name: "StructWithFormat",
				typ: reflect.TypeFor[struct {
					CreatedAt string `tabular:"创建时间,format=2006-01-02 15:04:05"`
					Amount    string `tabular:"金额,format=%.2f"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "创建时间", Format: "2006-01-02 15:04:05", Order: 0},
					{Index: []int{1}, Name: "金额", Format: "%.2f", Order: 1},
				},
			},
			{
				name: "StructWithFormatter",
				typ: reflect.TypeFor[struct {
					Status string `tabular:"状态,formatter=status_formatter"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "状态", Formatter: "status_formatter", Order: 0},
				},
			},
			{
				name: "StructWithParser",
				typ: reflect.TypeFor[struct {
					Data string `tabular:"数据,parser=json_parser"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "数据", Parser: "json_parser", Order: 0},
				},
			},
			{
				name: "StructWithIgnoredField",
				typ: reflect.TypeFor[struct {
					Name     string `tabular:"姓名"`
					Internal string `tabular:"-"`
					Email    string `tabular:"邮箱"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "姓名", Order: 0},
					{Index: []int{2}, Name: "邮箱", Order: 1},
				},
			},
			{
				name: "StructWithoutTags",
				typ: reflect.TypeFor[struct {
					Name  string
					Email string
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "Name", Order: 0},
					{Index: []int{1}, Name: "Email", Order: 1},
				},
			},
			{
				name: "StructWithMixedTagsAndNoTags",
				typ: reflect.TypeFor[struct {
					UserName string `tabular:"用户名"`
					Age      int
					Email    string `tabular:"邮箱"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "用户名", Order: 0},
					{Index: []int{1}, Name: "Age", Order: 1},
					{Index: []int{2}, Name: "邮箱", Order: 2},
				},
			},
			{
				name: "StructWithAllAttributes",
				typ: reflect.TypeFor[struct {
					Status string `tabular:"状态,name=用户状态,width=15,order=1,default=pending,format=%s,formatter=fmt,parser=psr"`
				}](),
				expected: []*Column{
					{
						Index:     []int{0},
						Name:      "用户状态",
						Width:     15,
						Order:     1,
						Default:   "pending",
						Format:    "%s",
						Formatter: "fmt",
						Parser:    "psr",
					},
				},
			},
			{
				name:     "EmptyStruct",
				typ:      reflect.TypeFor[struct{}](),
				expected: []*Column{},
			},
			{
				name: "StructWithUntaggedEmbedded",
				typ: reflect.TypeFor[struct {
					Name     string `tabular:"姓名"`
					Embedded struct {
						Field1 string
						Field2 string
					}
					Email string `tabular:"邮箱"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "姓名", Order: 0},
					{Index: []int{1}, Name: "Embedded", Order: 1},
					{Index: []int{2}, Name: "邮箱", Order: 2},
				},
			},
			{
				name: "StructWithEmptyTagValue",
				typ: reflect.TypeFor[struct {
					Name string `tabular:""`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "Name", Order: 0},
				},
			},
			{
				name: "StructWithUnicode",
				typ: reflect.TypeFor[struct {
					Name   string `tabular:"用户名称"`
					Status string `tabular:"状态,default=已激活"`
				}](),
				expected: []*Column{
					{Index: []int{0}, Name: "用户名称", Order: 0},
					{Index: []int{1}, Name: "状态", Default: "已激活", Order: 1},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parseStruct(tt.typ)
				require.Len(t, result, len(tt.expected), "parseStruct should return one column per visible exported field")

				for i, expected := range tt.expected {
					actual := result[i]
					assert.Equal(t, expected.Index, actual.Index, "Index should mirror the field's reflect.StructField.Index at position %d", i)
					assert.Equal(t, expected.Name, actual.Name, "Name should resolve to tag value or field name at position %d", i)
					assert.Equal(t, expected.Width, actual.Width, "Width tag attribute should be parsed at position %d", i)
					assert.Equal(t, expected.Order, actual.Order, "Order tag attribute (or sequential default) should win at position %d", i)
					assert.Equal(t, expected.Default, actual.Default, "Default tag attribute should be preserved at position %d", i)
					assert.Equal(t, expected.Format, actual.Format, "Format tag attribute should be preserved at position %d", i)
					assert.Equal(t, expected.Formatter, actual.Formatter, "Formatter tag attribute should be preserved at position %d", i)
					assert.Equal(t, expected.Parser, actual.Parser, "Parser tag attribute should be preserved at position %d", i)
				}
			})
		}
	})

	t.Run("NonStructTypes", func(t *testing.T) {
		tests := []struct {
			name string
			typ  reflect.Type
		}{
			{"IntType", reflect.TypeFor[int]()},
			{"StringType", reflect.TypeFor[string]()},
			{"SliceType", reflect.TypeFor[[]int]()},
			{"MapType", reflect.TypeFor[map[string]int]()},
			{"PointerToInt", reflect.TypeFor[*int]()},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parseStruct(tt.typ)
				assert.Nil(t, result, "parseStruct should return nil when given a non-struct type (%s)", tt.typ.Kind())
			})
		}
	})
}

// TestNewSchema covers the public NewSchema / NewSchemaFor entry points and
// the high-level ordering / lookup behavior of the resulting Schema.
func TestNewSchema(t *testing.T) {
	t.Run("RespectsOrderTag", func(t *testing.T) {
		type TestStruct struct {
			Field3 string `tabular:"字段3,order=2"`
			Field1 string `tabular:"字段1,order=0"`
			Field2 string `tabular:"字段2,order=1"`
		}

		schema := NewSchema(reflect.TypeFor[TestStruct]())
		require.Equal(t, 3, schema.ColumnCount(), "Schema should have one column per exported field")

		columns := schema.Columns()
		assert.Equal(t, "字段1", columns[0].Name, "order=0 column should sort first")
		assert.Equal(t, "字段2", columns[1].Name, "order=1 column should sort second")
		assert.Equal(t, "字段3", columns[2].Name, "order=2 column should sort third")
	})

	t.Run("NewSchemaForGenericHelper", func(t *testing.T) {
		type TestStruct struct {
			Name  string `tabular:"姓名"`
			Email string `tabular:"邮箱"`
		}

		schema := NewSchemaFor[TestStruct]()

		assert.Equal(t, 2, schema.ColumnCount(), "Generic helper should produce the same columns as NewSchema")
		assert.Equal(t, []string{"姓名", "邮箱"}, schema.ColumnNames(), "ColumnNames should reflect declaration order without explicit Order")
	})

	t.Run("ColumnNames", func(t *testing.T) {
		type TestStruct struct {
			Name  string `tabular:"用户名"`
			Email string `tabular:"电子邮箱"`
			Age   int    `tabular:"年龄"`
		}

		schema := NewSchemaFor[TestStruct]()
		assert.Equal(t, []string{"用户名", "电子邮箱", "年龄"}, schema.ColumnNames(),
			"ColumnNames should return tag-derived display names in column order")
	})

	t.Run("OrderingAcrossManyFields", func(t *testing.T) {
		type UnorderedStruct struct {
			Field5 string `tabular:"字段5,order=4"`
			Field2 string `tabular:"字段2,order=1"`
			Field4 string `tabular:"字段4,order=3"`
			Field1 string `tabular:"字段1,order=0"`
			Field3 string `tabular:"字段3,order=2"`
		}

		schema := NewSchemaFor[UnorderedStruct]()

		names := schema.ColumnNames()
		orders := lo.Map(schema.Columns(), func(col *Column, _ int) int {
			return col.Order
		})

		assert.Equal(t, []string{"字段1", "字段2", "字段3", "字段4", "字段5"}, names,
			"Columns should sort by Order regardless of source declaration order")
		assert.Equal(t, []int{0, 1, 2, 3, 4}, orders,
			"Sorted column Order values should be strictly ascending")
	})

	t.Run("EmptySchema", func(t *testing.T) {
		type EmptyStruct struct{}

		schema := NewSchemaFor[EmptyStruct]()
		assert.Equal(t, 0, schema.ColumnCount(), "Schema for empty struct should have zero columns")
		assert.Empty(t, schema.Columns(), "Columns slice for empty struct should be empty")
		assert.Empty(t, schema.ColumnNames(), "ColumnNames for empty struct should be empty")
	})
}

// TestBuildColumn covers the lower-level helper that turns a single
// (StructField, attrs, order) triple into a *Column descriptor.
func TestBuildColumn(t *testing.T) {
	tests := []struct {
		name     string
		field    reflect.StructField
		attrs    map[string]string
		order    int
		expected *Column
	}{
		{
			name: "BasicField",
			field: reflect.StructField{
				Name:  "UserName",
				Index: []int{0},
			},
			attrs: map[string]string{},
			order: 0,
			expected: &Column{
				Index: []int{0},
				Name:  "UserName",
				Order: 0,
			},
		},
		{
			name: "FieldWithDefaultValue",
			field: reflect.StructField{
				Name:  "Status",
				Index: []int{0},
			},
			attrs: map[string]string{
				"__default": "状态",
			},
			order: 5,
			expected: &Column{
				Index: []int{0},
				Name:  "状态",
				Order: 5,
			},
		},
		{
			name: "FieldWithNameAttribute",
			field: reflect.StructField{
				Name:  "Status",
				Index: []int{1},
			},
			attrs: map[string]string{
				"name": "用户状态",
			},
			order: 2,
			expected: &Column{
				Index: []int{1},
				Name:  "用户状态",
				Order: 2,
			},
		},
		{
			name: "FieldWithAllAttributes",
			field: reflect.StructField{
				Name:  "CreatedAt",
				Index: []int{2, 1},
			},
			attrs: map[string]string{
				"name":      "创建时间",
				"width":     "25.5",
				"order":     "10",
				"default":   "2024-01-01",
				"format":    "2006-01-02",
				"formatter": "date_fmt",
				"parser":    "date_psr",
			},
			order: 99,
			expected: &Column{
				Index:     []int{2, 1},
				Name:      "创建时间",
				Width:     25.5,
				Order:     10,
				Default:   "2024-01-01",
				Format:    "2006-01-02",
				Formatter: "date_fmt",
				Parser:    "date_psr",
			},
		},
		{
			name: "InvalidWidthFallsBackToZero",
			field: reflect.StructField{
				Name:  "Field1",
				Index: []int{0},
			},
			attrs: map[string]string{
				"width": "invalid",
			},
			order: 0,
			expected: &Column{
				Index: []int{0},
				Name:  "Field1",
				Width: 0,
				Order: 0,
			},
		},
		{
			name: "InvalidOrderFallsBackToZero",
			field: reflect.StructField{
				Name:  "Field2",
				Index: []int{0},
			},
			attrs: map[string]string{
				"order": "notanumber",
			},
			order: 3,
			expected: &Column{
				Index: []int{0},
				Name:  "Field2",
				Order: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildColumn(tt.field, tt.attrs, tt.order)
			assert.Equal(t, tt.expected.Index, result.Index, "buildColumn should preserve the source field's reflect index")
			assert.Equal(t, tt.expected.Name, result.Name, "buildColumn should resolve Name from name attribute, __default, or field name")
			assert.Equal(t, tt.expected.Width, result.Width, "buildColumn should parse width attribute or default to zero on parse failure")
			assert.Equal(t, tt.expected.Order, result.Order, "buildColumn should parse order attribute or fall back to argument when invalid")
			assert.Equal(t, tt.expected.Default, result.Default, "buildColumn should pass default attribute through verbatim")
			assert.Equal(t, tt.expected.Format, result.Format, "buildColumn should pass format attribute through verbatim")
			assert.Equal(t, tt.expected.Formatter, result.Formatter, "buildColumn should pass formatter attribute through verbatim")
			assert.Equal(t, tt.expected.Parser, result.Parser, "buildColumn should pass parser attribute through verbatim")
		})
	}
}
