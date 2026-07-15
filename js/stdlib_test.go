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

// TestBigNumber tests the embedded bignumber.js library.
func TestBigNumber(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "PreciseDecimalAddition",
			script: `BigNumber('0.1').plus('0.2').toString()`,
			want:   "0.3",
		},
		{
			name:   "PreciseDecimalMultiplication",
			script: `BigNumber('19.99').times('1.08').toString()`,
			want:   "21.5892",
		},
		{
			name:   "PreciseDecimalDivision",
			script: `BigNumber('10').div('3').toFixed(2)`,
			want:   "3.33",
		},
		{
			name:   "CompareNumbers",
			script: `BigNumber('10.5').gt(BigNumber('10.4'))`,
			want:   "true",
		},
		{
			name:   "ChainedOperations",
			script: `BigNumber('100').minus('10').times('0.5').plus('5').toString()`,
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
		_, err := rt.RunString(t.Context(), `BigNumber('invalid')`)
		require.Error(t, err, "Invalid BigNumber input should return an error")
	})
}

// TestRadashi tests the embedded radashi library.
func TestRadashi(t *testing.T) {
	rt := newStdRuntime(t)

	tests := []struct {
		name   string
		script string
		check  func(t *testing.T, result js.Value)
	}{
		{
			name:   "CapitalizeString",
			script: `radashi.capitalize('hello world')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "Hello world", result.String(), "Should capitalize first letter")
			},
		},
		{
			name:   "CamelCase",
			script: `radashi.camel('user-name')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "userName", result.String(), "Should convert to camelCase")
			},
		},
		{
			name:   "SnakeCase",
			script: `radashi.snake('userName')`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "user_name", result.String(), "Should convert to snake_case")
			},
		},
		{
			name:   "UniqueArray",
			script: `JSON.stringify(radashi.unique([1, 2, 2, 3, 3, 4]))`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "[1,2,3,4]", result.String(), "Should remove duplicates")
			},
		},
		{
			name:   "SumArray",
			script: `radashi.sum([1, 2, 3, 4, 5])`,
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
				Object.keys(radashi.group(users, u => u.role)).sort().join(',')
			`,
			check: func(t *testing.T, result js.Value) {
				assert.Equal(t, "admin,user", result.String(), "Should group by role")
			},
		},
		{
			name: "SortByKey",
			script: `
				const items = [{ price: 30 }, { price: 10 }, { price: 20 }];
				radashi.sort(items, i => i.price).map(i => i.price).join(',')
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

// TestZod tests the embedded zod schema validation library.
func TestZod(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("ParseSuccess", func(t *testing.T) {
		script := `
			const User = z.object({
				name: z.string().min(2),
				age: z.coerce.number().int().positive(),
				email: z.email(),
				tags: z.array(z.string()).default([]),
			});
			JSON.stringify(User.parse({ name: 'vef', age: '3', email: 'a@b.co' }))
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.JSONEq(t, `{"name":"vef","age":3,"email":"a@b.co","tags":[]}`, result.String(), "Parse should coerce and apply defaults")
	})

	t.Run("SafeParseFailure", func(t *testing.T) {
		script := `
			const Form = z.object({ name: z.string().min(2), email: z.email() });
			const result = Form.safeParse({ name: 'v', email: 'nope' });
			JSON.stringify({ ok: result.success, issues: result.error.issues.map(i => i.code + ':' + i.path.join('.')) })
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.JSONEq(t, `{"ok":false,"issues":["too_small:name","invalid_format:email"]}`, result.String(), "safeParse should report structured issues per field")
	})

	t.Run("SchemaComposition", func(t *testing.T) {
		tests := []struct {
			name   string
			script string
			want   string
		}{
			{
				name:   "Transform",
				script: `z.string().transform(s => s.toUpperCase()).parse('ok')`,
				want:   "OK",
			},
			{
				name:   "Union",
				script: `String(z.union([z.string(), z.number()]).parse(7))`,
				want:   "7",
			},
			{
				name:   "RefineRejects",
				script: `String(z.number().refine(n => n % 2 === 0).safeParse(3).success)`,
				want:   "false",
			},
			{
				name:   "UUIDFormat",
				script: `String(z.uuid().safeParse('550e8400-e29b-41d4-a716-446655440000').success)`,
				want:   "true",
			},
			{
				name:   "URLFormat",
				script: `String(z.url().safeParse('not-a-url').success)`,
				want:   "false",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := rt.RunString(t.Context(), tt.script)
				require.NoError(t, err, "Script should execute successfully")
				assert.Equal(t, tt.want, result.String(), "Result should match expected value")
			})
		}
	})

	t.Run("ChineseDefaultLocale", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `z.string().min(2).safeParse('v').error.issues[0].message`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Contains(t, result.String(), "过小", "Issue messages should default to the zh-CN locale")
	})

	t.Run("EnglishLocaleOptIn", func(t *testing.T) {
		localeRt := newStdRuntime(t)

		script := `
			z.config(z.locales.en());
			z.string().min(2).safeParse('v').error.issues[0].message
		`

		result, err := localeRt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.Contains(t, result.String(), "Too small", "Scripts should be able to switch issue messages to English")
	})
}

