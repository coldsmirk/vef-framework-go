package excel

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

type exporter struct {
	schema     *tabular.Schema
	formatters map[string]tabular.Formatter
	options    exportConfig
	typ        reflect.Type
}

func NewExporter(typ reflect.Type, opts ...ExportOption) tabular.Exporter {
	options := exportConfig{
		sheetName: "Sheet1",
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &exporter{
		schema:     tabular.NewSchema(typ),
		formatters: make(map[string]tabular.Formatter),
		options:    options,
		typ:        typ,
	}
}

func (e *exporter) RegisterFormatter(name string, formatter tabular.Formatter) {
	e.formatters[name] = formatter
}

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
		sheetIndex, err = f.NewSheet(e.options.sheetName)
		if err != nil {
			return nil, fmt.Errorf("create sheet: %w", err)
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
	columns := e.schema.Columns()

	for colIdx, col := range columns {
		colLetter, err := excelize.ColumnNumberToName(colIdx + 1)
		if err != nil {
			return fmt.Errorf("convert column number to name: %w", err)
		}

		cell := fmt.Sprintf("%s1", colLetter)
		if err := f.SetCellValue(sheetName, cell, col.Name); err != nil {
			return fmt.Errorf("set header cell %s: %w", cell, err)
		}

		if col.Width > 0 {
			if err := f.SetColWidth(sheetName, colLetter, colLetter, col.Width); err != nil {
				return fmt.Errorf("set column width for %s: %w", colLetter, err)
			}
		}
	}

	return nil
}

func (e *exporter) writeData(f *excelize.File, sheetName string, data any) error {
	columns := e.schema.Columns()

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Slice {
		return fmt.Errorf("%w, got %s", ErrDataMustBeSlice, dataValue.Kind())
	}

	for rowIdx := 0; rowIdx < dataValue.Len(); rowIdx++ {
		item := dataValue.Index(rowIdx)
		excelRow := rowIdx + 2

		for colIdx, col := range columns {
			fieldValue := item.FieldByIndex(col.Index)

			cellValue, err := e.formatValue(fieldValue.Interface(), col)
			if err != nil {
				return tabular.ExportError{Row: rowIdx, Column: col.Name, Field: fieldValue.Type().Name(), Err: fmt.Errorf("format value: %w", err)}
			}

			colLetter, err := excelize.ColumnNumberToName(colIdx + 1)
			if err != nil {
				return fmt.Errorf("convert column number to name: %w", err)
			}

			cell := fmt.Sprintf("%s%d", colLetter, excelRow)
			if err := f.SetCellValue(sheetName, cell, cellValue); err != nil {
				return fmt.Errorf("set cell %s: %w", cell, err)
			}
		}
	}

	return nil
}

func (e *exporter) formatValue(value any, col *tabular.Column) (string, error) {
	if col.Formatter != "" {
		if formatter, ok := e.formatters[col.Formatter]; ok {
			return formatter.Format(value)
		}

		logger.Warnf("Formatter %s not found, using default formatter", col.Formatter)
	}

	formatter := tabular.NewDefaultFormatter(col.Format)

	return formatter.Format(value)
}
