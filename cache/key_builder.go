package cache

import (
	"strings"
)

var defaultKeyBuilder = NewPrefixKeyBuilder("")

// Key builds a key with the default key builder.
func Key(keyParts ...string) string {
	return defaultKeyBuilder.Build(keyParts...)
}

// KeyBuilder defines the interface for building cache keys with different naming strategies.
type KeyBuilder interface {
	// Build constructs a cache key from the given base key
	Build(keyParts ...string) string
}

// PrefixKeyBuilder implements KeyBuilder with prefix-based naming strategy.
type PrefixKeyBuilder struct {
	prefix    string
	separator string
}

// NewPrefixKeyBuilder creates a new prefix-based key builder with default ":" separator.
func NewPrefixKeyBuilder(prefix string) *PrefixKeyBuilder {
	return &PrefixKeyBuilder{
		prefix:    prefix,
		separator: ":",
	}
}

// NewPrefixKeyBuilderWithSeparator creates a new prefix-based key builder with custom separator.
func NewPrefixKeyBuilderWithSeparator(prefix, separator string) *PrefixKeyBuilder {
	return &PrefixKeyBuilder{
		prefix:    prefix,
		separator: separator,
	}
}

// Build constructs a cache key with prefix.
func (k *PrefixKeyBuilder) Build(keyParts ...string) string {
	if k.prefix == "" {
		return strings.Join(keyParts, k.separator)
	}

	// When no keyParts provided, return prefix only (without trailing separator)
	if len(keyParts) == 0 {
		return k.prefix
	}

	return k.prefix + k.separator + strings.Join(keyParts, k.separator)
}
