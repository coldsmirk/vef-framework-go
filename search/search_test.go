package search

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/monad"
)

// Test structs with various field types and conditions

type SimpleSearch struct {
	Name   string `search:"column=name,operator=contains"`
	Age    int    `search:"operator=gte"`
	Active bool   `search:"column=is_active"`
	Salary float64
}

type ComplexSearch struct {
	Title       string `search:"column=title,operator=eq"`
	Description string `search:"operator=contains"`
	Content     string `search:"operator=startsWith"`
	Tags        string `search:"operator=endsWith"`
	Category    string `search:"operator=iContains"`

	MinPrice int     `search:"column=price,operator=gte"`
	MaxPrice int     `search:"column=price,operator=lte"`
	Rating   float64 `search:"operator=gt"`

	StatusList string `search:"column=status,operator=in,params=delimiter:|"`
	ExcludeIDs string `search:"column=id,operator=notIn"`

	PriceRange monad.Range[int] `search:"column=price,operator=between"`
	DateRange  string           `search:"column=created_at,operator=between,params=type:date,delimiter:-"`

	DeletedAt bool `search:"column=deleted_at,operator=isNull"`
	UpdatedAt bool `search:"column=updated_at,operator=isNotNull"`

	SearchText string `search:"column=title|description|content,operator=contains"`
}

type NestedSearch struct {
	User      UserSearch    `search:"dive"`
	Status    string        `search:"operator:eq"`
	Product   ProductSearch `search:"dive"`
	CreatedAt time.Time     `search:"column=created_at,operator=gte"`
}

type UserSearch struct {
	Name     string `search:"column=user_name,operator=contains"`
	Email    string `search:"column=user_email,operator=eq"`
	IsActive bool   `search:"column=user_active"`
}

type ProductSearch struct {
	ProductName string         `search:"column=product_name"`
	Price       float64        `search:"column=product_price,operator=gte"`
	Category    CategorySearch `search:"dive"`
}

type CategorySearch struct {
	Name string `search:"column=category_name,operator=eq"`
	Code string `search:"column=category_code"`
}

type EdgeCaseSearch struct {
	NoTagField   string `search:""`
	IgnoredField string `search:"-"`
	OnlyOperator string `search:"operator=contains"`
	CustomAlias  string `search:"alias=t1,column=name"`
	WithArgs     string `search:"operator=in,params=delimiter:;,type:int"`
	WithDefault  string `search:"startsWith"`
	InvalidDive  string `search:"dive"`
}

type ShorthandSearch struct {
	Name1 string `search:"eq"`
	Name2 string `search:"contains"`
	Name3 string `search:"startsWith"`
	Name4 string `search:"contains,column=title|description"`
	Name5 string `search:"in,column=status,params=delimiter:|"`
	Name6 string `search:"gte,column=price"`
	Name7 string `search:"operator=endsWith,column=suffix"`
}

// TestNew tests new functionality.
func TestNew(t *testing.T) {
	search := NewFor[SimpleSearch]()

	assert.NotNil(t, search.conditions, "Conditions should be initialized")
	assert.Len(t, search.conditions, 4, "Should have all fields including no-tag field")
}

// TestNewFromType tests new from type functionality.
func TestNewFromType(t *testing.T) {
	search := New(reflect.TypeFor[SimpleSearch]())

	assert.NotNil(t, search.conditions, "Conditions should be initialized")
	assert.Len(t, search.conditions, 4, "Should have all fields including no-tag field")
}

// TestSimpleSearch tests simple search functionality.
func TestSimpleSearch(t *testing.T) {
	search := NewFor[SimpleSearch]()

	tests := []struct {
		column   string
		operator Operator
		alias    string
		params   map[string]string
	}{
		{
			column:   "name",
			operator: Contains,
			alias:    "",
			params:   map[string]string{},
		},
		{
			column:   "age",
			operator: GreaterThanOrEqual,
			alias:    "",
			params:   map[string]string{},
		},
		{
			column:   "is_active",
			operator: Equals,
			alias:    "",
			params:   map[string]string{},
		},
		{
			column:   "salary",
			operator: Equals,
			alias:    "",
			params:   map[string]string{},
		},
	}

	assert.Len(t, search.conditions, len(tests), "Should have correct number of conditions")

	conditionsByColumn := make(map[string]Condition)
	for _, condition := range search.conditions {
		assert.Len(t, condition.Columns, 1, "Each condition should have exactly one column")
		conditionsByColumn[condition.Columns[0]] = condition
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			condition, exists := conditionsByColumn[tt.column]
			assert.True(t, exists, "Column should exist in conditions")
			assert.Equal(t, tt.operator, condition.Operator, "Operator should match")
			assert.Equal(t, tt.alias, condition.Alias, "Alias should match")
			assert.Equal(t, tt.params, condition.Params, "Params should match")
		})
	}
}

