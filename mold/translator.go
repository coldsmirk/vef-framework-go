package mold

import "context"

// Translator translates field values to human-readable descriptions based on kind.
type Translator interface {
	// Supports returns true if the translator supports the given kind
	Supports(kind string) bool
	// Translate translates the current value to the corresponding description
	Translate(ctx context.Context, kind, value string) (string, error)
}

// DataDictResolver defines the data dictionary resolver interface for converting codes to readable names
// Supports multi-level data dictionaries, using key to distinguish different dictionary types.
type DataDictResolver interface {
	// Resolve resolves the corresponding name based on dictionary key and code value
	// Returns the translated name and an error if resolution fails
	Resolve(ctx context.Context, key, code string) (string, error)
}

// DataDictLoader defines the contract for loading dictionary entries by key.
// Implementations should return a map where the key is the dictionary item's code
// and the value is the translated/display name.
type DataDictLoader interface {
	// Load loads all dictionary entries for the given key, returning a code-to-name mapping.
	Load(ctx context.Context, key string) (map[string]string, error)
}
