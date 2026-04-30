package csv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/coldsmirk/vef-framework-go/tabular"
)

// exporter writes rows to CSV sinks via a tabular.RowAdapter.
type exporter struct {
	adapter    tabular.RowAdapter
	formatters map[string]tabular.Formatter
	options    exportConfig
}

// NewExporter creates a CSV exporter driven by the provided RowAdapter.
func NewExporter(adapter tabular.RowAdapter, opts ...ExportOption) tabular.Exporter {
	options := exportConfig{
		delimiter:   ',',
		writeHeader: true,
		useCrlf:     false,
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

// ExportToFile writes the export output to the specified file.
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

// Export writes the export output to a bytes.Buffer.
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
	if err := csvWriter.Write(e.adapter.Schema().ColumnNames()); err != nil {
		return fmt.Errorf("write header row: %w", err)
	}

	return nil
}

func (e *exporter) writeData(csvWriter *csv.Writer, data any) error {
	columns := e.adapter.Schema().Columns()

	reader, err := e.adapter.Reader(data)
	if err != nil {
		return err
	}

	for rowIdx, view := range reader.All() {
		row := make([]string, len(columns))

		for colIdx, col := range columns {
			raw, err := view.Get(col)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIdx,
					Column: col.Name,
					Field:  col.Key,
					Err:    fmt.Errorf("read cell: %w", err),
				}
			}

			cellValue, err := tabular.ResolveFormatter(col, e.formatters).Format(raw)
			if err != nil {
				return tabular.ExportError{
					Row:    rowIdx,
					Column: col.Name,
					Field:  col.Key,
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
