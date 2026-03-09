package excel

import (
	"fmt"
	"io"
	"reflect"

	"github.com/coldsmirk/go-streams"
	"github.com/xuri/excelize/v2"

	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/tabular"
	"github.com/coldsmirk/vef-framework-go/validator"
)

var logger = log.Named("excel")

type importer struct {
	schema  *tabular.Schema
	parsers map[string]tabular.ValueParser
	options importConfig
	typ     reflect.Type
}

func NewImporter(typ reflect.Type, opts ...ImportOption) tabular.Importer {
	options := importConfig{
		sheetIndex: 0,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &importer{
		schema:  tabular.NewSchema(typ),
		parsers: make(map[string]tabular.ValueParser),
		options: options,
		typ:     typ,
	}
}

func (i *importer) RegisterParser(name string, parser tabular.ValueParser) {
	i.parsers[name] = parser
}

func (i *importer) ImportFromFile(filename string) (any, []tabular.ImportError, error) {
	f, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open Excel file %s: %w", filename, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file %s: %v", filename, closeErr)
		}
	}()

	return i.doImport(f)
}

func (i *importer) Import(reader io.Reader) (any, []tabular.ImportError, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("open Excel from reader: %w", err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close Excel file from reader: %v", closeErr)
		}
	}()

	return i.doImport(f)
}

func (i *importer) doImport(f *excelize.File) (any, []tabular.ImportError, error) {
	sheetName := i.options.sheetName
	if sheetName == "" {
		sheets := f.GetSheetList()
		if i.options.sheetIndex >= len(sheets) {
			return nil, nil, fmt.Errorf("%w: %d (total sheets: %d)", ErrSheetIndexOutOfRange, i.options.sheetIndex, len(sheets))
		}

		sheetName = sheets[i.options.sheetIndex]
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("get rows: %w", err)
	}

	if len(rows) <= i.options.skipRows+1 {
		return nil, nil, fmt.Errorf("%w (total rows: %d, skip rows: %d)", ErrNoDataRowsFound, len(rows), i.options.skipRows)
	}

	headerRowIdx := i.options.skipRows
	headerRow := rows[headerRowIdx]

	columnMapping, err := i.buildColumnMapping(headerRow)
	if err != nil {
		return nil, nil, fmt.Errorf("build column mapping: %w", err)
	}

	dataRows := rows[headerRowIdx+1:]
	resultSlice := reflect.MakeSlice(reflect.SliceOf(i.typ), 0, len(dataRows))

	var importErrors []tabular.ImportError

	for rowIdx, row := range dataRows {
		excelRow := headerRowIdx + rowIdx + 2

		if i.isEmptyRow(row) {
			continue
		}

		item, rowErrors := i.parseRow(row, columnMapping, excelRow)
		if len(rowErrors) > 0 {
			importErrors = append(importErrors, rowErrors...)

			continue
		}

		if err := validator.Validate(item); err != nil {
			importErrors = append(importErrors, tabular.ImportError{Row: excelRow, Err: fmt.Errorf("validation failed: %w", err)})

			continue
		}

		resultSlice = reflect.Append(resultSlice, reflect.ValueOf(item))
	}

	return resultSlice.Interface(), importErrors, nil
}

func (i *importer) buildColumnMapping(headerRow []string) (map[int]int, error) {
	columns := i.schema.Columns()
	mapping := make(map[int]int)

	// Use streams to build name-to-index mapping
	nameToSchemaIdx := streams.ToMap(
		streams.ZipWithIndex(streams.FromSlice(columns)).ToPairs(),
		func(p streams.Pair[int, *tabular.Column]) string { return p.Second.Name },
		func(p streams.Pair[int, *tabular.Column]) int { return p.First },
	)

	seen := make(map[string]bool)
	for excelIdx, headerName := range headerRow {
		if headerName == "" {
			continue
		}

		if seen[headerName] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateColumnName, headerName)
		}

		seen[headerName] = true

		if schemaIdx, ok := nameToSchemaIdx[headerName]; ok {
			mapping[excelIdx] = schemaIdx
		}
	}

	return mapping, nil
}

func (i *importer) parseRow(row []string, columnMapping map[int]int, excelRow int) (any, []tabular.ImportError) {
	result := reflect.New(i.typ).Elem()

	var errors []tabular.ImportError

	columns := i.schema.Columns()

	for excelIdx, schemaIdx := range columnMapping {
		col := columns[schemaIdx]

		var cellValue string
		if excelIdx < len(row) {
			cellValue = row[excelIdx]
		}

		if cellValue == "" && col.Default != "" {
			cellValue = col.Default
		}

		field := result.FieldByIndex(col.Index)
		if !field.CanSet() {
			errors = append(errors, tabular.ImportError{Row: excelRow, Column: col.Name, Field: field.Type().Name(), Err: ErrFieldNotSettable})

			continue
		}

		value, err := i.parseValue(cellValue, field.Type(), col)
		if err != nil {
			errors = append(errors, tabular.ImportError{Row: excelRow, Column: col.Name, Field: field.Type().Name(), Err: fmt.Errorf("parse value: %w", err)})

			continue
		}

		field.Set(reflect.ValueOf(value))
	}

	return result.Interface(), errors
}

func (i *importer) parseValue(cellValue string, targetType reflect.Type, col *tabular.Column) (any, error) {
	if col.Parser != "" {
		if parser, ok := i.parsers[col.Parser]; ok {
			return parser.Parse(cellValue, targetType)
		}

		logger.Warnf("Parser %s not found, using default parser", col.Parser)
	}

	parser := tabular.NewDefaultParser(col.Format)

	return parser.Parse(cellValue, targetType)
}

func (*importer) isEmptyRow(row []string) bool {
	return streams.FromSlice(row).AllMatch(func(cell string) bool {
		return cell == ""
	})
}
