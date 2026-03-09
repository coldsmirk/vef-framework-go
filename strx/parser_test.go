package strx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseTagCommaSeparated tests ParseTag comma separated scenarios.
func TestParseTagCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "SingleAttributeWithoutKey",
			input: "required",
			expected: map[string]string{
				DefaultKey: "required",
			},
		},
		{
			name:  "SingleAttributeWithKey",
			input: "min=10",
			expected: map[string]string{
				"min": "10",
			},
		},
		{
			name:  "MultipleAttributes",
			input: "required,min=5,max=100",
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "5",
				"max":      "100",
			},
		},
		{
			name:  "AttributesWithSpaces",
			input: " required , min=5 , max=100 ",
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "5",
				"max":      "100",
			},
		},
		{
			name:     "EmptyTag",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "TagWithEmptyAttributes",
			input: "required,,min=5",
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "5",
			},
		},
		{
			name:  "DuplicateDefaultAttributes",
			input: "required,optional",
			expected: map[string]string{
				DefaultKey: "required",
			},
		},
		{
			name:  "AttributeWithEmptyValue",
			input: "min=,max=100",
			expected: map[string]string{
				"min": "",
				"max": "100",
			},
		},
		{
			name:  "ComplexTag",
			input: "required,min=1,max=255,pattern=^[a-zA-Z0-9]+$",
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "1",
				"max":      "255",
				"pattern":  "^[a-zA-Z0-9]+$",
			},
		},
		{
			name:     "OnlyWhitespace",
			input:    "   ",
			expected: map[string]string{},
		},
		{
			name:     "OnlyCommas",
			input:    ",,,",
			expected: map[string]string{},
		},
		{
			name:  "KeyWithoutValue",
			input: "key=",
			expected: map[string]string{
				"key": "",
			},
		},
		{
			name:  "MultipleEqualsInValue",
			input: "url=http://example.com?a=1&b=2",
			expected: map[string]string{
				"url": "http://example.com?a=1&b=2",
			},
		},
		{
			name:  "SpecialCharactersInValue",
			input: "regex=^[a-z]+$,chars=!@#$%",
			expected: map[string]string{
				"regex": "^[a-z]+$",
				"chars": "!@#$%",
			},
		},
		{
			name:  "UnicodeCharacters",
			input: "名称=测试,value=中文",
			expected: map[string]string{
				"名称":    "测试",
				"value": "中文",
			},
		},
		{
			name:  "EmptyKeyName",
			input: "=value,key=data",
			expected: map[string]string{
				"":    "value",
				"key": "data",
			},
		},
		{
			name:  "DuplicateKeys",
			input: "key=first,key=second",
			expected: map[string]string{
				"key": "second",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTag(tt.input)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestParseTagSpaceSeparated tests ParseTag space separated scenarios.
func TestParseTagSpaceSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "SingleArgumentWithoutKey",
			input: "search",
			expected: map[string]string{
				DefaultKey: "search",
			},
		},
		{
			name:  "SingleArgumentWithKey",
			input: "q:golang",
			expected: map[string]string{
				"q": "golang",
			},
		},
		{
			name:  "MultipleArguments",
			input: "q:golang page:1 limit:10",
			expected: map[string]string{
				"q":     "golang",
				"page":  "1",
				"limit": "10",
			},
		},
		{
			name:  "ArgumentsWithExtraSpaces",
			input: " q:golang   page:1    limit:10 ",
			expected: map[string]string{
				"q":     "golang",
				"page":  "1",
				"limit": "10",
			},
		},
		{
			name:     "EmptyArgs",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "DuplicateDefaultArguments",
			input: "search filter",
			expected: map[string]string{
				DefaultKey: "search",
			},
		},
		{
			name:  "ArgsWithEmptyValue",
			input: "q: page:1",
			expected: map[string]string{
				"q":    "",
				"page": "1",
			},
		},
		{
			name:  "ComplexArgs",
			input: "q:web+framework category:backend sort:popularity order:desc",
			expected: map[string]string{
				"q":        "web+framework",
				"category": "backend",
				"sort":     "popularity",
				"order":    "desc",
			},
		},
		{
			name:  "ArgsWithEncodedCharacters",
			input: "q:hello%20world filter:type%3Darticle",
			expected: map[string]string{
				"q":      "hello%20world",
				"filter": "type%3Darticle",
			},
		},
		{
			name:  "TabSeparated",
			input: "q:test\tpage:1\tlimit:20",
			expected: map[string]string{
				"q":     "test",
				"page":  "1",
				"limit": "20",
			},
		},
		{
			name:  "MixedWhitespace",
			input: "q:test \t page:1  \t  limit:20",
			expected: map[string]string{
				"q":     "test",
				"page":  "1",
				"limit": "20",
			},
		},
		{
			name:  "NewlineAsWhitespace",
			input: "q:test\npage:1\nlimit:20",
			expected: map[string]string{
				"q":     "test",
				"page":  "1",
				"limit": "20",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTag(tt.input, WithSpacePairDelimiter(), WithValueDelimiter(':'))
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestParseTagCustomDelimiters tests ParseTag custom delimiters scenarios.
func TestParseTagCustomDelimiters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		opts     []ParseOption
		expected map[string]string
	}{
		{
			name:  "SemicolonSeparated",
			input: "required;min=5;max=100",
			opts:  []ParseOption{WithPairDelimiter(';')},
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "5",
				"max":      "100",
			},
		},
		{
			name:  "PipeSeparatedWithColon",
			input: "name:User|age:25|city:NYC",
			opts:  []ParseOption{WithPairDelimiter('|'), WithValueDelimiter(':')},
			expected: map[string]string{
				"name": "User",
				"age":  "25",
				"city": "NYC",
			},
		},
		{
			name:  "DotSeparatedWithColon",
			input: "host:localhost.port:8080.ssl:true",
			opts:  []ParseOption{WithPairDelimiter('.'), WithValueDelimiter(':')},
			expected: map[string]string{
				"host": "localhost",
				"port": "8080",
				"ssl":  "true",
			},
		},
		{
			name:  "CustomFuncMultipleDelimiters",
			input: "a=1,b=2;c=3,d=4",
			opts: []ParseOption{
				WithPairDelimiterFunc(func(r rune) bool {
					return r == ',' || r == ';'
				}),
			},
			expected: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
				"d": "4",
			},
		},
		{
			name:  "CustomFuncWithWhitespaceAndComma",
			input: "a=1 b=2,c=3 d=4",
			opts: []ParseOption{
				WithPairDelimiterFunc(func(r rune) bool {
					return r == ',' || r == ' '
				}),
			},
			expected: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
				"d": "4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTag(tt.input, tt.opts...)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestParseTagBareValueMode tests ParseTag bare value mode scenarios.
func TestParseTagBareValueMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		mode     BareValueMode
		opts     []ParseOption
		expected map[string]string
	}{
		{
			name:  "BareAsValue_Default",
			input: "required,min=5,max=100",
			mode:  BareAsValue,
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "5",
				"max":      "100",
			},
		},
		{
			name:  "BareAsValue_MultipleBareValues",
			input: "required,optional,min=5",
			mode:  BareAsValue,
			expected: map[string]string{
				DefaultKey: "required",
				"min":      "5",
			},
		},
		{
			name:  "BareAsKey_SingleBareValue",
			input: "required,min=5,max=100",
			mode:  BareAsKey,
			expected: map[string]string{
				"required": "",
				"min":      "5",
				"max":      "100",
			},
		},
		{
			name:  "BareAsKey_MultipleBareValues",
			input: "required,optional,validated,min=5",
			mode:  BareAsKey,
			expected: map[string]string{
				"required":  "",
				"optional":  "",
				"validated": "",
				"min":       "5",
			},
		},
		{
			name:  "BareAsKey_OnlyBareValues",
			input: "required,optional,validated",
			mode:  BareAsKey,
			expected: map[string]string{
				"required":  "",
				"optional":  "",
				"validated": "",
			},
		},
		{
			name:  "BareAsKey_WithSpaceSeparator",
			input: "btn primary large disabled",
			mode:  BareAsKey,
			opts:  []ParseOption{WithSpacePairDelimiter()},
			expected: map[string]string{
				"btn":      "",
				"primary":  "",
				"large":    "",
				"disabled": "",
			},
		},
		{
			name:  "BareAsKey_MixedWithKeyValues",
			input: "verbose debug level=info output=json",
			mode:  BareAsKey,
			opts:  []ParseOption{WithSpacePairDelimiter(), WithValueDelimiter('=')},
			expected: map[string]string{
				"verbose": "",
				"debug":   "",
				"level":   "info",
				"output":  "json",
			},
		},
		{
			name:  "BareAsValue_OnlyBareValue",
			input: "single",
			mode:  BareAsValue,
			expected: map[string]string{
				DefaultKey: "single",
			},
		},
		{
			name:     "BareAsValue_Empty",
			input:    "",
			mode:     BareAsValue,
			expected: map[string]string{},
		},
		{
			name:  "BareAsKey_DuplicateBareValues",
			input: "flag,flag,flag",
			mode:  BareAsKey,
			expected: map[string]string{
				"flag": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := make([]ParseOption, 0, 1+len(tt.opts))
			opts = append(opts, WithBareValueMode(tt.mode))
			opts = append(opts, tt.opts...)
			result := ParseTag(tt.input, opts...)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestParseTagEdgeCases tests ParseTag edge cases scenarios.
func TestParseTagEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		opts     []ParseOption
		expected map[string]string
	}{
		{
			name:     "OnlyDelimiters",
			input:    ",,,,",
			expected: map[string]string{},
		},
		{
			name:     "OnlyWhitespace",
			input:    "   \t\n  ",
			expected: map[string]string{},
		},
		{
			name:  "LeadingAndTrailingDelimiters",
			input: ",key=value,",
			expected: map[string]string{
				"key": "value",
			},
		},
		{
			name:  "ValueWithDelimiterCharacter",
			input: "url=http://a.com,data=a,b,c",
			expected: map[string]string{
				"url":      "http://a.com",
				"data":     "a",
				DefaultKey: "b",
			},
		},
		{
			name:  "KeyWithSpaces",
			input: " key name = value , other=data",
			expected: map[string]string{
				"key name ": " value",
				"other":     "data",
			},
		},
		{
			name:  "ValueWithSpaces",
			input: "key= value with spaces ,other=data",
			expected: map[string]string{
				"key":   " value with spaces",
				"other": "data",
			},
		},
		{
			name:  "VeryLongValue",
			input: "key=" + string(make([]byte, 1000)),
			expected: map[string]string{
				"key": string(make([]byte, 1000)),
			},
		},
		{
			name:  "ConsecutiveEqualsInValue",
			input: "math=a=b=c+d,other=1",
			expected: map[string]string{
				"math":  "a=b=c+d",
				"other": "1",
			},
		},
		{
			name:  "EmptyKeyAndValue",
			input: "=,key=value",
			expected: map[string]string{
				"":    "",
				"key": "value",
			},
		},
		{
			name:  "OnlyEqualsSign",
			input: "=",
			expected: map[string]string{
				"": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTag(tt.input, tt.opts...)
			assert.Equal(t, tt.expected, result, "Should equal expected value")
		})
	}
}

// TestParseTagOptionsOrdering tests ParseTag options ordering scenarios.
func TestParseTagOptionsOrdering(t *testing.T) {
	t.Run("MultipleOptionsAppliedInOrder", func(t *testing.T) {
		input := "a:1 b:2"

		result := ParseTag(input,
			WithSpacePairDelimiter(),
			WithValueDelimiter(':'),
			WithBareValueMode(BareAsKey),
		)

		expected := map[string]string{
			"a": "1",
			"b": "2",
		}
		assert.Equal(t, expected, result, "Should equal expected value")
	})

	t.Run("OverridingOptions", func(t *testing.T) {
		input := "a=1,b=2"

		result := ParseTag(input,
			WithPairDelimiter(';'),
			WithPairDelimiter(','),
		)

		expected := map[string]string{
			"a": "1",
			"b": "2",
		}
		assert.Equal(t, expected, result, "Should equal expected value")
	})
}
