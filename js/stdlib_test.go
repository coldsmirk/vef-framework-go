package js_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// TestDayJs tests the embedded dayjs library.
func TestDayJs(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		check  func(t *testing.T, result js.Value)
	}{
		{
			name:   "FormatCurrentDate",
			script: `dayjs().format('YYYY-MM-DD')`,
			check: func(t *testing.T, result js.Value) {
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, result.String(), "Should match YYYY-MM-DD format")
			},
		},
		{
			name:   "DateArithmetic",
			script: `dayjs('2025-01-01').add(7, 'day').format('YYYY-MM-DD')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "2025-01-08", result.String(), "Should add 7 days correctly")
			},
		},
		{
			name:   "DateDifference",
			script: `dayjs('2025-01-10').diff(dayjs('2025-01-01'), 'day')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, int64(9), result.ToInteger(), "Should calculate 9 days difference")
			},
		},
		{
			name:   "ParseAndValidate",
			script: `dayjs('2025-01-01').isValid()`,
			check: func(t *testing.T, result js.Value) {
				assert.True(t, result.ToBoolean(), "Valid date should return true")
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

// TestBigJs tests the embedded big.js library.
func TestBigJs(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "PreciseDecimalAddition",
			script: `Big('0.1').plus('0.2').toString()`,
			want:   "0.3",
		},
		{
			name:   "PreciseDecimalMultiplication",
			script: `Big('19.99').times('1.08').toString()`,
			want:   "21.5892",
		},
		{
			name:   "PreciseDecimalDivision",
			script: `Big('10').div('3').toFixed(2)`,
			want:   "3.33",
		},
		{
			name:   "CompareNumbers",
			script: `Big('10.5').gt(Big('10.4'))`,
			want:   "true",
		},
		{
			name:   "ChainedOperations",
			script: `Big('100').minus('10').times('0.5').plus('5').toString()`,
			want:   "50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rt.RunString(t.Context(), tt.script)
			require.NoError(t, err, "Script should execute successfully")
			assert.Equal(t, tt.want, result.String(), "Result should match expected value")
		})
	}

	t.Run("InvalidInput", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `Big('invalid')`)
		require.Error(t, err, "Invalid Big.js input should return an error")
	})
}

// TestRadashUtils tests the embedded radash utils library.
func TestRadashUtils(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		check  func(t *testing.T, result js.Value)
	}{
		{
			name:   "CapitalizeString",
			script: `utils.capitalize('hello world')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "Hello world", result.String(), "Should capitalize first letter")
			},
		},
		{
			name:   "CamelCase",
			script: `utils.camel('user-name')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "userName", result.String(), "Should convert to camelCase")
			},
		},
		{
			name:   "SnakeCase",
			script: `utils.snake('userName')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "user_name", result.String(), "Should convert to snake_case")
			},
		},
		{
			name:   "UniqueArray",
			script: `JSON.stringify(utils.unique([1, 2, 2, 3, 3, 4]))`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "[1,2,3,4]", result.String(), "Should remove duplicates")
			},
		},
		{
			name:   "SumArray",
			script: `utils.sum([1, 2, 3, 4, 5])`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, int64(15), result.ToInteger(), "Should sum array correctly")
			},
		},
		{
			name: "GroupByKey",
			script: `
				const users = [
					{ role: 'admin', name: 'Alice' },
					{ role: 'user', name: 'Bob' },
					{ role: 'admin', name: 'Charlie' }
				];
				Object.keys(utils.group(users, u => u.role)).sort().join(',')
			`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "admin,user", result.String(), "Should group by role")
			},
		},
		{
			name: "SortByKey",
			script: `
				const items = [{ price: 30 }, { price: 10 }, { price: 20 }];
				utils.sort(items, i => i.price).map(i => i.price).join(',')
			`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "10,20,30", result.String(), "Should sort by price")
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

// TestValidatorJs tests the embedded validator library.
func TestValidatorJs(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{
			name:   "ValidEmail",
			script: `validator.isEmail('test@example.com')`,
			want:   true,
		},
		{
			name:   "InvalidEmail",
			script: `validator.isEmail('invalid-email')`,
			want:   false,
		},
		{
			name:   "ValidURL",
			script: `validator.isURL('https://github.com/coldsmirk/vef-framework-go')`,
			want:   true,
		},
		{
			name:   "InvalidURL",
			script: `validator.isURL('not-a-url')`,
			want:   false,
		},
		{
			name:   "ValidUUID",
			script: `validator.isUUID('550e8400-e29b-41d4-a716-446655440000')`,
			want:   true,
		},
		{
			name:   "InvalidUUID",
			script: `validator.isUUID('not-a-uuid')`,
			want:   false,
		},
		{
			name:   "ValidJSON",
			script: `validator.isJSON('{"name":"test"}')`,
			want:   true,
		},
		{
			name:   "InvalidJSON",
			script: `validator.isJSON('{invalid json}')`,
			want:   false,
		},
		{
			name:   "ValidNumeric",
			script: `validator.isNumeric('12345')`,
			want:   true,
		},
		{
			name:   "InvalidNumeric",
			script: `validator.isNumeric('abc123')`,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rt.RunString(t.Context(), tt.script)
			require.NoError(t, err, "Script should execute successfully")
			assert.Equal(t, tt.want, result.ToBoolean(), "Validation result should match expected")
		})
	}
}

// TestCombinedLibraries tests standard libraries working together.
func TestCombinedLibraries(t *testing.T) {
	t.Run("DateFormattingAndValidation", func(t *testing.T) {
		rt := newStdRuntime(t)

		script := `
			const date = dayjs('2025-01-15').format('YYYY-MM-DD');
			const isValid = validator.isISO8601(date);
			({ date, isValid })
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")

		obj := result.ToObject(rt.VM())
		assert.Equal(t, "2025-01-15", obj.Get("date").String(), "Formatted date should match the expected output")
		assert.True(t, obj.Get("isValid").ToBoolean(), "Formatted date should be ISO8601-valid")
	})

	t.Run("PriceCalculationWithFormatting", func(t *testing.T) {
		rt := newStdRuntime(t)

		script := `
			const total = Big('19.99').times(Big('0.08').plus(1));
			utils.capitalize('total: $') + total.toFixed(2)
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "Total: $21.59", result.String(), "Combined calculation should match the expected output")
	})

	t.Run("DataProcessingPipeline", func(t *testing.T) {
		rt := newStdRuntime(t)

		script := `
			const data = [
				{ email: 'alice@example.com', amount: '10.50' },
				{ email: 'invalid-email', amount: '20.75' },
				{ email: 'bob@example.com', amount: '30.25' }
			];

			const valid = data.filter(item => validator.isEmail(item.email));
			const total = valid.reduce((sum, item) => sum.plus(Big(item.amount)), Big('0'));

			({ count: valid.length, total: total.toString() })
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")

		obj := result.ToObject(rt.VM())
		assert.Equal(t, int64(2), obj.Get("count").ToInteger(), "Should count only valid emails")
		assert.Equal(t, "40.75", obj.Get("total").String(), "Should sum only valid amounts")
	})
}
