package mapx

import (
	"mime/multipart"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/null"
	"github.com/coldsmirk/vef-framework-go/sortx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type TestStruct struct {
	Name       string        `json:"name"`
	Age        int           `json:"age"`
	Email      string        `json:"email,omitempty"`
	Active     bool          `json:"active"`
	Score      float64       `json:"score"`
	Created    time.Time     `json:"created"`
	Duration   time.Duration `json:"duration"`
	Website    *url.URL      `json:"website"`
	IP         net.IP        `json:"ip"`
	Unexported string
}

type EmbeddedStruct struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

type StructWithEmbedding struct {
	Name     string         `json:"name"`
	Embedded EmbeddedStruct `json:"embedded,inline"`
}

// TestNewDecoder tests decoder creation with various options.
func TestNewDecoder(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		var result TestStruct

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")
		assert.NotNil(t, decoder, "Decoder should not be nil")
	})

	t.Run("CustomOptions", func(t *testing.T) {
		var result TestStruct

		decoder, err := NewDecoder(&result, WithTagName("custom"), WithErrorUnused())
		require.NoError(t, err, "Decoder creation with custom options should succeed")
		assert.NotNil(t, decoder, "Decoder should not be nil")
	})
}

// TestToMap tests struct to map conversion.
func TestToMap(t *testing.T) {
	t.Run("ValidStruct", func(t *testing.T) {
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		testURL, _ := url.Parse("https://example.com")
		input := TestStruct{
			Name:     "John Doe",
			Age:      30,
			Email:    "john@example.com",
			Active:   true,
			Score:    95.5,
			Created:  testTime,
			Duration: time.Hour,
			Website:  testURL,
			IP:       net.ParseIP("192.168.1.1"),
		}

		result, err := ToMap(input)
		require.NoError(t, err, "Struct to map conversion should succeed")

		assert.Equal(t, "John Doe", result["name"], "Name should match")
		assert.Equal(t, 30, result["age"], "Age should match")
		assert.Equal(t, "john@example.com", result["email"], "Email should match")
		assert.Equal(t, true, result["active"], "Active should match")
		assert.Equal(t, 95.5, result["score"], "Score should match")
		assert.Contains(t, result, "created", "Created field should exist")
		assert.Contains(t, result, "duration", "Duration field should exist")
		assert.Contains(t, result, "website", "Website field should exist")
		assert.Contains(t, result, "ip", "IP field should exist")
	})

	t.Run("PointerToStruct", func(t *testing.T) {
		input := &TestStruct{
			Name: "Jane Doe",
			Age:  25,
		}

		result, err := ToMap(input)
		require.NoError(t, err, "Pointer to struct conversion should succeed")

		assert.Equal(t, "Jane Doe", result["name"], "Name should match")
		assert.Equal(t, 25, result["age"], "Age should match")
	})

	t.Run("StructWithEmbedding", func(t *testing.T) {
		input := StructWithEmbedding{
			Name: "Test",
			Embedded: EmbeddedStruct{
				ID:   123,
				Type: "example",
			},
		}

		result, err := ToMap(input)
		require.NoError(t, err, "Struct with embedding conversion should succeed")

		assert.Equal(t, "Test", result["name"], "Name should match")
		assert.Equal(t, 123, result["id"], "Embedded id should be inlined")
		assert.Equal(t, "example", result["type"], "Embedded type should be inlined")
	})

	t.Run("NonStructValue", func(t *testing.T) {
		input := "not a struct"

		result, err := ToMap(input)
		assert.Error(t, err, "Non-struct value should error")
		assert.Nil(t, result, "Result should be nil for error case")
		assert.Contains(t, err.Error(), "must be a struct", "Error should mention struct requirement")
	})

	t.Run("SliceInput", func(t *testing.T) {
		input := []int{1, 2, 3}

		result, err := ToMap(input)
		assert.Error(t, err, "Slice input should error")
		assert.Nil(t, result, "Result should be nil for error case")
		assert.Contains(t, err.Error(), "must be a struct", "Error should mention struct requirement")
	})

	t.Run("CustomTagName", func(t *testing.T) {
		type CustomTagStruct struct {
			Name string `custom:"full_name"`
			Age  int    `custom:"years"`
		}

		input := CustomTagStruct{Name: "John", Age: 30}
		result, err := ToMap(input, WithTagName("custom"))
		require.NoError(t, err, "Conversion with custom tag should succeed")

		assert.Equal(t, "John", result["full_name"], "Custom tag name should be used")
		assert.Equal(t, 30, result["years"], "Custom tag name should be used")
	})
}

