package tabular

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TypedRow struct {
	ID   int
	Name string
}

// TypedFakeImporter is a stub Importer used to verify TypedImporter wiring
// without depending on csv/excel implementations.
type TypedFakeImporter struct {
	value           any
	errs            []ImportError
	err             error
	registeredName  string
	registeredOnce  bool
	importFromFile  string
	importedFromIO  bool
}

func (f *TypedFakeImporter) RegisterParser(name string, _ ValueParser) {
	f.registeredName = name
	f.registeredOnce = true
}

func (f *TypedFakeImporter) ImportFromFile(filename string) (any, []ImportError, error) {
	f.importFromFile = filename
	return f.value, f.errs, f.err
}

func (f *TypedFakeImporter) Import(_ io.Reader) (any, []ImportError, error) {
	f.importedFromIO = true
	return f.value, f.errs, f.err
}

// TypedFakeExporter is a stub Exporter used to verify TypedExporter wiring.
type TypedFakeExporter struct {
	receivedRows    any
	exportToFile    string
	registeredName  string
	registeredOnce  bool
	buf             *bytes.Buffer
	err             error
}

func (f *TypedFakeExporter) RegisterFormatter(name string, _ Formatter) {
	f.registeredName = name
	f.registeredOnce = true
}

func (f *TypedFakeExporter) ExportToFile(data any, filename string) error {
	f.receivedRows = data
	f.exportToFile = filename
	return f.err
}

func (f *TypedFakeExporter) Export(data any) (*bytes.Buffer, error) {
	f.receivedRows = data
	if f.err != nil {
		return nil, f.err
	}
	if f.buf == nil {
		f.buf = bytes.NewBuffer(nil)
	}
	return f.buf, nil
}

// TestTypedImporterReturnsTypedSlice ensures Import unwraps []T from any.
func TestTypedImporterReturnsTypedSlice(t *testing.T) {
	want := []TypedRow{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"}}
	inner := &TypedFakeImporter{value: want}
	typed := NewTypedImporter[TypedRow](inner)

	rows, errs, err := typed.Import(bytes.NewReader(nil))
	require.NoError(t, err, "Import should succeed when inner importer succeeds")
	assert.Nil(t, errs, "errs should be propagated as-is")
	assert.Equal(t, want, rows, "rows should equal the inner result")
	assert.True(t, inner.importedFromIO, "Import should delegate to inner importer")
}

// TestTypedImporterImportFromFileDelegates ensures filename is forwarded and
// the typed slice is unwrapped.
func TestTypedImporterImportFromFileDelegates(t *testing.T) {
	want := []TypedRow{{ID: 7, Name: "Carol"}}
	inner := &TypedFakeImporter{value: want}
	typed := NewTypedImporter[TypedRow](inner)

	rows, _, err := typed.ImportFromFile("data.csv")
	require.NoError(t, err, "ImportFromFile should succeed when inner succeeds")
	assert.Equal(t, "data.csv", inner.importFromFile, "filename should be forwarded to inner")
	assert.Equal(t, want, rows, "rows should equal the inner result")
}

// TestTypedImporterPropagatesError preserves the underlying error and errs.
func TestTypedImporterPropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	wantErrs := []ImportError{{Row: 1, Err: wantErr}}
	inner := &TypedFakeImporter{value: nil, errs: wantErrs, err: wantErr}
	typed := NewTypedImporter[TypedRow](inner)

	rows, errs, err := typed.Import(bytes.NewReader(nil))
	require.ErrorIs(t, err, wantErr, "Import should propagate inner error")
	assert.Equal(t, wantErrs, errs, "errs should be propagated as-is")
	assert.Nil(t, rows, "rows should be nil when error returned")
}

