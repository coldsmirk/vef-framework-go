package mold

import "context"

// Translator translates field values to human-readable descriptions based on kind.
type Translator interface {
	// Supports returns true if the translator supports the given kind
	Supports(kind string) bool
	// Translate translates the current value to the corresponding description
	Translate(ctx context.Context, kind, value string) (string, error)
}

// CodeSetResolver resolves a code's display name within one host code set.
// Supports multiple code sets, using codeSet to distinguish between them.
type CodeSetResolver interface {
	// Resolve resolves the display name for the given code set and code value.
	// Returns the translated name and an error if resolution fails.
	Resolve(ctx context.Context, codeSet, code string) (string, error)
}

// CodeSetLoader loads all codes of one code set as a code-to-label map.
type CodeSetLoader interface {
	// Load loads all codes for the given code set, returning a code-to-label mapping.
	Load(ctx context.Context, codeSet string) (map[string]string, error)
}

// CodeSetInfo describes one code set the host catalog exposes.
type CodeSetInfo struct {
	// CodeSet is the code set identifier referenced by translate tags and
	// integration code maps (e.g. "gender").
	CodeSet string `json:"codeSet"`
	// Name is the code set's human-readable name.
	Name string `json:"name"`
}

// CodeInfo describes one code within a code set.
type CodeInfo struct {
	// Code is the canonical code value.
	Code string `json:"code"`
	// Label is the code's display name.
	Label string `json:"label"`
}

// CodeSetInspector is the optional enumeration view of the host code set
// catalog. A host implements it alongside its CodeSetLoader (or a
// wholesale-replaced CodeSetResolver); consumers type-assert for it
// (mirroring event.StreamInspector) and degrade gracefully when it is absent.
type CodeSetInspector interface {
	// ListCodeSets enumerates the code sets the host catalog exposes.
	ListCodeSets(ctx context.Context) ([]CodeSetInfo, error)
	// ListCodes enumerates one code set's codes with their display labels.
	ListCodes(ctx context.Context, codeSet string) ([]CodeInfo, error)
}
