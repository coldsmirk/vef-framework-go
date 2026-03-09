package csv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"reflect"

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
		delimiter:   ',',
		writeHeader: true,
		useCrlf:     false,
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
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create CSV file %s: %w", filename, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close CSV file %s: %v", filename, closeErr)
		}
	}()

	return e.writeToWriter(csv.NewWriter(f), data)
}

func (e *exporter) Export(data any) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	if err := e.writeToWriter(csv.NewWriter(buf), data); err != nil {
		return nil, err
	}

	return buf, nil
}

func (e *exporter) writeToWriter(csvWriter *csv.Writer, data any) error {
	csvWriter.Comma = e.options.delimiter
	csvWriter.UseCRLF = e.options.useCrlf

	if err := e.doExport(csvWriter, data); err != nil {
		return err
	}

	csvWriter.Flush()

	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("flush CSV writer: %w", err)
	}

	return nil
}

func (e *exporter) doExport(csvWriter *csv.Writer, data any) error {
	if e.options.writeHeader {
		if err := e.writeHeader(csvWriter); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
	}

	if err := e.writeData(csvWriter, data); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

func (e *exporter) writeHeader(csvWriter *csv.Writer) error {
	columns := e.schema.Columns()

	headerRow := make([]string, len(columns))
	for idx, col := range columns {
		headerRow[idx] = col.Name
	}

	if err := csvWriter.Write(headerRow); err != nil {
		return fmt.Errorf("write header row: %w", err)
	}

	return nil
}

func (e *exporter) writeData(csvWriter *csv.Writer, data any) error {
	columns := e.schema.Columns()

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Slice {
		return fmt.Errorf("%w, got %s", ErrDataMustBeSlice, dataValue.Kind())
	}

	for rowIdx := 0; rowIdx < dataValue.Len(); rowIdx++ {
		item := dataValue.Index(rowIdx)
		row := make([]string, len(columns))

		for colIdx, col := range columns {
			fieldValue := item.FieldByIndex(col.Index)

			cellValue, err := e.formatValue(fieldValue.Interface(), col)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIdx,
					Column: col.Name,
					Field:  fieldValue.Type().Name(),
					Err:    fmt.Errorf("format value: %w", err),
				}
			}

			row[colIdx] = cellValue
		}

		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("write row %d: %w", rowIdx, err)
		}
	}

	return nil
}

// formatValue falls back to default formatter when custom formatter is missing,
// preventing export failures due to configuration errors.
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