// TestComplexSearch tests complex search functionality.
func TestComplexSearch(t *testing.T) {
	search := NewFor[ComplexSearch]()

	assert.Greater(t, len(search.conditions), 5, "Should have multiple conditions")

	t.Run("MultiColumnCondition", func(t *testing.T) {
		foundMultiColumn := false
		for _, condition := range search.conditions {
			if len(condition.Columns) > 1 {
				foundMultiColumn = true

				assert.Equal(t, Contains, condition.Operator, "Multi-column should use contains operator")
			}
		}

		assert.True(t, foundMultiColumn, "Should have multi-column condition")
	})

	t.Run("ConditionWithParams", func(t *testing.T) {
		foundWithParams := false
		for _, condition := range search.conditions {
			if len(condition.Params) > 0 {
				foundWithParams = true

				break
			}
		}

		assert.True(t, foundWithParams, "Should have condition with params")
	})

	t.Run("RangeOperators", func(t *testing.T) {
		foundRangeOp := false
		for _, condition := range search.conditions {
			if condition.Operator == Between || condition.Operator == NotBetween {
				foundRangeOp = true

				break
			}
		}

		assert.True(t, foundRangeOp, "Should have range operator")
	})
}

// TestNestedSearch tests nested search functionality.
func TestNestedSearch(t *testing.T) {
	search := NewFor[NestedSearch]()

	expectedColumns := []string{
		"user_name", "user_email", "user_active",
		"status", "created_at",
		"product_name", "product_price",
		"category_name", "category_code",
	}

	assert.Len(t, search.conditions, len(expectedColumns), "Should have all nested fields")

	for _, expectedCol := range expectedColumns {
		t.Run(expectedCol, func(t *testing.T) {
			found := false
			for _, condition := range search.conditions {
				if len(condition.Columns) == 1 && condition.Columns[0] == expectedCol {
					found = true

					break
				}
			}

			assert.True(t, found, "Expected column should be found")
		})
	}
}

// TestEdgeCases tests edge cases functionality.
func TestEdgeCases(t *testing.T) {
	search := NewFor[EdgeCaseSearch]()

	assert.Len(t, search.conditions, 5, "Should have exactly 5 conditions")

	t.Run("CustomAlias", func(t *testing.T) {
		foundWithAlias := false
		for _, condition := range search.conditions {
			if condition.Alias == "t1" {
				foundWithAlias = true

				assert.Equal(t, []string{"name"}, condition.Columns, "Should have correct column")
			}
		}

		assert.True(t, foundWithAlias, "Should have condition with custom alias")
	})

	t.Run("WithParams", func(t *testing.T) {
		foundWithParams := false
		for _, condition := range search.conditions {
			if len(condition.Params) > 0 {
				foundWithParams = true

				break
			}
		}

		assert.True(t, foundWithParams, "Should have condition with params")
	})

	t.Run("DefaultOperator", func(t *testing.T) {
		foundDefault := false
		for _, condition := range search.conditions {
			if condition.Operator == "startsWith" {
				foundDefault = true

				break
			}
		}

		assert.True(t, foundDefault, "Should have condition with default operator")
	})
}

