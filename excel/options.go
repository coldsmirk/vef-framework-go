package excel

type exportConfig struct {
	sheetName string
}

type ExportOption func(*exportConfig)

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
}

type ImportOption func(*importConfig)

func WithImportSheetName(name string) ImportOption {
	return func(o *importConfig) {
		o.sheetName = name
	}
}

func WithImportSheetIndex(index int) ImportOption {
	return func(o *importConfig) {
		o.sheetIndex = index
	}
}

func WithSkipRows(rows int) ImportOption {
	return func(o *importConfig) {
		o.skipRows = rows
	}
}

// WithoutHeader treats the first non-skipped row as data instead of headers.
// Columns are mapped to source positions in schema order.
func WithoutHeader() ImportOption {
	return func(o *importConfig) {
		o.hasHeader = false
	}
}
