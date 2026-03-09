package validator

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestDecimalMinValidation tests decimal min validation functionality.
func TestDecimalMinValidation(t *testing.T) {
	type testStruct struct {
		Value decimal.Decimal `validate:"dec_min=10.5" label:"最小值"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"ValidMinimum", "10.5", false},
		{"ValidAboveMinimum", "20.0", false},
		{"InvalidBelowMinimum", "5.0", true},
		{"InvalidZero", "0", true},
		{"ValidExactMinimum", "10.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, _ := decimal.NewFromString(tt.value)
			s := testStruct{Value: value}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for value: %s", tt.value)
				assert.Contains(t, err.Error(), "最小值", "Error message should contain label")
			} else {
				assert.NoError(t, err, "Should not return validation error for value: %s", tt.value)
			}
		})
	}
}

// TestDecimalMaxValidation tests decimal max validation functionality.
func TestDecimalMaxValidation(t *testing.T) {
	type testStruct struct {
		Value decimal.Decimal `validate:"dec_max=100" label:"最大值"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"ValidMaximum", "100", false},
		{"ValidBelowMaximum", "50.5", false},
		{"InvalidAboveMaximum", "150.0", true},
		{"ValidExactMaximum", "100.00", false},
		{"ValidZero", "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, _ := decimal.NewFromString(tt.value)
			s := testStruct{Value: value}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for value: %s", tt.value)
				assert.Contains(t, err.Error(), "最大值", "Error message should contain label")
			} else {
				assert.NoError(t, err, "Should not return validation error for value: %s", tt.value)
			}
		})
	}
}

// TestDecimalRangeValidation tests decimal range validation functionality.
func TestDecimalRangeValidation(t *testing.T) {
	type testStruct struct {
		Value decimal.Decimal `validate:"dec_min=1,dec_max=50" label:"范围值"`
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"ValidInRange", "25.5", false},
		{"ValidMinimumBoundary", "1", false},
		{"ValidMaximumBoundary", "50", false},
		{"InvalidBelowRange", "0.5", true},
		{"InvalidAboveRange", "51", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, _ := decimal.NewFromString(tt.value)
			s := testStruct{Value: value}

			err := Validate(&s)
			if tt.wantErr {
				assert.Error(t, err, "Should return validation error for value: %s", tt.value)
				assert.Contains(t, err.Error(), "范围值", "Error message should contain label")
			} else {
				assert.NoError(t, err, "Should not return validation error for value: %s", tt.value)
			}
		})
	}
}
