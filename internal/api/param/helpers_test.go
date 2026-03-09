package param

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/page"
)

// Test structs for helper functions.

type EmbeddedParams struct {
	api.P

	Name string
}

type EmbeddedMeta struct {
	api.M

	Page int
}

type DeepEmbeddedParams struct {
	EmbeddedParams

	Extra string
}

type NoEmbed struct {
	Name string
	Age  int
}

type WithInterface struct {
	Value string
}

type TaggedDive struct {
	Inner InnerTagged `api:"dive"`
}

type InnerTagged struct {
	Name string
}

// TestEmbedsAPIParams tests embedsAPIParams detection.
func TestEmbedsAPIParams(t *testing.T) {
	t.Run("DirectEmbed", func(t *testing.T) {
		assert.True(t, embedsAPIParams(reflect.TypeFor[EmbeddedParams]()), "Should detect direct api.P embed")
	})

	t.Run("DeepEmbed", func(t *testing.T) {
		assert.True(t, embedsAPIParams(reflect.TypeFor[DeepEmbeddedParams]()), "Should detect deeply embedded api.P")
	})

	t.Run("NoEmbed", func(t *testing.T) {
		assert.False(t, embedsAPIParams(reflect.TypeFor[NoEmbed]()), "Should return false for no api.P embed")
	})

	t.Run("PointerType", func(t *testing.T) {
		assert.True(t, embedsAPIParams(reflect.TypeFor[*EmbeddedParams]()), "Should handle pointer types")
	})

	t.Run("NonStruct", func(t *testing.T) {
		assert.False(t, embedsAPIParams(reflect.TypeFor[string]()), "Should return false for non-struct")
	})
}

// TestEmbedsAPIMeta tests embedsAPIMeta detection.
func TestEmbedsAPIMeta(t *testing.T) {
	t.Run("DirectEmbed", func(t *testing.T) {
		assert.True(t, embedsAPIMeta(reflect.TypeFor[EmbeddedMeta]()), "Should detect direct api.M embed")
	})

	t.Run("NoEmbed", func(t *testing.T) {
		assert.False(t, embedsAPIMeta(reflect.TypeFor[NoEmbed]()), "Should return false for no api.M embed")
	})
}

// TestIsBuiltinParamsType tests isBuiltinParamsType.
func TestIsBuiltinParamsType(t *testing.T) {
	t.Run("NotBuiltin", func(t *testing.T) {
		assert.False(t, isBuiltinParamsType(reflect.TypeFor[string]()), "String should not be builtin params type")
	})

	t.Run("NotBuiltinStruct", func(t *testing.T) {
		assert.False(t, isBuiltinParamsType(reflect.TypeFor[NoEmbed]()), "Custom struct should not be builtin params type")
	})
}

// TestIsBuiltinMetaType tests isBuiltinMetaType.
func TestIsBuiltinMetaType(t *testing.T) {
	t.Run("PageableIsBuiltin", func(t *testing.T) {
		assert.True(t, isBuiltinMetaType(reflect.TypeFor[page.Pageable]()), "page.Pageable should be builtin meta type")
	})

	t.Run("NotBuiltin", func(t *testing.T) {
		assert.False(t, isBuiltinMetaType(reflect.TypeFor[string]()), "String should not be builtin meta type")
	})
}

// TestEmbedsSentinelType tests the embedsSentinelType BFS search.
func TestEmbedsSentinelType(t *testing.T) {
	t.Run("MatchesSelf", func(t *testing.T) {
		assert.True(t, embedsSentinelType(reflect.TypeFor[api.P](), apiParamsType), "api.P should match itself")
	})

	t.Run("NestedMatch", func(t *testing.T) {
		assert.True(t, embedsSentinelType(reflect.TypeFor[DeepEmbeddedParams](), apiParamsType), "Should find api.P in nested embed")
	})

	t.Run("NoMatch", func(t *testing.T) {
		assert.False(t, embedsSentinelType(reflect.TypeFor[NoEmbed](), apiParamsType), "Should return false when sentinel not embedded")
	})

	t.Run("NonStructType", func(t *testing.T) {
		assert.False(t, embedsSentinelType(reflect.TypeFor[int](), apiParamsType), "Should return false for non-struct type")
	})

	t.Run("PointerToStruct", func(t *testing.T) {
		assert.True(t, embedsSentinelType(reflect.TypeFor[*EmbeddedParams](), apiParamsType), "Should handle pointer indirection")
	})
}

// TestFindFieldInStruct tests the multi-pass field search strategy.
func TestFindFieldInStruct(t *testing.T) {
	type WithDirectField struct {
		DB     string
		Logger string
	}

	t.Run("DirectFieldMatch", func(t *testing.T) {
		target := reflect.ValueOf(WithDirectField{DB: "test_db"})
		found := findFieldInStruct(target, reflect.TypeFor[string]())

		assert.True(t, found.IsValid(), "Should find direct string field")
		assert.Equal(t, "test_db", found.String(), "Should return the first matching field")
	})

	t.Run("NoMatch", func(t *testing.T) {
		target := reflect.ValueOf(WithDirectField{DB: "test"})
		found := findFieldInStruct(target, reflect.TypeFor[int]())

		assert.False(t, found.IsValid(), "Should return invalid value when no match")
	})

	t.Run("TaggedFieldMatch", func(t *testing.T) {
		target := reflect.ValueOf(TaggedDive{Inner: InnerTagged{Name: "tagged"}})
		found := findFieldInStruct(target, reflect.TypeFor[string]())

		assert.True(t, found.IsValid(), "Should find tagged field via dive")
	})
}

// TestSearchDirectFields tests searchDirectFields.
func TestSearchDirectFields(t *testing.T) {
	type Target struct {
		Name  string
		Count int
	}

	t.Run("FoundByType", func(t *testing.T) {
		target := reflect.ValueOf(Target{Name: "hello", Count: 42})
		found := searchDirectFields(target, reflect.TypeFor[int]())

		assert.True(t, found.IsValid(), "Should find int field")
		assert.Equal(t, int64(42), found.Int(), "Should return correct value")
	})

	t.Run("NotFound", func(t *testing.T) {
		target := reflect.ValueOf(Target{Name: "hello"})
		found := searchDirectFields(target, reflect.TypeFor[float64]())

		assert.False(t, found.IsValid(), "Should return invalid when type not found")
	})
}

// TestSearchEmbeddedFields tests searchEmbeddedFields.
func TestSearchEmbeddedFields(t *testing.T) {
	type Base struct {
		ID string
	}

	type WithEmbed struct {
		Base

		Name string
	}

	t.Run("EmbeddedTypeMatch", func(t *testing.T) {
		target := reflect.ValueOf(WithEmbed{Base: Base{ID: "123"}, Name: "test"})
		found := searchEmbeddedFields(target, reflect.TypeFor[Base]())

		assert.True(t, found.IsValid(), "Should find embedded type")
	})

	t.Run("NoEmbeddedMatch", func(t *testing.T) {
		target := reflect.ValueOf(NoEmbed{Name: "test"})
		found := searchEmbeddedFields(target, reflect.TypeFor[WithInterface]())

		assert.False(t, found.IsValid(), "Should return invalid when no embedded match")
	})
}
