package tabular

import (
	"bytes"
	"fmt"
	"io"
)

// TypedImporter wraps an Importer to return strongly-typed []T results,
// removing the need for callers to perform a type assertion on each call.
// It is intended for struct-backed adapters; map adapters return
// []map[string]any and should keep using the untyped Importer.
type TypedImporter[T any] struct {
	inner Importer
}

// NewTypedImporter wraps an existing Importer with strongly-typed result
// access. The underlying importer must produce []T values; otherwise Import
// and ImportFromFile will return a type-mismatch error at call time.
func NewTypedImporter[T any](inner Importer) TypedImporter[T] {
	return TypedImporter[T]{inner: inner}
}

// Inner returns the underlying untyped importer for advanced usage such as
// registering custom parsers without going through the typed wrapper.
func (i TypedImporter[T]) Inner() Importer {
	return i.inner
}

// RegisterParser delegates to the underlying importer.
func (i TypedImporter[T]) RegisterParser(name string, parser ValueParser) {
	i.inner.RegisterParser(name, parser)
}

// ImportFromFile imports rows from disk and returns them as []T.
func (i TypedImporter[T]) ImportFromFile(filename string) ([]T, []ImportError, error) {
	v, errs, err := i.inner.ImportFromFile(filename)
	return assertTypedRows[T](v, errs, err)
}

// Import imports rows from reader and returns them as []T.
func (i TypedImporter[T]) Import(reader io.Reader) ([]T, []ImportError, error) {
	v, errs, err := i.inner.Import(reader)
	return assertTypedRows[T](v, errs, err)
}

func assertTypedRows[T any](v any, errs []ImportError, err error) ([]T, []ImportError, error) {
	if err != nil {
		return nil, errs, err
	}
	if v == nil {
		return nil, errs, nil
	}
	rows, ok := v.([]T)
	if !ok {
		var zero T
		return nil, errs, fmt.Errorf("tabular: importer returned %T, expected []%T", v, zero)
	}
	return rows, errs, nil
}

// TypedExporter wraps an Exporter to accept strongly-typed []T input,
// keeping export call sites free of any-typed values.
type TypedExporter[T any] struct {
	inner Exporter
}

// NewTypedExporter wraps an existing Exporter with strongly-typed input.
func NewTypedExporter[T any](inner Exporter) TypedExporter[T] {
	return TypedExporter[T]{inner: inner}
}

// Inner returns the underlying untyped exporter for advanced usage such as
// registering custom formatters without going through the typed wrapper.
func (e TypedExporter[T]) Inner() Exporter {
	return e.inner
}

// RegisterFormatter delegates to the underlying exporter.
func (e TypedExporter[T]) RegisterFormatter(name string, formatter Formatter) {
	e.inner.RegisterFormatter(name, formatter)
}

// ExportToFile exports rows to a file on disk.
func (e TypedExporter[T]) ExportToFile(rows []T, filename string) error {
	return e.inner.ExportToFile(rows, filename)
}

// Export exports rows to an in-memory buffer.
func (e TypedExporter[T]) Export(rows []T) (*bytes.Buffer, error) {
	return e.inner.Export(rows)
}