// TestFromMap tests map to struct conversion.
func TestFromMap(t *testing.T) {
	t.Run("ValidMapToStruct", func(t *testing.T) {
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		input := map[string]any{
			"name":     "John Doe",
			"age":      30,
			"email":    "john@example.com",
			"active":   true,
			"score":    95.5,
			"created":  testTime,
			"duration": "1h",
			"website":  "https://example.com",
			"ip":       "192.168.1.1",
		}

		result, err := FromMap[TestStruct](input)
		require.NoError(t, err, "Map to struct conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "John Doe", result.Name, "Name should match")
		assert.Equal(t, 30, result.Age, "Age should match")
		assert.Equal(t, "john@example.com", result.Email, "Email should match")
		assert.Equal(t, true, result.Active, "Active should match")
		assert.Equal(t, 95.5, result.Score, "Score should match")
		assert.Equal(t, testTime, result.Created, "Created time should match")
		assert.Equal(t, time.Hour, result.Duration, "Duration should match")
		assert.Equal(t, "https://example.com", result.Website.String(), "Website URL should match")
		assert.Equal(t, "192.168.1.1", result.IP.String(), "IP address should match")
	})

	t.Run("PartialMap", func(t *testing.T) {
		input := map[string]any{
			"name": "Jane Doe",
			"age":  25,
		}

		result, err := FromMap[TestStruct](input)
		require.NoError(t, err, "Partial map conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "Jane Doe", result.Name, "Name should match")
		assert.Equal(t, 25, result.Age, "Age should match")
		assert.Equal(t, "", result.Email, "Email should be empty")
		assert.Equal(t, false, result.Active, "Active should be false (zero value)")
	})

	t.Run("EmptyMap", func(t *testing.T) {
		input := map[string]any{}

		result, err := FromMap[TestStruct](input)
		require.NoError(t, err, "Empty map conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "", result.Name, "Name should be empty (zero value)")
		assert.Equal(t, 0, result.Age, "Age should be 0 (zero value)")
	})

	t.Run("MapWithEmbedding", func(t *testing.T) {
		input := map[string]any{
			"name": "Test",
			"id":   123,
			"type": "example",
		}

		result, err := FromMap[StructWithEmbedding](input)
		require.NoError(t, err, "Map with embedded fields conversion should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "Test", result.Name, "Name should match")
		assert.Equal(t, 123, result.Embedded.ID, "Embedded id should match")
		assert.Equal(t, "example", result.Embedded.Type, "Embedded type should match")
	})

	t.Run("NonStructTypeParameter", func(t *testing.T) {
		input := map[string]any{"value": "test"}

		result, err := FromMap[string](input)
		assert.Error(t, err, "Non-struct type parameter should error")
		assert.Nil(t, result, "Result should be nil for error case")
		assert.Contains(t, err.Error(), "must be a struct", "Error should mention struct requirement")
	})

	t.Run("CustomTagName", func(t *testing.T) {
		type CustomTagStruct struct {
			Name string `custom:"full_name"`
			Age  int    `custom:"years"`
		}

		input := map[string]any{
			"full_name": "John",
			"years":     30,
		}

		result, err := FromMap[CustomTagStruct](input, WithTagName("custom"))
		require.NoError(t, err, "Conversion with custom tag should succeed")
		require.NotNil(t, result, "Result should not be nil")

		assert.Equal(t, "John", result.Name, "Name should match")
		assert.Equal(t, 30, result.Age, "Age should match")
	})
}

// TestDecoderOptions tests various decoder configuration options.
func TestDecoderOptions(t *testing.T) {
	t.Run("WithTagName", func(t *testing.T) {
		type TestStruct struct {
			Name string `yaml:"fullName"`
		}

		input := map[string]any{"fullName": "John"}
		result, err := FromMap[TestStruct](input, WithTagName("yaml"))
		require.NoError(t, err, "Decoding with custom tag name should succeed")
		assert.Equal(t, "John", result.Name, "Name should match")
	})

	t.Run("WithIgnoreUntaggedFields", func(t *testing.T) {
		type TestStruct struct {
			Name          string `json:"name"`
			UntaggedField string
		}

		input := map[string]any{
			"name":          "John",
			"UntaggedField": "should be ignored",
		}

		result, err := FromMap[TestStruct](input, WithIgnoreUntaggedFields(true))
		require.NoError(t, err, "Decoding with ignored untagged fields should succeed")
		assert.Equal(t, "John", result.Name, "Name should match")
		assert.Equal(t, "", result.UntaggedField, "Untagged field should be empty")
	})

	t.Run("WithWeaklyTypedInput", func(t *testing.T) {
		type TestStruct struct {
			Age int `json:"age"`
		}

		input := map[string]any{"age": "30"}
		result, err := FromMap[TestStruct](input, WithWeaklyTypedInput())
		require.NoError(t, err, "Weakly typed input conversion should succeed")
		assert.Equal(t, 30, result.Age, "Age should be converted from string to int")
	})

	t.Run("WithZeroFields", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		input := map[string]any{"name": "New Name"}
		result, err := FromMap[TestStruct](input, WithZeroFields())
		require.NoError(t, err, "Decoding with zero fields should succeed")

		assert.Equal(t, "New Name", result.Name, "Name should match")
		assert.Equal(t, 0, result.Age, "Age should be zero (default)")
	})

	t.Run("WithMetadata", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var metadata Metadata

		input := map[string]any{
			"name":  "John",
			"age":   30,
			"extra": "unused field",
		}

		result, err := FromMap[TestStruct](input, WithMetadata(&metadata))
		require.NoError(t, err, "Decoding with metadata should succeed")
		assert.Equal(t, "John", result.Name, "Name should match")
		assert.Equal(t, 30, result.Age, "Age should match")
		assert.Contains(t, metadata.Unused, "extra", "Metadata should contain unused fields")
	})
}

// TestComplexTypeConversions tests conversion of complex types.
func TestComplexTypeConversions(t *testing.T) {
	t.Run("TimeConversion", func(t *testing.T) {
		type TimeStruct struct {
			Created time.Time `json:"created"`
		}

		input := map[string]any{
			"created": "2023-01-01T12:00:00Z",
		}

		result, err := FromMap[TimeStruct](input)
		require.NoError(t, err, "Time conversion should succeed")

		expectedTime, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
		assert.Equal(t, expectedTime, result.Created, "Time should match")
	})

	t.Run("DurationConversion", func(t *testing.T) {
		type DurationStruct struct {
			Timeout time.Duration `json:"timeout"`
		}

		input := map[string]any{
			"timeout": "5m30s",
		}

		result, err := FromMap[DurationStruct](input)
		require.NoError(t, err, "Duration conversion should succeed")

		expected, _ := time.ParseDuration("5m30s")
		assert.Equal(t, expected, result.Timeout, "Duration should match")
	})

	t.Run("URLConversion", func(t *testing.T) {
		type URLStruct struct {
			Website *url.URL `json:"website"`
		}

		input := map[string]any{
			"website": "https://example.com/path?param=value",
		}

		result, err := FromMap[URLStruct](input)
		require.NoError(t, err, "URL conversion should succeed")
		assert.Equal(t, "https://example.com/path?param=value", result.Website.String(), "URL should match")
	})

	t.Run("IPConversion", func(t *testing.T) {
		type IPStruct struct {
			Address net.IP `json:"address"`
		}

		input := map[string]any{
			"address": "192.168.1.100",
		}

		result, err := FromMap[IPStruct](input)
		require.NoError(t, err, "IP conversion should succeed")
		assert.Equal(t, "192.168.1.100", result.Address.String(), "IP address should match")
	})
}

// TestRoundTripConversion tests struct-to-map and back-to-struct conversion.
func TestRoundTripConversion(t *testing.T) {
	t.Run("StructToMapAndBack", func(t *testing.T) {
		testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		original := TestStruct{
			Name:     "John Doe",
			Age:      30,
			Email:    "john@example.com",
			Active:   true,
			Score:    95.5,
			Created:  testTime,
			Duration: time.Hour * 2,
		}

		mapResult, err := ToMap(original)
		require.NoError(t, err, "Struct to map should succeed")

		structResult, err := FromMap[TestStruct](mapResult)
		require.NoError(t, err, "Map to struct should succeed")

		assert.Equal(t, original.Name, structResult.Name, "Name should match")
		assert.Equal(t, original.Age, structResult.Age, "Age should match")
		assert.Equal(t, original.Email, structResult.Email, "Email should match")
		assert.Equal(t, original.Active, structResult.Active, "Active should match")
		assert.Equal(t, original.Score, structResult.Score, "Score should match")
	})
}

// TestNullBoolDecodeHook tests null.Bool type conversions.
func TestNullBoolDecodeHook(t *testing.T) {
	t.Run("NullBoolToBool", func(t *testing.T) {
		type StructWithBool struct {
			Active bool `json:"active"`
		}

		input := map[string]any{
			"active": null.BoolFrom(true),
		}

		result, err := FromMap[StructWithBool](input)
		require.NoError(t, err, "null.Bool to bool conversion should succeed")
		assert.True(t, result.Active, "Active should be true")
	})

	t.Run("BoolToNullBool", func(t *testing.T) {
		type StructWithNullBool struct {
			Active null.Bool `json:"active"`
		}

		input := map[string]any{
			"active": true,
		}

		result, err := FromMap[StructWithNullBool](input)
		require.NoError(t, err, "bool to null.Bool conversion should succeed")
		assert.True(t, result.Active.Valid, "null.Bool should be valid")
		assert.True(t, result.Active.Bool, "null.Bool value should be true")
	})

	t.Run("InvalidNullBoolToBool", func(t *testing.T) {
		type StructWithBool struct {
			Active bool `json:"active"`
		}

		input := map[string]any{
			"active": null.NewBool(true, false),
		}

		result, err := FromMap[StructWithBool](input)
		require.NoError(t, err, "Invalid null.Bool conversion should succeed")
		assert.False(t, result.Active, "Active should be false for invalid null.Bool")
	})

	t.Run("ToMapWithNullBool", func(t *testing.T) {
		type StructWithNullBool struct {
			Active null.Bool `json:"active"`
		}

		input := StructWithNullBool{
			Active: null.BoolFrom(true),
		}

		result, err := ToMap(input)
		require.NoError(t, err, "ToMap with null.Bool should succeed")

		t.Logf("Result type: %T, value: %v", result["active"], result["active"])

		if boolVal, ok := result["active"].(bool); ok {
			assert.True(t, boolVal, "Active should be true")
		} else if mapVal, ok := result["active"].(map[string]any); ok {
			assert.True(t, mapVal["Valid"].(bool), "Valid should be true")
			assert.True(t, mapVal["Bool"].(bool), "Bool should be true")
		} else {
			t.Fatalf("Unexpected type for active field: %T", result["active"])
		}
	})
}

// TestNullValueDecodeHook tests null.Value[T] type conversions.
func TestNullValueDecodeHook(t *testing.T) {
	t.Run("NullValueStringToString", func(t *testing.T) {
		type StructWithString struct {
			Name string `json:"name"`
		}

		input := map[string]any{
			"name": null.ValueFrom("John Doe"),
		}

		result, err := FromMap[StructWithString](input)
		require.NoError(t, err, "null.Value[string] to string conversion should succeed")
		assert.Equal(t, "John Doe", result.Name, "Name should match")
	})

	t.Run("StringToNullValueString", func(t *testing.T) {
		type StructWithNullString struct {
			Name null.Value[string] `json:"name"`
		}

		input := map[string]any{
			"name": "John Doe",
		}

		result, err := FromMap[StructWithNullString](input)
		require.NoError(t, err, "string to null.Value[string] conversion should succeed")
		assert.True(t, result.Name.Valid, "null.Value should be valid")
		assert.Equal(t, "John Doe", result.Name.V, "Value should match")
	})

	t.Run("NullValueIntToInt", func(t *testing.T) {
		type StructWithInt struct {
			Age int `json:"age"`
		}

		input := map[string]any{
			"age": null.ValueFrom(30),
		}

		result, err := FromMap[StructWithInt](input)
		require.NoError(t, err, "null.Value[int] to int conversion should succeed")
		assert.Equal(t, 30, result.Age, "Age should match")
	})

	t.Run("IntToNullValueInt", func(t *testing.T) {
		type StructWithNullInt struct {
			Age null.Value[int] `json:"age"`
		}

		input := map[string]any{
			"age": 30,
		}

		result, err := FromMap[StructWithNullInt](input)
		require.NoError(t, err, "int to null.Value[int] conversion should succeed")
		assert.True(t, result.Age.Valid, "null.Value should be valid")
		assert.Equal(t, 30, result.Age.V, "Value should match")
	})

	t.Run("InvalidNullValueToPrimitive", func(t *testing.T) {
		type StructWithString struct {
			Name string `json:"name"`
		}

		input := map[string]any{
			"name": null.NewValue("John", false),
		}

		result, err := FromMap[StructWithString](input)
		require.NoError(t, err, "Invalid null.Value conversion should succeed")
		assert.Equal(t, "", result.Name, "Name should be zero value for invalid null.Value")
	})

	t.Run("ToMapWithNullValue", func(t *testing.T) {
		type StructWithNullValue struct {
			Name null.Value[string] `json:"name"`
			Age  null.Value[int]    `json:"age"`
		}

		input := StructWithNullValue{
			Name: null.ValueFrom("John Doe"),
			Age:  null.ValueFrom(30),
		}

		result, err := ToMap(input)
		require.NoError(t, err, "ToMap with null.Value should succeed")

		if nameVal, ok := result["name"].(string); ok {
			assert.Equal(t, "John Doe", nameVal, "Name should match")
		} else if mapVal, ok := result["name"].(map[string]any); ok {
			assert.True(t, mapVal["Valid"].(bool), "Valid should be true")
			assert.Equal(t, "John Doe", mapVal["V"], "Value should match")
		}

		if ageVal, ok := result["age"].(int); ok {
			assert.Equal(t, 30, ageVal, "Age should match")
		} else if mapVal, ok := result["age"].(map[string]any); ok {
			assert.True(t, mapVal["Valid"].(bool), "Valid should be true")
			assert.Equal(t, 30, mapVal["V"], "Value should match")
		}
	})
}

// TestNullTypesIntegration tests integration of various null types.
func TestNullTypesIntegration(t *testing.T) {
	t.Run("ComplexStructWithNullTypes", func(t *testing.T) {
		type ComplexStruct struct {
			Name          null.Value[string]  `json:"name"`
			Age           null.Value[int]     `json:"age"`
			Active        null.Bool           `json:"active"`
			Score         null.Value[float64] `json:"score"`
			OptionalField null.Value[string]  `json:"optionalField"`
		}

		input := map[string]any{
			"name":   "John Doe",
			"age":    30,
			"active": true,
			"score":  95.5,
		}

		result, err := FromMap[ComplexStruct](input)
		require.NoError(t, err, "Complex struct conversion should succeed")

		assert.True(t, result.Name.Valid, "Name should be valid")
		assert.Equal(t, "John Doe", result.Name.V, "Name should match")

		assert.True(t, result.Age.Valid, "Age should be valid")
		assert.Equal(t, 30, result.Age.V, "Age should match")

		assert.True(t, result.Active.Valid, "Active should be valid")
		assert.True(t, result.Active.Bool, "Active should be true")

		assert.True(t, result.Score.Valid, "Score should be valid")
		assert.Equal(t, 95.5, result.Score.V, "Score should match")

		assert.False(t, result.OptionalField.Valid, "Optional field should be invalid")
	})

	t.Run("RoundTripWithNullTypes", func(t *testing.T) {
		type NullStruct struct {
			Name   null.Value[string] `json:"name"`
			Age    null.Value[int]    `json:"age"`
			Active null.Bool          `json:"active"`
		}

		original := NullStruct{
			Name:   null.ValueFrom("Jane Doe"),
			Age:    null.ValueFrom(25),
			Active: null.BoolFrom(false),
		}

		mapResult, err := ToMap(original)
		require.NoError(t, err, "ToMap should succeed")

		nameMap, ok := mapResult["name"].(map[string]any)
		require.True(t, ok, "Name should be converted to map")
		assert.True(t, nameMap["Valid"].(bool), "Name Valid should be true")
		assert.Equal(t, "Jane Doe", nameMap["V"], "Name value should match")

		ageMap, ok := mapResult["age"].(map[string]any)
		require.True(t, ok, "Age should be converted to map")
		assert.True(t, ageMap["Valid"].(bool), "Age Valid should be true")
		assert.Equal(t, 25, ageMap["V"], "Age value should match")

		activeMap, ok := mapResult["active"].(map[string]any)
		require.True(t, ok, "Active should be converted to map")
		assert.True(t, activeMap["Valid"].(bool), "Active Valid should be true")
		assert.False(t, activeMap["Bool"].(bool), "Active Bool should be false")
	})

	t.Run("MixedNullAndRegularTypes", func(t *testing.T) {
		type MixedStruct struct {
			RegularName string             `json:"regularName"`
			NullName    null.Value[string] `json:"nullName"`
			RegularAge  int                `json:"regularAge"`
			NullAge     null.Value[int]    `json:"nullAge"`
			RegularFlag bool               `json:"regularFlag"`
			NullFlag    null.Bool          `json:"nullFlag"`
		}

		input := map[string]any{
			"regularName": "John",
			"nullName":    "Jane",
			"regularAge":  30,
			"nullAge":     25,
			"regularFlag": true,
			"nullFlag":    false,
		}

		result, err := FromMap[MixedStruct](input)
		require.NoError(t, err, "Mixed struct conversion should succeed")

		assert.Equal(t, "John", result.RegularName, "Regular name should match")
		assert.Equal(t, "Jane", result.NullName.V, "Null name value should match")
		assert.True(t, result.NullName.Valid, "Null name should be valid")

		assert.Equal(t, 30, result.RegularAge, "Regular age should match")
		assert.Equal(t, 25, result.NullAge.V, "Null age value should match")
		assert.True(t, result.NullAge.Valid, "Null age should be valid")

		assert.True(t, result.RegularFlag, "Regular flag should be true")
		assert.False(t, result.NullFlag.Bool, "Null flag should be false")
		assert.True(t, result.NullFlag.Valid, "Null flag should be valid")
	})
}

// TestNullBoolBasicOperations tests null.Bool creation and operations.
func TestNullBoolBasicOperations(t *testing.T) {
	t.Run("BoolFromCreatesValid", func(t *testing.T) {
		b := null.BoolFrom(true)
		assert.True(t, b.Valid, "Should be valid")
		assert.True(t, b.Bool, "Bool should be true")
		assert.True(t, b.ValueOrZero(), "ValueOrZero should be true")
	})

	t.Run("BoolFromPtrWithNil", func(t *testing.T) {
		b := null.BoolFromPtr(nil)
		assert.False(t, b.Valid, "Should be invalid")
		assert.False(t, b.Bool, "Bool should be false")
		assert.False(t, b.ValueOrZero(), "ValueOrZero should be false")
	})

	t.Run("BoolFromPtrWithValue", func(t *testing.T) {
		value := true
		b := null.BoolFromPtr(&value)
		assert.True(t, b.Valid, "Should be valid")
		assert.True(t, b.Bool, "Bool should be true")
		assert.True(t, b.ValueOrZero(), "ValueOrZero should be true")
	})

	t.Run("NewBoolWithValidity", func(t *testing.T) {
		validBool := null.NewBool(true, true)
		assert.True(t, validBool.Valid, "Should be valid")
		assert.True(t, validBool.Bool, "Bool should be true")

		invalidBool := null.NewBool(true, false)
		assert.False(t, invalidBool.Valid, "Should be invalid")
		assert.True(t, invalidBool.Bool, "Bool should be true")
		assert.False(t, invalidBool.ValueOrZero(), "ValueOrZero should be false for invalid")
	})

	t.Run("ValueOrReturnsDefault", func(t *testing.T) {
		invalidBool := null.NewBool(false, false)
		assert.True(t, invalidBool.ValueOr(true), "Should return default for invalid")

		validBool := null.BoolFrom(false)
		assert.False(t, validBool.ValueOr(true), "Should return actual value for valid")
	})
}

// TestNullValueBasicOperations tests null.Value[T] creation and operations.
func TestNullValueBasicOperations(t *testing.T) {
	t.Run("ValueFromCreatesValid", func(t *testing.T) {
		str := null.ValueFrom("hello")
		assert.True(t, str.Valid, "Should be valid")
		assert.Equal(t, "hello", str.V, "Value should match")
		assert.Equal(t, "hello", str.ValueOrZero(), "ValueOrZero should match")
	})

	t.Run("ValueFromPtrWithNil", func(t *testing.T) {
		var nilStr *string

		str := null.ValueFromPtr(nilStr)
		assert.False(t, str.Valid, "Should be invalid")
		assert.Equal(t, "", str.ValueOrZero(), "ValueOrZero should be zero value")
	})

	t.Run("ValueFromPtrWithValue", func(t *testing.T) {
		value := "hello"
		str := null.ValueFromPtr(&value)
		assert.True(t, str.Valid, "Should be valid")
		assert.Equal(t, "hello", str.V, "Value should match")
		assert.Equal(t, "hello", str.ValueOrZero(), "ValueOrZero should match")
	})

	t.Run("NewValueWithValidity", func(t *testing.T) {
		validValue := null.NewValue("hello", true)
		assert.True(t, validValue.Valid, "Should be valid")
		assert.Equal(t, "hello", validValue.V, "Value should match")

		invalidValue := null.NewValue("hello", false)
		assert.False(t, invalidValue.Valid, "Should be invalid")
		assert.Equal(t, "hello", invalidValue.V, "Value should match")
		assert.Equal(t, "", invalidValue.ValueOrZero(), "ValueOrZero should be zero value for invalid")
	})

	t.Run("NullValueWithDifferentTypes", func(t *testing.T) {
		intVal := null.ValueFrom(42)
		assert.True(t, intVal.Valid, "Int should be valid")
		assert.Equal(t, 42, intVal.V, "Int value should match")
		assert.Equal(t, 42, intVal.ValueOrZero(), "Int ValueOrZero should match")

		floatVal := null.ValueFrom(3.14)
		assert.True(t, floatVal.Valid, "Float should be valid")
		assert.Equal(t, 3.14, floatVal.V, "Float value should match")
		assert.Equal(t, 3.14, floatVal.ValueOrZero(), "Float ValueOrZero should match")

		boolVal := null.ValueFrom(true)
		assert.True(t, boolVal.Valid, "Bool should be valid")
		assert.True(t, boolVal.V, "Bool value should be true")
		assert.True(t, boolVal.ValueOrZero(), "Bool ValueOrZero should be true")
	})
}

// TestNullSpecificTypesDecodeHook tests specific null type conversions.
func TestNullSpecificTypesDecodeHook(t *testing.T) {
	t.Run("NullStringDecodeHook", func(t *testing.T) {
		type StructWithNullString struct {
			Name null.String `json:"name"`
		}

		input := map[string]any{
			"name": "John Doe",
		}

		result, err := FromMap[StructWithNullString](input)
		require.NoError(t, err, "String to null.String conversion should succeed")
		assert.True(t, result.Name.Valid, "Should be valid")
		assert.Equal(t, "John Doe", result.Name.String, "Value should match")

		type StructWithString struct {
			Name string `json:"name"`
		}

		input2 := map[string]any{
			"name": null.StringFrom("Jane Doe"),
		}

		result2, err := FromMap[StructWithString](input2)
		require.NoError(t, err, "null.String to string conversion should succeed")
		assert.Equal(t, "Jane Doe", result2.Name, "Value should match")

		input3 := map[string]any{
			"name": null.NewString("", false),
		}

		result3, err := FromMap[StructWithString](input3)
		require.NoError(t, err, "Invalid null.String conversion should succeed")
		assert.Equal(t, "", result3.Name, "Should be zero value for invalid")
	})

	t.Run("NullIntDecodeHook", func(t *testing.T) {
		type StructWithNullInt struct {
			Age null.Int `json:"age"`
		}

		input := map[string]any{
			"age": int64(30),
		}

		result, err := FromMap[StructWithNullInt](input)
		require.NoError(t, err, "int64 to null.Int conversion should succeed")
		assert.True(t, result.Age.Valid, "Should be valid")
		assert.Equal(t, int64(30), result.Age.Int64, "Value should match")

		type StructWithInt struct {
			Age int64 `json:"age"`
		}

		input2 := map[string]any{
			"age": null.IntFrom(25),
		}

		result2, err := FromMap[StructWithInt](input2)
		require.NoError(t, err, "null.Int to int64 conversion should succeed")
		assert.Equal(t, int64(25), result2.Age, "Value should match")
	})

	t.Run("NullInt16DecodeHook", func(t *testing.T) {
		type StructWithNullInt16 struct {
			Count null.Int16 `json:"count"`
		}

		input := map[string]any{
			"count": int16(100),
		}

		result, err := FromMap[StructWithNullInt16](input)
		require.NoError(t, err, "int16 to null.Int16 conversion should succeed")
		assert.True(t, result.Count.Valid, "Should be valid")
		assert.Equal(t, int16(100), result.Count.Int16, "Value should match")

		type StructWithInt16 struct {
			Count int16 `json:"count"`
		}

		input2 := map[string]any{
			"count": null.Int16From(200),
		}

		result2, err := FromMap[StructWithInt16](input2)
		require.NoError(t, err, "null.Int16 to int16 conversion should succeed")
		assert.Equal(t, int16(200), result2.Count, "Value should match")
	})

	t.Run("NullInt32DecodeHook", func(t *testing.T) {
		type StructWithNullInt32 struct {
			ID null.Int32 `json:"id"`
		}

		input := map[string]any{
			"id": int32(12345),
		}

		result, err := FromMap[StructWithNullInt32](input)
		require.NoError(t, err, "int32 to null.Int32 conversion should succeed")
		assert.True(t, result.ID.Valid, "Should be valid")
		assert.Equal(t, int32(12345), result.ID.Int32, "Value should match")

		type StructWithInt32 struct {
			ID int32 `json:"id"`
		}

		input2 := map[string]any{
			"id": null.Int32From(54321),
		}

		result2, err := FromMap[StructWithInt32](input2)
		require.NoError(t, err, "null.Int32 to int32 conversion should succeed")
		assert.Equal(t, int32(54321), result2.ID, "Value should match")
	})

	t.Run("NullFloatDecodeHook", func(t *testing.T) {
		type StructWithNullFloat struct {
			Score null.Float `json:"score"`
		}

		input := map[string]any{
			"score": float64(95.5),
		}

		result, err := FromMap[StructWithNullFloat](input)
		require.NoError(t, err, "float64 to null.Float conversion should succeed")
		assert.True(t, result.Score.Valid, "Should be valid")
		assert.Equal(t, 95.5, result.Score.Float64, "Value should match")

		type StructWithFloat struct {
			Score float64 `json:"score"`
		}

		input2 := map[string]any{
			"score": null.FloatFrom(87.3),
		}

		result2, err := FromMap[StructWithFloat](input2)
		require.NoError(t, err, "null.Float to float64 conversion should succeed")
		assert.Equal(t, 87.3, result2.Score, "Value should match")
	})

	t.Run("NullByteDecodeHook", func(t *testing.T) {
		type StructWithNullByte struct {
			Flag null.Byte `json:"flag"`
		}

		input := map[string]any{
			"flag": byte(255),
		}

		result, err := FromMap[StructWithNullByte](input)
		require.NoError(t, err, "byte to null.Byte conversion should succeed")
		assert.True(t, result.Flag.Valid, "Should be valid")
		assert.Equal(t, byte(255), result.Flag.Byte, "Value should match")

		type StructWithByte struct {
			Flag byte `json:"flag"`
		}

		input2 := map[string]any{
			"flag": null.ByteFrom(128),
		}

		result2, err := FromMap[StructWithByte](input2)
		require.NoError(t, err, "null.Byte to byte conversion should succeed")
		assert.Equal(t, byte(128), result2.Flag, "Value should match")
	})

	t.Run("NullDateTimeDecodeHook", func(t *testing.T) {
		testDateTime := timex.Of(time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC))

		type StructWithNullDateTime struct {
			Created null.DateTime `json:"created"`
		}

		input := map[string]any{
			"created": testDateTime,
		}

		result, err := FromMap[StructWithNullDateTime](input)
		require.NoError(t, err, "timex.DateTime to null.DateTime conversion should succeed")
		assert.True(t, result.Created.Valid, "Should be valid")
		assert.Equal(t, testDateTime, result.Created.V, "Value should match")

		type StructWithDateTime struct {
			Created timex.DateTime `json:"created"`
		}

		input2 := map[string]any{
			"created": null.DateTimeFrom(testDateTime),
		}

		result2, err := FromMap[StructWithDateTime](input2)
		require.NoError(t, err, "null.DateTime to timex.DateTime conversion should succeed")
		assert.Equal(t, testDateTime, result2.Created, "Value should match")
	})

	t.Run("NullDateDecodeHook", func(t *testing.T) {
		testDate := timex.DateOf(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC))

		type StructWithNullDate struct {
			Birthday null.Date `json:"birthday"`
		}

		input := map[string]any{
			"birthday": testDate,
		}

		result, err := FromMap[StructWithNullDate](input)
		require.NoError(t, err, "timex.Date to null.Date conversion should succeed")
		assert.True(t, result.Birthday.Valid, "Should be valid")
		assert.Equal(t, testDate, result.Birthday.V, "Value should match")

		type StructWithDate struct {
			Birthday timex.Date `json:"birthday"`
		}

		input2 := map[string]any{
			"birthday": null.DateFrom(testDate),
		}

		result2, err := FromMap[StructWithDate](input2)
		require.NoError(t, err, "null.Date to timex.Date conversion should succeed")
		assert.Equal(t, testDate, result2.Birthday, "Value should match")
	})

	t.Run("NullTimeDecodeHook", func(t *testing.T) {
		testTime := timex.TimeOf(time.Date(0, 1, 1, 15, 30, 45, 0, time.UTC))

		type StructWithNullTime struct {
			MeetingTime null.Time `json:"meetingTime"`
		}

		input := map[string]any{
			"meetingTime": testTime,
		}

		result, err := FromMap[StructWithNullTime](input)
		require.NoError(t, err, "timex.Time to null.Time conversion should succeed")
		assert.True(t, result.MeetingTime.Valid, "Should be valid")
		assert.Equal(t, testTime, result.MeetingTime.V, "Value should match")

		type StructWithTime struct {
			MeetingTime timex.Time `json:"meetingTime"`
		}

		input2 := map[string]any{
			"meetingTime": null.TimeFrom(testTime),
		}

		result2, err := FromMap[StructWithTime](input2)
		require.NoError(t, err, "null.Time to timex.Time conversion should succeed")
		assert.Equal(t, testTime, result2.MeetingTime, "Value should match")
	})

	t.Run("NullDecimalDecodeHook", func(t *testing.T) {
		testDecimal := decimal.NewFromFloat(123.456)

		type StructWithNullDecimal struct {
			Price null.Decimal `json:"price"`
		}

		input := map[string]any{
			"price": testDecimal,
		}

		result, err := FromMap[StructWithNullDecimal](input)
		require.NoError(t, err, "decimal.Decimal to null.Decimal conversion should succeed")
		assert.True(t, result.Price.Valid, "Should be valid")
		assert.True(t, testDecimal.Equal(result.Price.Decimal), "Value should match")

		type StructWithDecimal struct {
			Price decimal.Decimal `json:"price"`
		}

		input2 := map[string]any{
			"price": null.DecimalFrom(testDecimal),
		}

		result2, err := FromMap[StructWithDecimal](input2)
		require.NoError(t, err, "null.Decimal to decimal.Decimal conversion should succeed")
		assert.True(t, testDecimal.Equal(result2.Price), "Value should match")
	})
}