// TestTypedImporterRejectsWrongElementType returns a descriptive error when
// the inner importer yields a slice of a different element type.
func TestTypedImporterRejectsWrongElementType(t *testing.T) {
	inner := &TypedFakeImporter{value: []string{"oops"}}
	typed := NewTypedImporter[TypedRow](inner)

	rows, _, err := typed.Import(bytes.NewReader(nil))
	require.Error(t, err, "Import should fail when inner returns wrong element type")
	assert.Contains(t, err.Error(), "[]string", "error should describe actual type")
	assert.Contains(t, err.Error(), "TypedRow", "error should describe expected element type")
	assert.Nil(t, rows, "rows should be nil on type mismatch")
}

// TestTypedImporterHandlesNilValue treats nil from inner as an empty slice.
func TestTypedImporterHandlesNilValue(t *testing.T) {
	inner := &TypedFakeImporter{value: nil}
	typed := NewTypedImporter[TypedRow](inner)

	rows, _, err := typed.Import(bytes.NewReader(nil))
	require.NoError(t, err, "nil value should not raise an error")
	assert.Nil(t, rows, "rows should be nil when inner returned nil")
}

// TestTypedImporterRegisterParserDelegates ensures wrapper passes through.
func TestTypedImporterRegisterParserDelegates(t *testing.T) {
	inner := &TypedFakeImporter{}
	typed := NewTypedImporter[TypedRow](inner)

	typed.RegisterParser("custom", nil)

	assert.True(t, inner.registeredOnce, "RegisterParser should delegate to inner importer")
	assert.Equal(t, "custom", inner.registeredName, "parser name should be forwarded")
	assert.Same(t, inner, typed.Inner(), "Inner should expose the wrapped importer")
}

// TestTypedExporterExportDelegates ensures Export forwards typed rows as any
// and returns the inner buffer.
func TestTypedExporterExportDelegates(t *testing.T) {
	rows := []TypedRow{{ID: 1, Name: "Alice"}}
	wantBuf := bytes.NewBufferString("payload")
	inner := &TypedFakeExporter{buf: wantBuf}
	typed := NewTypedExporter[TypedRow](inner)

	got, err := typed.Export(rows)
	require.NoError(t, err, "Export should succeed when inner succeeds")
	assert.Same(t, wantBuf, got, "Export should return the inner buffer")
	assert.Equal(t, rows, inner.receivedRows, "inner exporter should receive typed rows")
}

// TestTypedExporterExportPropagatesError surfaces the underlying error.
func TestTypedExporterExportPropagatesError(t *testing.T) {
	wantErr := errors.New("disk full")
	inner := &TypedFakeExporter{err: wantErr}
	typed := NewTypedExporter[TypedRow](inner)

	got, err := typed.Export([]TypedRow{{ID: 1}})
	require.ErrorIs(t, err, wantErr, "Export should propagate inner error")
	assert.Nil(t, got, "buffer should be nil on error")
}

// TestTypedExporterExportToFileDelegates ensures file path and rows reach
// the inner exporter.
func TestTypedExporterExportToFileDelegates(t *testing.T) {
	rows := []TypedRow{{ID: 9, Name: "Dan"}}
	inner := &TypedFakeExporter{}
	typed := NewTypedExporter[TypedRow](inner)

	require.NoError(t, typed.ExportToFile(rows, "out.csv"), "ExportToFile should succeed")
	assert.Equal(t, "out.csv", inner.exportToFile, "filename should be forwarded")
	assert.Equal(t, rows, inner.receivedRows, "inner exporter should receive typed rows")
}

// TestTypedExporterRegisterFormatterDelegates ensures wrapper passes through.
func TestTypedExporterRegisterFormatterDelegates(t *testing.T) {
	inner := &TypedFakeExporter{}
	typed := NewTypedExporter[TypedRow](inner)

	typed.RegisterFormatter("custom", nil)

	assert.True(t, inner.registeredOnce, "RegisterFormatter should delegate to inner exporter")
	assert.Equal(t, "custom", inner.registeredName, "formatter name should be forwarded")
	assert.Same(t, inner, typed.Inner(), "Inner should expose the wrapped exporter")
}
