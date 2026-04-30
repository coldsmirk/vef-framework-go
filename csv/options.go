package csv

type importConfig struct {
	delimiter rune
	hasHeader bool
	skipRows  int
	trimSpace bool
	comment   rune
}

// ImportOption configures the CSV importer behavior.
type ImportOption func(*importConfig)

// WithImportDelimiter sets the field delimiter (default: comma).
func WithImportDelimiter(delimiter rune) ImportOption {
	return func(o *importConfig) {
		o.delimiter = delimiter
	}
}

// WithoutHeader treats the first non-skipped row as data instead of headers.
// Columns are mapped to source positions in schema order.
func WithoutHeader() ImportOption {
	return func(o *importConfig) {
		o.hasHeader = false
	}
}

// WithSkipRows skips the first n rows before reading the header or data.
// Negative values are clamped to zero.
func WithSkipRows(rows int) ImportOption {
	return func(o *importConfig) {
		o.skipRows = max(rows, 0)
	}
}

// WithoutTrimSpace disables leading/trailing whitespace trimming on cell
// values. Trimming is enabled by default and affects empty-row detection,
// header matching, and cell value parsing.
func WithoutTrimSpace() ImportOption {
	return func(o *importConfig) {
		o.trimSpace = false
	}
}

// WithComment sets the comment character. Lines beginning with this rune are
// skipped by the CSV reader.
func WithComment(comment rune) ImportOption {
	return func(o *importConfig) {
		o.comment = comment
	}
}

type exportConfig struct {
	delimiter   rune
	writeHeader bool
	useCRLF     bool
}

// ExportOption configures the CSV exporter behavior.
type ExportOption func(*exportConfig)

// WithExportDelimiter sets the field delimiter for export (default: comma).
func WithExportDelimiter(delimiter rune) ExportOption {
	return func(o *exportConfig) {
		o.delimiter = delimiter
	}
}

// WithoutWriteHeader suppresses the header row in the exported CSV output.
func WithoutWriteHeader() ExportOption {
	return func(o *exportConfig) {
		o.writeHeader = false
	}
}

// WithCRLF enables Windows-style line endings for compatibility with legacy systems.
func WithCRLF() ExportOption {
	return func(o *exportConfig) {
		o.useCRLF = true
	}
}