// TestNullTypesWithPointersDecodeHook tests null type conversions with pointers.
func TestNullTypesWithPointersDecodeHook(t *testing.T) {
	t.Run("PointerTypesConversion", func(t *testing.T) {
		type StructWithNullString struct {
			Name null.String `json:"name"`
		}

		stringVal := "John Doe"
		input := map[string]any{
			"name": &stringVal,
		}

		result, err := FromMap[StructWithNullString](input)
		require.NoError(t, err, "*string to null.String conversion should succeed")
		assert.True(t, result.Name.Valid, "Should be valid")
		assert.Equal(t, "John Doe", result.Name.String, "Value should match")

		type StructWithStringPtr struct {
			Name *string `json:"name"`
		}

		input2 := map[string]any{
			"name": null.StringFrom("Jane Doe"),
		}

		result2, err := FromMap[StructWithStringPtr](input2)
		require.NoError(t, err, "null.String to *string conversion should succeed")
		require.NotNil(t, result2.Name, "Pointer should not be nil")
		assert.Equal(t, "Jane Doe", *result2.Name, "Value should match")

		var nilString *string

		input3 := map[string]any{
			"name": nilString,
		}

		result3, err := FromMap[StructWithNullString](input3)
		require.NoError(t, err, "nil pointer conversion should succeed")
		assert.False(t, result3.Name.Valid, "Should be invalid for nil pointer")

		input4 := map[string]any{
			"name": null.NewString("test", false),
		}

		result4, err := FromMap[StructWithStringPtr](input4)
		require.NoError(t, err, "Invalid null.String to pointer conversion should succeed")
		assert.Nil(t, result4.Name, "Pointer should be nil for invalid null.String")
	})

	t.Run("IntegerPointerTypes", func(t *testing.T) {
		type StructWithNullInt struct {
			Age null.Int `json:"age"`
		}

		intVal := int64(30)
		input := map[string]any{
			"age": &intVal,
		}

		result, err := FromMap[StructWithNullInt](input)
		require.NoError(t, err, "*int64 to null.Int conversion should succeed")
		assert.True(t, result.Age.Valid, "Should be valid")
		assert.Equal(t, int64(30), result.Age.Int64, "Value should match")

		type StructWithIntPtr struct {
			Age *int64 `json:"age"`
		}

		input2 := map[string]any{
			"age": null.IntFrom(25),
		}

		result2, err := FromMap[StructWithIntPtr](input2)
		require.NoError(t, err, "null.Int to *int64 conversion should succeed")
		require.NotNil(t, result2.Age, "Pointer should not be nil")
		assert.Equal(t, int64(25), *result2.Age, "Value should match")
	})
}

