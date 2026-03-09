package sortx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrderDirectionString tests order direction string functionality.
func TestOrderDirectionString(t *testing.T) {
	tests := []struct {
		name     string
		od       OrderDirection
		expected string
	}{
		{"AscendingOrder", OrderAsc, "ASC"},
		{"DescendingOrder", OrderDesc, "DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.od.String(), "String representation should match expected value")
		})
	}
}

// TestOrderDirectionMarshalText tests order direction marshal text functionality.
func TestOrderDirectionMarshalText(t *testing.T) {
	tests := []struct {
		name     string
		od       OrderDirection
		expected string
	}{
		{"AscendingOrder", OrderAsc, "asc"},
		{"DescendingOrder", OrderDesc, "desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := tt.od.MarshalText()
			require.NoError(t, err, "MarshalText should not error")
			assert.Equal(t, tt.expected, string(text), "Marshaled text should be lowercase")
		})
	}
}

// TestOrderDirectionUnmarshalText tests order direction unmarshal text functionality.
func TestOrderDirectionUnmarshalText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  OrderDirection
		shouldErr bool
	}{
		{"LowercaseAsc", "asc", OrderAsc, false},
		{"UppercaseASC", "ASC", OrderAsc, false},
		{"MixedCaseAsc", "Asc", OrderAsc, false},
		{"LowercaseDesc", "desc", OrderDesc, false},
		{"UppercaseDESC", "DESC", OrderDesc, false},
		{"MixedCaseDesc", "Desc", OrderDesc, false},
		{"WithLeadingSpace", " asc", OrderAsc, false},
		{"WithTrailingSpace", "desc ", OrderDesc, false},
		{"WithBothSpaces", " DESC ", OrderDesc, false},
		{"InvalidValue", "invalid", OrderAsc, true},
		{"EmptyString", "", OrderAsc, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var od OrderDirection

			err := od.UnmarshalText([]byte(tt.input))

			if tt.shouldErr {
				assert.Error(t, err, "Should error for invalid input")
			} else {
				require.NoError(t, err, "Should unmarshal valid input")
				assert.Equal(t, tt.expected, od, "Unmarshaled value should match expected")
			}
		})
	}
}

// TestOrderDirectionMarshalJSON tests order direction marshal JSON functionality.
func TestOrderDirectionMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		od       OrderDirection
		expected string
	}{
		{"AscendingOrder", OrderAsc, `"asc"`},
		{"DescendingOrder", OrderDesc, `"desc"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.od)
			require.NoError(t, err, "JSON marshaling should not error")
			assert.Equal(t, tt.expected, string(data), "JSON output should be lowercase string")
		})
	}
}

// TestOrderDirectionUnmarshalJSON tests order direction unmarshal JSON functionality.
func TestOrderDirectionUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  OrderDirection
		shouldErr bool
	}{
		{"LowercaseAsc", `"asc"`, OrderAsc, false},
		{"UppercaseASC", `"ASC"`, OrderAsc, false},
		{"MixedCaseAsc", `"Asc"`, OrderAsc, false},
		{"LowercaseDesc", `"desc"`, OrderDesc, false},
		{"UppercaseDESC", `"DESC"`, OrderDesc, false},
		{"MixedCaseDesc", `"Desc"`, OrderDesc, false},
		{"WithSpaces", `" desc "`, OrderDesc, false},
		{"InvalidValue", `"invalid"`, OrderAsc, true},
		{"NotAString", `123`, OrderAsc, true},
		{"BooleanValue", `true`, OrderAsc, true},
		{"NullValue", `null`, OrderAsc, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var od OrderDirection

			err := json.Unmarshal([]byte(tt.input), &od)

			if tt.shouldErr {
				assert.Error(t, err, "Should error for invalid JSON input")
			} else {
				require.NoError(t, err, "Should unmarshal valid JSON")
				assert.Equal(t, tt.expected, od, "Unmarshaled value should match expected")
			}
		})
	}
}

// TestOrderDirectionJSONRoundTrip tests order direction JSON round trip functionality.
func TestOrderDirectionJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input OrderDirection
	}{
		{"AscendingOrder", OrderAsc},
		{"DescendingOrder", OrderDesc},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err, "Marshal should not error")

			var result OrderDirection

			err = json.Unmarshal(data, &result)
			require.NoError(t, err, "Unmarshal should not error")

			assert.Equal(t, tt.input, result, "Round-trip should preserve value")
		})
	}
}

// TestOrderDirectionInStruct tests order direction in struct functionality.
func TestOrderDirectionInStruct(t *testing.T) {
	type testStruct struct {
		Direction OrderDirection `json:"direction"`
		Column    string         `json:"column"`
	}

	tests := []struct {
		name  string
		input testStruct
	}{
		{"WithAscending", testStruct{Direction: OrderAsc, Column: "name"}},
		{"WithDescending", testStruct{Direction: OrderDesc, Column: "created_at"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err, "Marshal should not error")

			var result testStruct

			err = json.Unmarshal(data, &result)
			require.NoError(t, err, "Unmarshal should not error")

			assert.Equal(t, tt.input.Direction, result.Direction, "Direction should be preserved")
			assert.Equal(t, tt.input.Column, result.Column, "Column should be preserved")
		})
	}
}

// TestNullsOrderString tests nulls order string functionality.
func TestNullsOrderString(t *testing.T) {
	tests := []struct {
		name     string
		no       NullsOrder
		expected string
	}{
		{"DefaultNullsOrder", NullsDefault, ""},
		{"NullsFirst", NullsFirst, "NULLS FIRST"},
		{"NullsLast", NullsLast, "NULLS LAST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.no.String(), "String representation should match expected value")
		})
	}
}

// TestOrderSpecIsValid tests order spec is valid functionality.
func TestOrderSpecIsValid(t *testing.T) {
	tests := []struct {
		name     string
		spec     OrderSpec
		expected bool
	}{
		{
			"ValidWithColumn",
			OrderSpec{Column: "name", Direction: OrderAsc},
			true,
		},
		{
			"ValidWithColumnAndNullsOrder",
			OrderSpec{Column: "age", Direction: OrderDesc, NullsOrder: NullsLast},
			true,
		},
		{
			"InvalidWithoutColumn",
			OrderSpec{Direction: OrderAsc},
			false,
		},
		{
			"InvalidWithEmptyColumn",
			OrderSpec{Column: "", Direction: OrderDesc},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.spec.IsValid(), "IsValid should return expected result")
		})
	}
}
