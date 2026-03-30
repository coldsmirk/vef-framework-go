package validator

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidate tests the Validate function with various inputs.
func TestValidate(t *testing.T) {
	t.Run("ValidStruct", func(t *testing.T) {
		type Input struct {
			Name  string `validate:"required" label:"Name"`
			Email string `validate:"required,email" label:"Email"`
		}

		err := Validate(&Input{Name: "John", Email: "john@example.com"})
		assert.NoError(t, err, "Valid struct should pass validation")
	})

	t.Run("MissingRequiredField", func(t *testing.T) {
		type Input struct {
			Name string `validate:"required" label:"Name"`
		}

		err := Validate(&Input{Name: ""})

		require.Error(t, err, "Missing required field should fail")
		assert.Contains(t, err.Error(), "Name", "Error should contain field label")
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		type Input struct {
			Name  string `validate:"required" label:"Name"`
			Email string `validate:"required,email" label:"Email"`
		}

		err := Validate(&Input{})

		require.Error(t, err, "Multiple missing fields should fail")
	})

	t.Run("PointerStringValid", func(t *testing.T) {
		type Input struct {
			Name *string `validate:"required" label:"Name"`
		}

		err := Validate(&Input{Name: new("John")})
		assert.NoError(t, err, "Valid *string should pass")
	})

	t.Run("PointerStringNil", func(t *testing.T) {
		type Input struct {
			Name *string `validate:"required" label:"Name"`
		}

		err := Validate(&Input{Name: nil})

		require.Error(t, err, "Nil *string should fail required")
	})

	t.Run("PointerIntValid", func(t *testing.T) {
		type Input struct {
			Age *int `validate:"required" label:"Age"`
		}

		err := Validate(&Input{Age: new(25)})
		assert.NoError(t, err, "Valid *int should pass")
	})

	t.Run("PointerBoolValid", func(t *testing.T) {
		type Input struct {
			Active *bool `validate:"required" label:"Active"`
		}

		err := Validate(&Input{Active: new(true)})
		assert.NoError(t, err, "Valid *bool should pass")
	})

	t.Run("PointerFloat64Valid", func(t *testing.T) {
		type Input struct {
			Score *float64 `validate:"required" label:"Score"`
		}

		err := Validate(&Input{Score: new(9.5)})
		assert.NoError(t, err, "Valid *float64 should pass")
	})

	t.Run("DecimalWithNonDecimalType", func(t *testing.T) {
		type Input struct {
			Value string `validate:"dec_min=10" label:"Value"`
		}

		err := Validate(&Input{Value: "notadecimal"})
		require.Error(t, err, "Non-decimal type with dec_min should fail")
	})

	t.Run("DecimalWithInvalidParam", func(t *testing.T) {
		type Input struct {
			Value decimal.Decimal `validate:"dec_min=notanumber" label:"Value"`
		}

		err := Validate(&Input{Value: decimal.NewFromFloat(5.0)})
		require.Error(t, err, "Invalid dec_min param should fail")
	})
}

// TestReplacePlaceholders tests the replacePlaceholders method.
func TestReplacePlaceholders(t *testing.T) {
	rule := ValidationRule{}

	t.Run("SinglePlaceholder", func(t *testing.T) {
		result := rule.replacePlaceholders("{0}不能为空", []string{"Name"})
		assert.Equal(t, "Name不能为空", result, "Should replace single placeholder")
	})

	t.Run("MultiplePlaceholders", func(t *testing.T) {
		result := rule.replacePlaceholders("{0}的值必须在{1}和{2}之间", []string{"Age", "0", "150"})
		assert.Equal(t, "Age的值必须在0和150之间", result, "Should replace multiple placeholders")
	})

	t.Run("NoPlaceholders", func(t *testing.T) {
		result := rule.replacePlaceholders("static message", []string{"unused"})
		assert.Equal(t, "static message", result, "Should return message unchanged")
	})

	t.Run("EmptyParams", func(t *testing.T) {
		result := rule.replacePlaceholders("{0}不能为空", []string{})
		assert.Equal(t, "{0}不能为空", result, "Should keep unreplaced placeholders")
	})

	t.Run("EmptyMessage", func(t *testing.T) {
		result := rule.replacePlaceholders("", []string{"A"})
		assert.Empty(t, result, "Empty message should return empty")
	})
}