// TestFileHeaderConversion tests multipart file header conversions.
func TestFileHeaderConversion(t *testing.T) {
	t.Run("SliceWithSingleFileToSinglePointer", func(t *testing.T) {
		type StructWithSingleFile struct {
			Avatar *multipart.FileHeader `json:"avatar"`
		}

		fileHeader := &multipart.FileHeader{
			Filename: "avatar.jpg",
			Size:     1024,
		}

		input := map[string]any{
			"avatar": []*multipart.FileHeader{fileHeader},
		}

		result, err := FromMap[StructWithSingleFile](input)
		require.NoError(t, err, "Slice to single file conversion should succeed")
		require.NotNil(t, result.Avatar, "Avatar should not be nil")
		assert.Equal(t, "avatar.jpg", result.Avatar.Filename, "Filename should match")
		assert.Equal(t, int64(1024), result.Avatar.Size, "Size should match")
	})

	t.Run("SliceWithMultipleFilesRemainsSlice", func(t *testing.T) {
		type StructWithMultipleFiles struct {
			Attachments []*multipart.FileHeader `json:"attachments"`
		}

		fileHeaders := []*multipart.FileHeader{
			{Filename: "file1.pdf", Size: 2048},
			{Filename: "file2.pdf", Size: 3072},
		}

		input := map[string]any{
			"attachments": fileHeaders,
		}

		result, err := FromMap[StructWithMultipleFiles](input)
		require.NoError(t, err, "Multiple files conversion should succeed")
		require.Len(t, result.Attachments, 2, "Should have 2 attachments")
		assert.Equal(t, "file1.pdf", result.Attachments[0].Filename, "First filename should match")
		assert.Equal(t, "file2.pdf", result.Attachments[1].Filename, "Second filename should match")
	})

	t.Run("EmptySliceToSinglePointer", func(t *testing.T) {
		type StructWithSingleFile struct {
			Avatar *multipart.FileHeader `json:"avatar"`
		}

		input := map[string]any{
			"avatar": []*multipart.FileHeader{},
		}

		_, err := FromMap[StructWithSingleFile](input)
		assert.Error(t, err, "Empty slice to pointer should error")
		assert.Contains(t, err.Error(), "expected a map or struct", "Error should mention type mismatch")
	})

	t.Run("SliceToSliceRemainsUnchanged", func(t *testing.T) {
		type StructWithFileSlice struct {
			Files []*multipart.FileHeader `json:"files"`
		}

		fileHeader := &multipart.FileHeader{
			Filename: "document.pdf",
			Size:     4096,
		}

		input := map[string]any{
			"files": []*multipart.FileHeader{fileHeader},
		}

		result, err := FromMap[StructWithFileSlice](input)
		require.NoError(t, err, "Slice to slice conversion should succeed")
		require.Len(t, result.Files, 1, "Should have 1 file")
		assert.Equal(t, "document.pdf", result.Files[0].Filename, "Filename should match")
		assert.Equal(t, int64(4096), result.Files[0].Size, "Size should match")
	})

	t.Run("NilSliceToSinglePointer", func(t *testing.T) {
		type StructWithSingleFile struct {
			Avatar *multipart.FileHeader `json:"avatar"`
		}

		input := map[string]any{
			"avatar": []*multipart.FileHeader(nil),
		}

		result, err := FromMap[StructWithSingleFile](input)
		require.NoError(t, err, "Nil slice conversion should succeed")
		assert.Nil(t, result.Avatar, "Avatar should be nil")
	})
}

