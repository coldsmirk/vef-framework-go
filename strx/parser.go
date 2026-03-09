package strx

import (
	"strings"
	"unicode"

	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var logger = log.Named("strx")

const (
	// DefaultKey is the key used for values without an explicit key.
	DefaultKey = "__default"
)

// BareValueMode defines how to treat values without a separator.
type BareValueMode int

const (
	// BareAsValue treats bare values as values under DefaultKey (default behavior).
	// Example: "required,optional" → {"__default": "required"} (optional is ignored with warning).
	BareAsValue BareValueMode = iota
	// BareAsKey treats bare values as keys with empty values.
	// Example: "required,optional" → {"required": "", "optional": ""}.
	BareAsKey
)

// ParseOption configures the tag parser behavior.
type ParseOption func(*parseConfig)

type parseConfig struct {
	pairSeparator  func(rune) bool
	valueSeparator rune
	bareValueMode  BareValueMode
}

// defaultParseConfig returns comma-separated key=value pairs as the default format.
func defaultParseConfig() *parseConfig {
	return &parseConfig{
		pairSeparator:  func(r rune) bool { return r == ',' },
		valueSeparator: '=',
		bareValueMode:  BareAsValue,
	}
}

// ParseTag parses a tag string into a map of key-value pairs with configurable separators.
// By default, it parses comma-separated key=value pairs (e.g., "contains,column=name,operator=eq").
// Use ParseOption functions to customize the separator behavior.
func ParseTag(input string, opts ...ParseOption) map[string]string {
	cfg := defaultParseConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	result := make(map[string]string)

	for pair := range strings.FieldsFuncSeq(input, cfg.pairSeparator) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		idx := strings.IndexRune(pair, cfg.valueSeparator)
		if idx >= 0 {
			result[pair[:idx]] = pair[idx+1:]

			continue
		}

		if cfg.bareValueMode == BareAsKey {
			result[pair] = ""

			continue
		}

		if _, exists := result[DefaultKey]; exists {
			logger.Warnf("Ignoring duplicate default value %q in input: %s", pair, input)

			continue
		}

		result[DefaultKey] = pair
	}

	return result
}

// WithPairDelimiter uses a single rune to separate pairs.
func WithPairDelimiter(delimiter rune) ParseOption {
	return func(c *parseConfig) {
		c.pairSeparator = func(r rune) bool {
			return r == delimiter
		}
	}
}

// WithPairDelimiterFunc allows custom separator logic for complex cases.
func WithPairDelimiterFunc(fn func(rune) bool) ParseOption {
	return func(c *parseConfig) {
		c.pairSeparator = fn
	}
}

// WithSpacePairDelimiter is a convenient shorthand for space-separated formats
// commonly used in query strings and CLI arguments.
func WithSpacePairDelimiter() ParseOption {
	return func(c *parseConfig) {
		c.pairSeparator = unicode.IsSpace
	}
}

// WithValueDelimiter changes the key-value separator (default is '=').
func WithValueDelimiter(delimiter rune) ParseOption {
	return func(c *parseConfig) {
		c.valueSeparator = delimiter
	}
}

// WithBareValueMode sets how to treat values without a separator.
//
// BareAsValue (default): Treats bare values as values under DefaultKey.
// Only the first bare value is kept, subsequent ones are ignored with a warning.
// Example: ParseTag("required,optional") → {"__default": "required"} (optional ignored)
//
// BareAsKey: Treats bare values as keys with empty values.
// Multiple bare values are allowed.
// Example: ParseTag("required,optional", WithBareValueMode(BareAsKey)) → {"required": "", "optional": ""}.
func WithBareValueMode(mode BareValueMode) ParseOption {
	return func(c *parseConfig) {
		c.bareValueMode = mode
	}
}
