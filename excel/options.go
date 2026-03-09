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