// TestNullTypesIntegrationAdvanced tests comprehensive null type integration.
func TestNullTypesIntegrationAdvanced(t *testing.T) {
	t.Run("ComprehensiveStructWithAllNullTypes", func(t *testing.T) {
		type ComprehensiveStruct struct {
			Name        null.String   `json:"name"`
			Age         null.Int      `json:"age"`
			ShortCount  null.Int16    `json:"shortCount"`
			ID          null.Int32    `json:"id"`
			Score       null.Float    `json:"score"`
			Flag        null.Byte     `json:"flag"`
			Created     null.DateTime `json:"created"`
			Birthday    null.Date     `json:"birthday"`
			MeetingTime null.Time     `json:"meetingTime"`
			Price       null.Decimal  `json:"price"`
			Active      null.Bool     `json:"active"`
		}

		testDateTime := timex.Of(time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC))
		testDate := timex.DateOf(time.Date(1990, 5, 15, 0, 0, 0, 0, time.UTC))
		testTime := timex.TimeOf(time.Date(0, 1, 1, 14, 30, 0, 0, time.UTC))
		testDecimal := decimal.NewFromFloat(99.99)

		input := map[string]any{
			"name":        "John Doe",
			"age":         int64(30),
			"shortCount":  int16(100),
			"id":          int32(12345),
			"score":       95.5,
			"flag":        byte(255),
			"created":     testDateTime,
			"birthday":    testDate,
			"meetingTime": testTime,
			"price":       testDecimal,
			"active":      true,
		}

		result, err := FromMap[ComprehensiveStruct](input)
		require.NoError(t, err, "Comprehensive struct conversion should succeed")

		assert.True(t, result.Name.Valid, "Name should be valid")
		assert.Equal(t, "John Doe", result.Name.String, "Name should match")

		assert.True(t, result.Age.Valid, "Age should be valid")
		assert.Equal(t, int64(30), result.Age.Int64, "Age should match")

		assert.True(t, result.ShortCount.Valid, "ShortCount should be valid")
		assert.Equal(t, int16(100), result.ShortCount.Int16, "ShortCount should match")

		assert.True(t, result.ID.Valid, "ID should be valid")
		assert.Equal(t, int32(12345), result.ID.Int32, "ID should match")

		assert.True(t, result.Score.Valid, "Score should be valid")
		assert.Equal(t, 95.5, result.Score.Float64, "Score should match")

		assert.True(t, result.Flag.Valid, "Flag should be valid")
		assert.Equal(t, byte(255), result.Flag.Byte, "Flag should match")

		assert.True(t, result.Created.Valid, "Created should be valid")
		assert.Equal(t, testDateTime, result.Created.V, "Created should match")

		assert.True(t, result.Birthday.Valid, "Birthday should be valid")
		assert.Equal(t, testDate, result.Birthday.V, "Birthday should match")

		assert.True(t, result.MeetingTime.Valid, "MeetingTime should be valid")
		assert.Equal(t, testTime, result.MeetingTime.V, "MeetingTime should match")

		assert.True(t, result.Price.Valid, "Price should be valid")
		assert.True(t, testDecimal.Equal(result.Price.Decimal), "Price should match")

		assert.True(t, result.Active.Valid, "Active should be valid")
		assert.True(t, result.Active.Bool, "Active should be true")
	})

	t.Run("PartialInputWithSomeNullFields", func(t *testing.T) {
		type PartialStruct struct {
			Name   null.String `json:"name"`
			Age    null.Int    `json:"age"`
			Score  null.Float  `json:"score"`
			Active null.Bool   `json:"active"`
		}

		input := map[string]any{
			"name": "Jane Doe",
			"age":  int64(25),
		}

		result, err := FromMap[PartialStruct](input)
		require.NoError(t, err, "Partial input conversion should succeed")

		assert.True(t, result.Name.Valid, "Name should be valid")
		assert.Equal(t, "Jane Doe", result.Name.String, "Name should match")

		assert.True(t, result.Age.Valid, "Age should be valid")
		assert.Equal(t, int64(25), result.Age.Int64, "Age should match")

		assert.False(t, result.Score.Valid, "Score should be invalid")
		assert.False(t, result.Active.Valid, "Active should be invalid")
	})
}

