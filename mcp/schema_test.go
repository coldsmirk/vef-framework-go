package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/mcp"
)

// TestInput is a sample struct for testing schema generation.
type TestInput struct {
	Name     string   `json:"name" jsonschema:"required,description=The user name"`
	Age      int      `json:"age,omitempty" jsonschema:"minimum=0,maximum=150,description=User age"`
	Email    string   `json:"email,omitempty" jsonschema:"format=email"`
	Tags     []string `json:"tags,omitempty" jsonschema:"description=User tags"`
	Role     string   `json:"role" jsonschema:"required,enum=admin,enum=user,enum=guest"`
	IsActive bool     `json:"isActive,omitempty"`
}

// TestSchemaFor tests the SchemaFor generic function.
func TestSchemaFor(t *testing.T) {
	t.Run("BasicSchemaGeneration", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()

		require.NotNil(t, schema, "Schema should not be nil")
		assert.Equal(t, "object", schema["type"], "Top-level type should be object")

		// Verify properties exist
		props, ok := schema["properties"].(map[string]any)
		require.True(t, ok, "Properties should be a map")
		assert.Len(t, props, 6, "Should have 6 properties")

		t.Logf("Generated schema with %d properties", len(props))
	})

	t.Run("StringPropertyWithDescription", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()
		props := schema["properties"].(map[string]any)

		nameProp, ok := props["name"].(map[string]any)
		require.True(t, ok, "Name property should exist")
		assert.Equal(t, "string", nameProp["type"], "Name type should be string")
		assert.Equal(t, "The user name", nameProp["description"], "Name description should match")
	})

	t.Run("IntegerPropertyWithConstraints", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()
		props := schema["properties"].(map[string]any)

		ageProp, ok := props["age"].(map[string]any)
		require.True(t, ok, "Age property should exist")
		assert.Equal(t, "integer", ageProp["type"], "Age type should be integer")
		assert.Equal(t, float64(0), ageProp["minimum"], "Age minimum should be 0")
		assert.Equal(t, float64(150), ageProp["maximum"], "Age maximum should be 150")
	})

	t.Run("StringPropertyWithFormat", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()
		props := schema["properties"].(map[string]any)

		emailProp, ok := props["email"].(map[string]any)
		require.True(t, ok, "Email property should exist")
		assert.Equal(t, "email", emailProp["format"], "Email format should be email")
	})

	t.Run("EnumProperty", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()
		props := schema["properties"].(map[string]any)

		roleProp, ok := props["role"].(map[string]any)
		require.True(t, ok, "Role property should exist")

		enumVals, ok := roleProp["enum"].([]any)
		require.True(t, ok, "Role enum should be an array")
		assert.Len(t, enumVals, 3, "Should have 3 enum values")
		assert.Contains(t, enumVals, "admin", "Enum should contain admin")
		assert.Contains(t, enumVals, "user", "Enum should contain user")
		assert.Contains(t, enumVals, "guest", "Enum should contain guest")
	})

	t.Run("ArrayProperty", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()
		props := schema["properties"].(map[string]any)

		tagsProp, ok := props["tags"].(map[string]any)
		require.True(t, ok, "Tags property should exist")
		assert.Equal(t, "array", tagsProp["type"], "Tags type should be array")
		assert.Equal(t, "User tags", tagsProp["description"], "Tags description should match")

		items, ok := tagsProp["items"].(map[string]any)
		require.True(t, ok, "Tags items should exist")
		assert.Equal(t, "string", items["type"], "Tags items type should be string")
	})

	t.Run("RequiredFields", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()

		required, ok := schema["required"].([]any)
		require.True(t, ok, "Required should be an array")

		requiredNames := make(map[string]bool)
		for _, r := range required {
			requiredNames[r.(string)] = true
		}

		assert.True(t, requiredNames["name"], "Name should be required")
		assert.True(t, requiredNames["role"], "Role should be required")
		assert.Len(t, required, 2, "Should have 2 required fields")
	})

	t.Run("MetadataFieldsRemoved", func(t *testing.T) {
		schema := mcp.SchemaFor[TestInput]()

		// $schema is manually deleted in schemaFromType
		_, hasSchema := schema["$schema"]
		assert.False(t, hasSchema, "$schema should be removed")

		// $id is controlled by Reflector.Anonymous = true
		_, hasID := schema["$id"]
		assert.False(t, hasID, "$id should not be generated")

		// $defs is controlled by Reflector.DoNotReference = true
		_, hasDefs := schema["$defs"]
		assert.False(t, hasDefs, "$defs should not be generated")
	})
}

// TestSchemaOf tests the SchemaOf function with runtime type inference.
func TestSchemaOf(t *testing.T) {
	t.Run("PointerValue", func(t *testing.T) {
		schema := mcp.SchemaOf(&TestInput{})

		require.NotNil(t, schema, "Schema should not be nil")
		assert.Equal(t, "object", schema["type"], "Top-level type should be object")

		props, ok := schema["properties"].(map[string]any)
		require.True(t, ok, "Properties should be a map")

		_, hasName := props["name"]
		assert.True(t, hasName, "Name property should exist")
	})

	t.Run("NilValue", func(t *testing.T) {
		schema := mcp.SchemaOf(nil)
		assert.Nil(t, schema, "SchemaOf(nil) should return nil")
	})
}

// TestMustSchemaFor tests the MustSchemaFor panic behavior.
func TestMustSchemaFor(t *testing.T) {
	t.Run("ValidType", func(t *testing.T) {
		assert.NotPanics(t, func() {
			schema := mcp.MustSchemaFor[TestInput]()
			assert.NotNil(t, schema, "Schema should not be nil")
		}, "MustSchemaFor should not panic for valid type")
	})
}

// TestMustSchemaOf tests the MustSchemaOf panic behavior.
func TestMustSchemaOf(t *testing.T) {
	t.Run("ValidValue", func(t *testing.T) {
		assert.NotPanics(t, func() {
			schema := mcp.MustSchemaOf(&TestInput{})
			assert.NotNil(t, schema, "Schema should not be nil")
		}, "MustSchemaOf should not panic for valid value")
	})

	t.Run("NilValue", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = mcp.MustSchemaOf(nil)
		}, "MustSchemaOf should panic for nil value")
	})
}
