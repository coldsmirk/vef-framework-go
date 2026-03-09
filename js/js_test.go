package js_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
)

// TestNew tests new functionality.
func TestNew(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "New() should create runtime successfully")
	require.NotNil(t, vm, "Runtime should not be nil")
}

// TestDayJs tests day js functionality.
func TestDayJs(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

	tests := []struct {
		name   string
		script string
		check  func(t *testing.T, result js.Value)
	}{
		{
			name:   "FormatCurrentDate",
			script: `dayjs().format('YYYY-MM-DD')`,
			check: func(t *testing.T, result js.Value) {
				formatted := result.String()
				assert.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, formatted, "Should match YYYY-MM-DD format")
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
			result, err := vm.RunString(tt.script)
			require.NoError(t, err, "Script should execute successfully")
			tt.check(t, result)
		})
	}
}

// TestBigJs tests big js functionality.
func TestBigJs(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

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
			result, err := vm.RunString(tt.script)
			require.NoError(t, err, "Script should execute successfully")
			assert.Equal(t, tt.want, result.String(), "Result should match expected value")
		})
	}
}

// TestRadashUtils tests radash utils functionality.
func TestRadashUtils(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

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
				const grouped = utils.group(users, u => u.role);
				Object.keys(grouped).sort().join(',')
			`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "admin,user", result.String(), "Should group by role")
			},
		},
		{
			name: "SortByKey",
			script: `
				const items = [{ price: 30 }, { price: 10 }, { price: 20 }];
				const sorted = utils.sort(items, i => i.price);
				sorted.map(i => i.price).join(',')
			`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "10,20,30", result.String(), "Should sort by price")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.RunString(tt.script)
			require.NoError(t, err, "Script should execute successfully")
			tt.check(t, result)
		})
	}
}

// TestValidatorJs tests validator js functionality.
func TestValidatorJs(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

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
			result, err := vm.RunString(tt.script)
			require.NoError(t, err, "Script should execute successfully")
			assert.Equal(t, tt.want, result.ToBoolean(), "Validation result should match expected")
		})
	}
}

// TestCombinedLibraries tests combined libraries functionality.
func TestCombinedLibraries(t *testing.T) {
	t.Run("DateFormattingAndValidation", func(t *testing.T) {
		vm, err := js.New()
		require.NoError(t, err, "Should not return error")

		script := `
			const date = dayjs('2025-01-15').format('YYYY-MM-DD');
			const isValid = validator.isISO8601(date);
			({ date, isValid })
		`
		result, err := vm.RunString(script)
		require.NoError(t, err, "Should not return error")

		obj := result.ToObject(vm)
		assert.Equal(t, "2025-01-15", obj.Get("date").String(), "Should equal expected value")
		assert.True(t, obj.Get("isValid").ToBoolean(), "Should be true")
	})

	t.Run("PriceCalculationWithFormatting", func(t *testing.T) {
		vm, err := js.New()
		require.NoError(t, err, "Should not return error")

		script := `
			const price = Big('19.99');
			const tax = Big('0.08');
			const total = price.times(tax.plus(1));
			const formatted = utils.capitalize('total: $') + total.toFixed(2);
			formatted
		`
		result, err := vm.RunString(script)
		require.NoError(t, err, "Should not return error")
		assert.Equal(t, "Total: $21.59", result.String(), "Should equal expected value")
	})

	t.Run("DataProcessingPipeline", func(t *testing.T) {
		vm, err := js.New()
		require.NoError(t, err, "Should not return error")

		script := `
			const data = [
				{ email: 'alice@example.com', amount: '10.50' },
				{ email: 'invalid-email', amount: '20.75' },
				{ email: 'bob@example.com', amount: '30.25' }
			];

			const valid = data.filter(item => validator.isEmail(item.email));
			const totalAmount = valid.reduce((sum, item) => sum.plus(Big(item.amount)), Big('0'));
			const count = valid.length;

			({ count, total: totalAmount.toString() })
		`
		result, err := vm.RunString(script)
		require.NoError(t, err, "Should not return error")

		obj := result.ToObject(vm)
		assert.Equal(t, int64(2), obj.Get("count").ToInteger(), "Should have 2 valid emails")
		assert.Equal(t, "40.75", obj.Get("total").String(), "Should sum valid amounts")
	})
}

// TestGoJavaScriptInterop tests go java script interop functionality.
func TestGoJavaScriptInterop(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

	t.Run("PassGoStructToJS", func(t *testing.T) {
		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Age   int    `json:"age"`
		}

		user := User{Name: "alice", Email: "alice@example.com", Age: 30}
		err := vm.Set("user", user)
		require.NoError(t, err, "Should not return error")

		script := `
			const capitalized = utils.capitalize(user.name);
			const isValidEmail = validator.isEmail(user.email);
			const isAdult = user.age >= 18;
			({ capitalized, isValidEmail, isAdult })
		`
		result, err := vm.RunString(script)
		require.NoError(t, err, "Should not return error")

		obj := result.ToObject(vm)
		assert.Equal(t, "Alice", obj.Get("capitalized").String(), "Should equal expected value")
		assert.True(t, obj.Get("isValidEmail").ToBoolean(), "Should be true")
		assert.True(t, obj.Get("isAdult").ToBoolean(), "Should be true")
	})

	t.Run("PassArrayToJS", func(t *testing.T) {
		numbers := []int{5, 2, 8, 1, 9}
		err := vm.Set("numbers", numbers)
		require.NoError(t, err, "Should not return error")

		script := `
			const sorted = utils.sort(numbers);
			const sum = utils.sum(numbers);
			const max = utils.max(numbers);
			({ sorted, sum, max })
		`
		result, err := vm.RunString(script)
		require.NoError(t, err, "Should not return error")

		obj := result.ToObject(vm)
		assert.Equal(t, int64(25), obj.Get("sum").ToInteger(), "Should equal expected value")
		assert.Equal(t, int64(9), obj.Get("max").ToInteger(), "Should equal expected value")
	})

	t.Run("ReturnComplexObject", func(t *testing.T) {
		script := `
			({
				timestamp: dayjs().format('YYYY-MM-DD HH:mm:ss'),
				calculation: Big('100').times('1.15').toString(),
				validation: {
					email: validator.isEmail('test@example.com'),
					uuid: validator.isUUID('550e8400-e29b-41d4-a716-446655440000')
				},
				processed: utils.capitalize('hello world')
			})
		`
		result, err := vm.RunString(script)
		require.NoError(t, err, "Should not return error")

		obj := result.ToObject(vm)
		assert.NotEmpty(t, obj.Get("timestamp").String(), "Should not be empty")
		assert.Equal(t, "115", obj.Get("calculation").String(), "Should equal expected value")
		assert.Equal(t, "Hello world", obj.Get("processed").String(), "Should equal expected value")

		validation := obj.Get("validation").ToObject(vm)
		assert.True(t, validation.Get("email").ToBoolean(), "Should be true")
		assert.True(t, validation.Get("uuid").ToBoolean(), "Should be true")
	})
}

// TestParse tests parse functionality.
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
			name:      "ValidObjectLiteral",
			script:    `const obj = { name: 'test', value: 123 };`,
			shouldErr: false,
		},
		{
			name:      "ValidArrayLiteral",
			script:    `const arr = [1, 2, 3];`,
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
			} else {
				assert.NoError(t, err, "Should parse valid syntax successfully")
				assert.NotNil(t, ast, "AST should not be nil for valid syntax")
			}
		})
	}
}

// TestTypeCheckers tests type checkers functionality.
func TestTypeCheckers(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

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
			result, err := vm.RunString(tt.script)
			require.NoError(t, err, "Should not return error")
			tt.check(t, result)
		})
	}
}

// TestErrorHandling tests error handling functionality.
func TestErrorHandling(t *testing.T) {
	vm, err := js.New()
	require.NoError(t, err, "Should not return error")

	t.Run("ReferenceError", func(t *testing.T) {
		_, err := vm.RunString(`nonExistentVariable`)
		assert.Error(t, err, "Should return error for undefined variable")
		assert.Contains(t, err.Error(), "not defined", "Error should mention undefined reference")
	})

	t.Run("SyntaxError", func(t *testing.T) {
		_, err := vm.RunString(`const x = ;`)
		assert.Error(t, err, "Should return error for syntax error")
	})

	t.Run("TypeError", func(t *testing.T) {
		_, err := vm.RunString(`null.toString()`)
		assert.Error(t, err, "Should return error for type error")
	})

	t.Run("LibraryFunctionError", func(t *testing.T) {
		_, err := vm.RunString(`Big('invalid')`)
		assert.Error(t, err, "Should return error for invalid Big.js input")
	})
}