// TestDecodeOrderDirection tests sort.OrderDirection type conversions.
func TestDecodeOrderDirection(t *testing.T) {
	t.Run("DecodeStringToOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Column    string               `json:"column"`
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"column":    "name",
			"direction": "asc",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, "name", result.Column, "Column should match")
		assert.Equal(t, sortx.OrderAsc, result.Direction, "Direction should be asc")
	})

	t.Run("DecodeUppercaseStringToOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"direction": "DESC",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, sortx.OrderDesc, result.Direction, "Direction should be desc")
	})

	t.Run("DecodeMixedCaseStringToOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		tests := []struct {
			name     string
			input    string
			expected sortx.OrderDirection
		}{
			{"Asc", "Asc", sortx.OrderAsc},
			{"AsC", "AsC", sortx.OrderAsc},
			{"Desc", "Desc", sortx.OrderDesc},
			{"DeSc", "DeSc", sortx.OrderDesc},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				input := map[string]any{
					"direction": tt.input,
				}

				var result SortSpec

				decoder, err := NewDecoder(&result)
				require.NoError(t, err, "Decoder creation should succeed")

				err = decoder.Decode(input)
				require.NoError(t, err, "Decode should succeed")

				assert.Equal(t, tt.expected, result.Direction, "Direction should match expected value")
			})
		}
	})

	t.Run("DecodeOrderDirectionWithSpaces", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"direction": " asc ",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, sortx.OrderAsc, result.Direction, "Direction should be asc with spaces trimmed")
	})

	t.Run("DecodeInvalidOrderDirectionValue", func(t *testing.T) {
		type SortSpec struct {
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"direction": "invalid",
		}

		var result SortSpec

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		assert.Error(t, err, "Invalid value should error")
		assert.Contains(t, err.Error(), "invalid OrderDirection value", "Error should mention invalid value")
	})

	t.Run("DecodeMultipleOrderDirectionInSlice", func(t *testing.T) {
		type SortRequest struct {
			Sort []sortx.OrderSpec `json:"sort"`
		}

		input := map[string]any{
			"sort": []map[string]any{
				{"column": "name", "direction": "asc"},
				{"column": "age", "direction": "desc"},
			},
		}

		var result SortRequest

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		require.Len(t, result.Sort, 2, "Should have 2 sort specs")
		assert.Equal(t, "name", result.Sort[0].Column, "First column should be name")
		assert.Equal(t, sortx.OrderAsc, result.Sort[0].Direction, "First direction should be asc")
		assert.Equal(t, "age", result.Sort[1].Column, "Second column should be age")
		assert.Equal(t, sortx.OrderDesc, result.Sort[1].Direction, "Second direction should be desc")
	})

	t.Run("DecodeNestedOrderDirectionInComplexStruct", func(t *testing.T) {
		type FilterSpec struct {
			Field    string `json:"field"`
			Operator string `json:"operator"`
			Value    any    `json:"value"`
		}

		type QueryRequest struct {
			Filters []FilterSpec      `json:"filters"`
			Sort    []sortx.OrderSpec `json:"sort"`
			Page    int               `json:"page"`
			Size    int               `json:"size"`
		}

		input := map[string]any{
			"filters": []map[string]any{
				{"field": "status", "operator": "eq", "value": "active"},
			},
			"sort": []map[string]any{
				{"column": "created_at", "direction": "desc"},
				{"column": "name", "direction": "asc"},
			},
			"page": 1,
			"size": 20,
		}

		var result QueryRequest

		decoder, err := NewDecoder(&result)
		require.NoError(t, err, "Decoder creation should succeed")

		err = decoder.Decode(input)
		require.NoError(t, err, "Decode should succeed")

		assert.Equal(t, 1, result.Page, "Page should match")
		assert.Equal(t, 20, result.Size, "Size should match")
		require.Len(t, result.Sort, 2, "Should have 2 sort specs")
		assert.Equal(t, "created_at", result.Sort[0].Column, "First column should be created_at")
		assert.Equal(t, sortx.OrderDesc, result.Sort[0].Direction, "First direction should be desc")
		assert.Equal(t, "name", result.Sort[1].Column, "Second column should be name")
		assert.Equal(t, sortx.OrderAsc, result.Sort[1].Direction, "Second direction should be asc")
	})

	t.Run("FromMapWithOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Column    string               `json:"column"`
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := map[string]any{
			"column":    "email",
			"direction": "desc",
		}

		result, err := FromMap[SortSpec](input)
		require.NoError(t, err, "FromMap should succeed")

		assert.Equal(t, "email", result.Column, "Column should match")
		assert.Equal(t, sortx.OrderDesc, result.Direction, "Direction should be desc")
	})

	t.Run("ToMapWithOrderDirection", func(t *testing.T) {
		type SortSpec struct {
			Column    string               `json:"column"`
			Direction sortx.OrderDirection `json:"direction"`
		}

		input := SortSpec{
			Column:    "username",
			Direction: sortx.OrderAsc,
		}

		result, err := ToMap(input)
		require.NoError(t, err, "ToMap should succeed")

		assert.Equal(t, "username", result["column"], "Column should match")
		assert.Equal(t, sortx.OrderAsc, result["direction"], "Direction should match")
	})
}
