package validator

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/null"
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

	t.Run("NullStringValid", func(t *testing.T) {
		type Input struct {
			Name null.String `validate:"required" label:"Name"`
		}

		err := Validate(&Input{Name: null.StringFrom("John")})
		assert.NoError(t, err, "Valid null.String should pass")
	})

	t.Run("NullStringInvalid", func(t *testing.T) {
		type Input struct {
			Name null.String `validate:"required" label:"Name"`
		}

		err := Validate(&Input{Name: null.String{}})

		require.Error(t, err, "Invalid null.String should fail required")
	})

	t.Run("NullIntValid", func(t *testing.T) {
		type Input struct {
			Age null.Int `validate:"required" label:"Age"`
		}

		err := Validate(&Input{Age: null.IntFrom(25)})
		assert.NoError(t, err, "Valid null.Int should pass")
	})

	t.Run("NullBoolValid", func(t *testing.T) {
		type Input struct {
			Active null.Bool `validate:"required" label:"Active"`
		}

		err := Validate(&Input{Active: null.BoolFrom(true)})
		assert.NoError(t, err, "Valid null.Bool should pass")
	})

	t.Run("NullFloatValid", func(t *testing.T) {
		type Input struct {
			Score null.Float `validate:"required" label:"Score"`
		}

		err := Validate(&Input{Score: null.FloatFrom(9.5)})
		assert.NoError(t, err, "Valid null.Float should pass")
	})

	t.Run("NullDecimalValid", func(t *testing.T) {
		type Input struct {
			Price null.Decimal `validate:"required" label:"Price"`
		}

		err := Validate(&Input{Price: null.DecimalFrom(decimal.NewFromFloat(19.99))})
		assert.NoError(t, err, "Valid null.Decimal should pass")
	})

	t.Run("NullInt16Valid", func(t *testing.T) {
		type Input struct {
			Code null.Int16 `validate:"required" label:"Code"`
		}

		err := Validate(&Input{Code: null.Int16From(42)})
		assert.NoError(t, err, "Valid null.Int16 should pass")
	})

	t.Run("NullInt32Valid", func(t *testing.T) {
		type Input struct {
			Count null.Int32 `validate:"required" label:"Count"`
		}

		err := Validate(&Input{Count: null.Int32From(100)})
		assert.NoError(t, err, "Valid null.Int32 should pass")
	})

	t.Run("NullByteValid", func(t *testing.T) {
		type Input struct {
			Flag null.Byte `validate:"required" label:"Flag"`
		}

		err := Validate(&Input{Flag: null.ByteFrom(1)})
		assert.NoError(t, err, "Valid null.Byte should pass")
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

// TestNullValue tests the nullValue helper function.
func TestNullValue(t *testing.T) {
	t.Run("ValidReturnsValue", func(t *testing.T) {
		result := nullValue(true, "hello")
		assert.Equal(t, "hello", result, "Valid should return the value")
	})

	t.Run("InvalidReturnsNil", func(t *testing.T) {
		result := nullValue(false, "hello")
		assert.Nil(t, result, "Invalid should return nil")
	})

	t.Run("ValidIntReturnsValue", func(t *testing.T) {
		result := nullValue(true, 42)
		assert.Equal(t, 42, result, "Valid int should return the value")
	})

	t.Run("InvalidIntReturnsNil", func(t *testing.T) {
		result := nullValue(false, 42)
		assert.Nil(t, result, "Invalid int should return nil")
	})

	t.Run("ValidBoolReturnsValue", func(t *testing.T) {
		result := nullValue(true, false)
		assert.Equal(t, false, result, "Valid bool should return the value even if false")
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

// TestRegisterNullValueTypeFunc tests RegisterNullValueTypeFunc registration.
func TestRegisterNullValueTypeFunc(t *testing.T) {
	t.Run("DoesNotPanic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RegisterNullValueTypeFunc[string]()
		}, "RegisterNullValueTypeFunc should not panic")
	})
}
