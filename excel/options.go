package excel

type exportConfig struct {
	sheetName string
}

// ExportOption configures the Excel exporter behavior.
type ExportOption func(*exportConfig)

// WithSheetName sets the worksheet name (default: "Sheet1").
func WithSheetName(name string) ExportOption {
	return func(o *exportConfig) {
		o.sheetName = name
	}
}

type importConfig struct {
	sheetName  string
	sheetIndex int
	skipRows   int
	hasHeader  bool
	trimSpace  bool
}

// ImportOption configures the Excel importer behavior.
type ImportOption func(*importConfig)

// WithImportSheetName selects the worksheet to import by name. When set, it
// takes precedence over WithImportSheetIndex.
func WithImportSheetName(name string) ImportOption {
	return func(o *importConfig) {
		o.sheetName = name
	}
}

// WithImportSheetIndex selects the worksheet to import by 0-based index
// (default: 0). Ignored when WithImportSheetName is set.
func WithImportSheetIndex(index int) ImportOption {
	return func(o *importConfig) {
		o.sheetIndex = index
	}
}

// WithSkipRows skips the first n rows before reading the header or data.
// Negative values are clamped to zero.
func WithSkipRows(rows int) ImportOption {
	return func(o *importConfig) {
		o.skipRows = max(rows, 0)
	}
}

// WithoutHeader treats the first non-skipped row as data instead of headers.
// Columns are mapped to source positions in schema order.
func WithoutHeader() ImportOption {
	return func(o *importConfig) {
		o.hasHeader = false
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
