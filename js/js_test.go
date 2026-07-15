package js_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// TestParse tests JavaScript source parsing.
func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		script    string
		shouldErr bool
	}{
		{
			name:      "ValidScript",
			script:    `const x = 1 + 2;`,
			shouldErr: false,
		},
		{
			name:      "ValidFunction",
			script:    `function add(a, b) { return a + b; }`,
			shouldErr: false,
		},
		{
			name:      "ValidArrowFunction",
			script:    `const multiply = (a, b) => a * b;`,
			shouldErr: false,
		},
		{
			name:      "InvalidIncompleteExpression",
			script:    `const x = 1 +`,
			shouldErr: true,
		},
		{
			name:      "InvalidMissingBrace",
			script:    `function test() { return 1`,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := js.Parse("test.js", tt.script)

			if tt.shouldErr {
				assert.Error(t, err, "Should return error for invalid syntax")

				return
			}

			assert.NoError(t, err, "Should parse valid syntax successfully")
			assert.NotNil(t, ast, "AST should not be nil for valid syntax")
		})
	}
}

// TestTypeCheckers tests the goja value type checker aliases.
func TestTypeCheckers(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		check  func(t *testing.T, result js.Value)
	}{
		{
			name:   "IsString",
			script: `"hello"`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, js.IsString(result), "Should identify string")
				assert.False(t, js.IsNumber(result), "Should not identify as number")
			},
		},
		{
			name:   "IsNumber",
			script: `42`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, js.IsNumber(result), "Should identify number")
				assert.False(t, js.IsString(result), "Should not identify as string")
			},
		},
		{
			name:   "IsNaN",
			script: `NaN`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, js.IsNaN(result), "Should identify NaN")
			},
		},
		{
			name:   "IsInfinity",
			script: `Infinity`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, js.IsInfinity(result), "Should identify Infinity")
			},
		},
		{
			name:   "IsNull",
			script: `null`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, js.IsNull(result), "Should identify null")
				assert.False(t, js.IsUndefined(result), "Should not identify as undefined")
			},
		},
		{
			name:   "IsUndefined",
			script: `undefined`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, js.IsUndefined(result), "Should identify undefined")
				assert.False(t, js.IsNull(result), "Should not identify as null")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rt.RunString(t.Context(), tt.script)
			require.NoError(t, err, "Script should execute successfully")
			tt.check(t, result)
		})
	}
}
