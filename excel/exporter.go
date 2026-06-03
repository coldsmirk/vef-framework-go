package excel

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/tabular"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// exporter writes rows into an Excel workbook via a tabular.RowAdapter.
type exporter struct {
	adapter    tabular.RowAdapter
	formatters map[string]tabular.Formatter
	options    exportConfig
}

// NewExporter creates an Excel exporter driven by the provided RowAdapter.
func NewExporter(adapter tabular.RowAdapter, opts ...ExportOption) tabular.Exporter {
	options := exportConfig{
		sheetName: "Sheet1",
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &exporter{
		adapter:    adapter,
		formatters: make(map[string]tabular.Formatter),
		options:    options,
	}
}

// RegisterFormatter registers a named formatter referenced by Column.Formatter.
func (e *exporter) RegisterFormatter(name string, formatter tabular.Formatter) {
	e.formatters[name] = formatter
}

// ExportToFile saves the exported Excel workbook to the given file path.
func (e *exporter) ExportToFile(data any, filename string) error {
	f, err := e.doExport(data)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file: %v", closeErr)
		}
	}()

	if err := f.SaveAs(filename); err != nil {
		return fmt.Errorf("save file %s: %w", filename, err)
	}

	return nil
}

// Export writes the workbook to a bytes.Buffer.
func (e *exporter) Export(data any) (*bytes.Buffer, error) {
	f, err := e.doExport(data)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file after export: %v", closeErr)
		}
	}()

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write to buffer: %w", err)
	}

	return buf, nil
}

func (e *exporter) doExport(data any) (*excelize.File, error) {
	f := excelize.NewFile()

	sheetIndex, err := f.GetSheetIndex(e.options.sheetName)
	if err != nil {
		return nil, fmt.Errorf("get sheet index: %w", err)
	}

	if sheetIndex == -1 {
		// Rename the default sheet rather than creating a second one.
		if err := f.SetSheetName("Sheet1", e.options.sheetName); err != nil {
			return nil, fmt.Errorf("rename sheet: %w", err)
		}

		sheetIndex, err = f.GetSheetIndex(e.options.sheetName)
		if err != nil {
			return nil, fmt.Errorf("get sheet index: %w", err)
		}
	}

	if err := e.writeHeader(f, e.options.sheetName); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	if err := e.writeData(f, e.options.sheetName, data); err != nil {
		return nil, fmt.Errorf("write data: %w", err)
	}

	f.SetActiveSheet(sheetIndex)

	return f, nil
}

func (e *exporter) writeHeader(f *excelize.File, sheetName string) error {
	columns := e.adapter.Schema().Columns()

	headerRow := make([]any, len(columns))
	for i, column := range columns {
		headerRow[i] = column.Name
	}

	if err := f.SetSheetRow(sheetName, "A1", &headerRow); err != nil {
		return fmt.Errorf("write header row: %w", err)
	}

	for i, column := range columns {
		if column.Width <= 0 {
			continue
		}

		colLetter, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return fmt.Errorf("convert column number to name: %w", err)
		}

		if err := f.SetColWidth(sheetName, colLetter, colLetter, column.Width); err != nil {
			return fmt.Errorf("set column width for %s: %w", colLetter, err)
		}
	}

	return nil
}

func (e *exporter) writeData(f *excelize.File, sheetName string, data any) error {
	schema := e.adapter.Schema()
	columns := schema.Columns()
	formatters := tabular.ResolveFormatters(schema, e.formatters)

	// A column writes a native (typed) cell when it relies on the default
	// formatter and declares no explicit Format. An explicit Format or a custom
	// formatter is an opt-in to a specific textual rendering, so it stays a
	// string; otherwise excelize stores int/float/time.Time as typed cells so
	// numbers stay summable and dates stay chronologically sortable.
	native := make([]bool, len(columns))
	for i, column := range columns {
		native[i] = column.Format == "" && tabular.IsDefaultFormatter(column, e.formatters)
	}

	reader, err := e.adapter.Reader(data)
	if err != nil {
		return err
	}

	for rowIndex, view := range reader.All() {
		excelRow := rowIndex + 2

		rowValues := make([]any, len(columns))

		for columnIndex, column := range columns {
			raw, err := view.Get(column)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIndex,
					Column: column.Name,
					Field:  column.Key,
					Err:    fmt.Errorf("read cell: %w", err),
				}
			}

			if native[columnIndex] {
				rowValues[columnIndex] = nativeCellValue(raw)

				continue
			}

			cellValue, err := formatters[columnIndex].Format(raw)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIndex,
					Column: column.Name,
					Field:  column.Key,
					Err:    fmt.Errorf("format value: %w", err),
				}
			}

			rowValues[columnIndex] = cellValue
		}

		startCell := fmt.Sprintf("A%d", excelRow)
		if err := f.SetSheetRow(sheetName, startCell, &rowValues); err != nil {
			return fmt.Errorf("set row %d: %w", excelRow, err)
		}
	}

	return nil
}

// nativeCellValue normalizes a raw cell value into a form excelize stores as a
// typed cell: nil pointers collapse to an empty cell and pointers are
// dereferenced. timex.DateTime / timex.Date are unwrapped to time.Time so
// excelize stores a native date(time) cell that sorts chronologically. Other
// values pass through unchanged so SetSheetRow's type switch handles ints,
// floats, bools and time.Time directly, and stringifies the rest.
//
// timex.Time (time-of-day) is deliberately left unwrapped: its underlying date
// is the zero date, which predates the Excel epoch and would render a bogus
// date, so excelize stringifies it via its String() method instead.
func nativeCellValue(raw any) any {
	if raw == nil {
		return nil
	}

	rv := reflect.ValueOf(raw)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}

		raw = rv.Elem().Interface()
	}

	switch v := raw.(type) {
	case timex.DateTime:
		return v.Unwrap()
	case timex.Date:
		return v.Unwrap()
	default:
		return raw
	}
}