// TestOperatorShorthand tests operator shorthand functionality.
func TestOperatorShorthand(t *testing.T) {
	search := NewFor[ShorthandSearch]()

	tests := []struct {
		key      string
		operator Operator
		columns  []string
		params   map[string]string
	}{
		{
			key:      "name_1",
			operator: "eq",
			columns:  []string{"name_1"},
			params:   map[string]string{},
		},
		{
			key:      "name_2",
			operator: "contains",
			columns:  []string{"name_2"},
			params:   map[string]string{},
		},
		{
			key:      "name_3",
			operator: "startsWith",
			columns:  []string{"name_3"},
			params:   map[string]string{},
		},
		{
			key:      "title",
			operator: "contains",
			columns:  []string{"title", "description"},
			params:   map[string]string{},
		},
		{
			key:      "status",
			operator: "in",
			columns:  []string{"status"},
			params:   map[string]string{"delimiter": "|"},
		},
		{
			key:      "price",
			operator: "gte",
			columns:  []string{"price"},
			params:   map[string]string{},
		},
		{
			key:      "suffix",
			operator: "endsWith",
			columns:  []string{"suffix"},
			params:   map[string]string{},
		},
	}

	assert.Len(t, search.conditions, len(tests), "Should have correct number of conditions")

	conditionsByFirstColumn := make(map[string]Condition)
	for _, condition := range search.conditions {
		assert.Greater(t, len(condition.Columns), 0, "Should have at least one column")
		conditionsByFirstColumn[condition.Columns[0]] = condition
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			condition, exists := conditionsByFirstColumn[tt.key]
			assert.True(t, exists, "Column should exist in conditions")
			assert.Equal(t, tt.operator, condition.Operator, "Operator should match")
			assert.Equal(t, tt.columns, condition.Columns, "Columns should match")
			assert.Equal(t, tt.params, condition.Params, "Params should match")
		})
	}
}

// TestNewFromTypeWithNonStruct tests new from type with non struct functionality.
func TestNewFromTypeWithNonStruct(t *testing.T) {
	tests := []struct {
		name      string
		inputType reflect.Type
	}{
		{"String", reflect.TypeFor[string]()},
		{"Int", reflect.TypeFor[int]()},
		{"Slice", reflect.TypeFor[[]string]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			search := New(tt.inputType)
			assert.Empty(t, search.conditions, "Non-struct types should return empty conditions")
		})
	}
}

// TestEmptyStruct tests empty struct functionality.
func TestEmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	search := NewFor[EmptyStruct]()
	assert.Empty(t, search.conditions, "Empty struct should have no conditions")
}

// TestStructWithoutSearchTags tests struct without search tags functionality.
func TestStructWithoutSearchTags(t *testing.T) {
	type NoSearchTags struct {
		Name   string
		Age    int
		Active bool
	}

	search := NewFor[NoSearchTags]()

	assert.Len(t, search.conditions, 3, "Should have conditions for all fields")

	for _, condition := range search.conditions {
		assert.Equal(t, Equals, condition.Operator, "Should use default operator")
	}
}

// TestDeepNestedStruct tests deep nested struct functionality.
func TestDeepNestedStruct(t *testing.T) {
	type Level3 struct {
		Value string `search:"column=level3_value"`
	}

	type Level2 struct {
		Name   string `search:"column=level2_name"`
		Level3 Level3 `search:"dive"`
	}

	type Level1 struct {
		Title  string `search:"column=level1_title"`
		Level2 Level2 `search:"dive"`
	}

	search := NewFor[Level1]()

	expectedColumns := []string{"level1_title", "level2_name", "level3_value"}

	assert.Len(t, search.conditions, 3, "Should have all deeply nested fields")

	for _, expectedCol := range expectedColumns {
		t.Run(expectedCol, func(t *testing.T) {
			found := false
			for _, condition := range search.conditions {
				if len(condition.Columns) == 1 && condition.Columns[0] == expectedCol {
					found = true

					break
				}
			}

			assert.True(t, found, "Expected column should be found")
		})
	}
}

// TestNoTagStruct tests no tag struct functionality.
func TestNoTagStruct(t *testing.T) {
	type TestNoTagStruct struct {
		Name   string
		Age    int
		Email  string
		Status int `search:"-"`
	}

	search := NewFor[TestNoTagStruct]()

	tests := []struct {
		column   string
		operator Operator
	}{
		{"name", Equals},
		{"age", Equals},
		{"email", Equals},
	}

	assert.Len(t, search.conditions, len(tests), "Should have conditions excluding ignored fields")

	conditionsByColumn := make(map[string]Condition)
	for _, condition := range search.conditions {
		conditionsByColumn[condition.Columns[0]] = condition
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			condition, exists := conditionsByColumn[tt.column]
			assert.True(t, exists, "Column should exist in conditions")
			assert.Equal(t, tt.operator, condition.Operator, "Should use default operator")
		})
	}
}