// TestFxp tests the embedded fast-xml-parser library.
func TestFxp(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("Validate", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `String(fxp.XMLValidator.validate('<a><b/></a>') === true)`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "true", result.String(), "Well-formed XML should validate")
	})

	t.Run("ValidateReportsError", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `fxp.XMLValidator.validate('<a><b></a>').err.code`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "InvalidTag", result.String(), "Malformed XML should report a structured error")
	})

	t.Run("ParseRoundTrip", func(t *testing.T) {
		script := `
			const options = { ignoreAttributes: false, attributeNamePrefix: '@' };
			const parsed = new fxp.XMLParser(options).parse('<order id="7"><item qty="2">apple</item><item qty="1">pear</item></order>');
			const rebuilt = new fxp.XMLBuilder(options).build(parsed);
			JSON.stringify({ id: parsed.order['@id'], items: parsed.order.item.map(i => i['#text']), rebuilt })
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")

		want := `{"id":"7","items":["apple","pear"],"rebuilt":"<order id=\"7\"><item qty=\"2\">apple</item><item qty=\"1\">pear</item></order>"}`
		assert.JSONEq(t, want, result.String(), "Parse and build should round-trip attributes and repeated elements")
	})
}

// TestURL tests the embedded URL and URLSearchParams polyfills.
func TestURL(t *testing.T) {
	rt := newStdRuntime(t)

	t.Run("ComponentsAndEscaping", func(t *testing.T) {
		script := `
			const u = new URL('https://user:pass@example.com:8080/a b/c?x=1#frag');
			[u.protocol, u.username, u.hostname, u.port, u.pathname, u.hash].join('|')
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "https:|user|example.com|8080|/a%20b/c|#frag", result.String(), "URL components should parse and escape per spec")
	})

	t.Run("SearchParamsBuildQuery", func(t *testing.T) {
		script := `
			const api = new URL('https://api.example.com/search');
			api.searchParams.set('keyword', '审批 流程');
			api.searchParams.set('filter', 'a&b=c');
			api.href
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")

		want := "https://api.example.com/search?keyword=%E5%AE%A1%E6%89%B9+%E6%B5%81%E7%A8%8B&filter=a%26b%3Dc"
		assert.Equal(t, want, result.String(), "searchParams should escape query values including CJK and reserved characters")
	})

	t.Run("RelativeResolution", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `new URL('../up?q=2', 'https://host/base/dir/page').href`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "https://host/base/up?q=2", result.String(), "Relative URLs should resolve against the base")
	})

	t.Run("LiveBinding", func(t *testing.T) {
		script := `
			const live = new URL('https://example.com/?y=%E4%B8%AD');
			const wasDecoded = live.searchParams.get('y') === '中';
			live.searchParams.set('y', '新');
			wasDecoded && live.href.includes('y=%E6%96%B0')
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.True(t, result.ToBoolean(), "Mutating searchParams should reflect back into href")
	})

	t.Run("SearchParamsStandalone", func(t *testing.T) {
		script := `
			const p = new URLSearchParams('b=2&a=1&a=3');
			p.sort();
			[p.getAll('a').join(','), p.get('b'), p.toString()].join('|')
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "1,3|2|a=1&a=3&b=2", result.String(), "URLSearchParams should support multi-value keys and sort")
	})

	t.Run("IDNHostname", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `new URL('https://例子.中国/path').hostname`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "xn--fsqu00a.xn--fiqs8s", result.String(), "IDN hostnames should be punycode-encoded")
	})

	t.Run("InvalidURLThrows", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `new URL('not a url')`)
		require.Error(t, err, "An unparsable URL should throw")
	})
}

// TestCombinedLibraries tests standard libraries working together.
func TestCombinedLibraries(t *testing.T) {
	t.Run("DateFormattingAndValidation", func(t *testing.T) {
		rt := newStdRuntime(t)

		script := `
			const date = dayjs('2025-01-15').format('YYYY-MM-DD');
			const isValid = z.iso.date().safeParse(date).success;
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
			const total = BigNumber('19.99').times(BigNumber('0.08').plus(1));
			radashi.capitalize('total: $') + total.toFixed(2)
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

			const valid = data.filter(item => z.email().safeParse(item.email).success);
			const total = valid.reduce((sum, item) => sum.plus(BigNumber(item.amount)), BigNumber('0'));

			({ count: valid.length, total: total.toString() })
		`

		result, err := rt.RunString(t.Context(), script)
		require.NoError(t, err, "Script should execute successfully")

		obj := result.ToObject(rt.VM())
		assert.Equal(t, int64(2), obj.Get("count").ToInteger(), "Should count only valid emails")
		assert.Equal(t, "40.75", obj.Get("total").String(), "Should sum only valid amounts")
	})
}
