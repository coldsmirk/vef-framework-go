package tabular

import "iter"

// RowAdapter is the single bridge between the tabular engine (csv / excel
// importers and exporters) and any row model. Each adapter owns its Schema so
// callers only pass the adapter around; schema/model mismatches are impossible
// by construction.
type RowAdapter interface {
	// Schema returns the column definitions this adapter is built for.
	Schema() *Schema
	// Reader wraps the caller-provided data (e.g. []Struct, []map[string]any)
	// into an iterable sequence of read-only row views for export.
	Reader(data any) (RowReader, error)
	// Writer creates a result accumulator used by importers. The capacity hint
	// is advisory; adapters may ignore it.
	Writer(capacity int) RowWriter
}

// RowReader iterates over rows of export input.
type RowReader interface {
	// All yields 0-based row index and a RowView for each row.
	All() iter.Seq2[int, RowView]
}

// RowView exposes a read-only cell accessor for a single row.
type RowView interface {
	// Get returns the raw Go value held for the column. It may be nil for
	// missing map keys or zero struct fields.
	Get(col *Column) (any, error)
}

// RowWriter accumulates imported rows and produces the final result.
type RowWriter interface {
	// NewRow creates a fresh, writable row builder.
	NewRow() RowBuilder
	// Commit validates and appends the given row to the result set.
	Commit(row RowBuilder) error
	// Build returns the final aggregated result (e.g. []T or []map[string]any).
	Build() any
}

// RowBuilder represents a single row under construction during import.
type RowBuilder interface {
	// Set writes a parsed value for the given column.
	Set(col *Column, value any) error
	// Validate runs adapter-specific validation hooks (e.g. struct validator
	// or per-column cell validators plus row validators for maps).
	Validate() error
	// Value returns the underlying row representation (struct or map) in its
	// current state.
	Value() any
}
