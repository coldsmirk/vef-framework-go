package csv

type importConfig struct {
	delimiter rune
	hasHeader bool
	skipRows  int
	trimSpace bool
	comment   rune
}

type ImportOption func(*importConfig)

func WithImportDelimiter(delimiter rune) ImportOption {
	return func(o *importConfig) {
		o.delimiter = delimiter
	}
}

func WithoutHeader() ImportOption {
	return func(o *importConfig) {
		o.hasHeader = false
	}
}

func WithSkipRows(rows int) ImportOption {
	return func(o *importConfig) {
		o.skipRows = rows
	}
}

func WithoutTrimSpace() ImportOption {
	return func(o *importConfig) {
		o.trimSpace = false
	}
}

func WithComment(comment rune) ImportOption {
	return func(o *importConfig) {
		o.comment = comment
	}
}

type exportConfig struct {
	delimiter   rune
	writeHeader bool
	useCrlf     bool
}

type ExportOption func(*exportConfig)

func WithExportDelimiter(delimiter rune) ExportOption {
	return func(o *exportConfig) {
		o.delimiter = delimiter
	}
}

func WithoutWriteHeader() ExportOption {
	return func(o *exportConfig) {
		o.writeHeader = false
	}
}

// WithCrlf enables Windows-style line endings for compatibility with legacy systems.
func WithCrlf() ExportOption {
	return func(o *exportConfig) {
		o.useCrlf = true
	}
}
