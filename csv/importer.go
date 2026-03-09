package csv

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/coldsmirk/vef-framework-go/internal/log"
	"github.com/coldsmirk/vef-framework-go/tabular"
	"github.com/coldsmirk/vef-framework-go/validator"
)

var logger = log.Named("csv")

type importer struct {
	schema  *tabular.Schema
	parsers map[string]tabular.ValueParser
	options importConfig
	typ     reflect.Type
}

func NewImporter(typ reflect.Type, opts ...ImportOption) tabular.Importer {
	options := importConfig{
		delimiter: ',',
		hasHeader: true,
		skipRows:  0,
		trimSpace: true,
		comment:   0,
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
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open CSV file %s: %w", filename, err)
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Errorf("Failed to close CSV file %s: %v", filename, closeErr)
		}
	}()

	return i.Import(f)
}

func (i *importer) Import(reader io.Reader) (any, []tabular.ImportError, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = i.options.delimiter
	csvReader.TrimLeadingSpace = i.options.trimSpace
	csvReader.Comment = i.options.comment
	csvReader.FieldsPerRecord = -1

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read CSV: %w", err)
	}

	minRows := i.options.skipRows
	if i.options.hasHeader {
		minRows++
	}

	if len(rows) <= minRows {
		return nil, nil, fmt.Errorf("%w (total rows: %d, skip rows: %d, has header: %v)",
			ErrNoDataRowsFound, len(rows), i.options.skipRows, i.options.hasHeader)
	}

	var columnMapping map[int]int

	dataStartIdx := i.options.skipRows

	if i.options.hasHeader {
		headerRow := rows[i.options.skipRows]

		var mappingErr error

		columnMapping, mappingErr = i.buildColumnMapping(headerRow)
		if mappingErr != nil {
			return nil, nil, fmt.Errorf("build column mapping: %w", mappingErr)
		}

		dataStartIdx++
	} else {
		columnMapping = i.buildDefaultMapping()
	}

	dataRows := rows[dataStartIdx:]
	resultSlice := reflect.MakeSlice(reflect.SliceOf(i.typ), 0, len(dataRows))

	var importErrors []tabular.ImportError

	for rowIdx, row := range dataRows {
		csvRow := dataStartIdx + rowIdx + 1

		if i.isEmptyRow(row) {
			continue
		}

		item, rowErrors := i.parseRow(row, columnMapping, csvRow)
		if len(rowErrors) > 0 {
			importErrors = append(importErrors, rowErrors...)

			continue
		}

		if err := validator.Validate(item); err != nil {
			importErrors = append(importErrors, tabular.ImportError{
				Row: csvRow,
				Err: fmt.Errorf("validation failed: %w", err),
			})

			continue
		}

		resultSlice = reflect.Append(resultSlice, reflect.ValueOf(item))
	}

	return resultSlice.Interface(), importErrors, nil
}

func (i *importer) buildColumnMapping(headerRow []string) (map[int]int, error) {
	columns := i.schema.Columns()
	mapping := make(map[int]int)

	nameToSchemaIdx := make(map[string]int, len(columns))
	for idx, col := range columns {
		nameToSchemaIdx[col.Name] = idx
	}

	seen := make(map[string]bool)
	for csvIdx, headerName := range headerRow {
		if i.options.trimSpace {
			headerName = strings.TrimSpace(headerName)
		}

		if headerName == "" {
			continue
		}

		if seen[headerName] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateColumnName, headerName)
		}

		seen[headerName] = true

		if schemaIdx, ok := nameToSchemaIdx[headerName]; ok {
			mapping[csvIdx] = schemaIdx
		}
	}

	return mapping, nil
}

func (i *importer) buildDefaultMapping() map[int]int {
	columns := i.schema.Columns()

	mapping := make(map[int]int)
	for idx := range columns {
		mapping[idx] = idx
	}

	return mapping
}

func (i *importer) parseRow(row []string, columnMapping map[int]int, csvRow int) (any, []tabular.ImportError) {
	result := reflect.New(i.typ).Elem()

	var errors []tabular.ImportError

	columns := i.schema.Columns()

	for csvIdx, schemaIdx := range columnMapping {
		col := columns[schemaIdx]

		var cellValue string
		if csvIdx < len(row) {
			cellValue = row[csvIdx]
			if i.options.trimSpace {
				cellValue = strings.TrimSpace(cellValue)
			}
		}

		if cellValue == "" && col.Default != "" {
			cellValue = col.Default
		}

		field := result.FieldByIndex(col.Index)
		if !field.CanSet() {
			errors = append(errors, tabular.ImportError{
				Row:    csvRow,
				Column: col.Name,
				Field:  field.Type().Name(),
				Err:    ErrFieldNotSettable,
			})

			continue
		}

		value, err := i.parseValue(cellValue, field.Type(), col)
		if err != nil {
			errors = append(errors, tabular.ImportError{
				Row:    csvRow,
				Column: col.Name,
				Field:  field.Type().Name(),
				Err:    fmt.Errorf("parse value: %w", err),
			})

			continue
		}

		field.Set(reflect.ValueOf(value))
	}

	return result.Interface(), errors
}

// parseValue falls back to default parser when custom parser is missing,
// preventing import failures due to configuration errors.
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

func (i *importer) isEmptyRow(row []string) bool {
	for _, cell := range row {
		value := cell
		if i.options.trimSpace {
			value = strings.TrimSpace(cell)
		}

		if value != "" {
			return false
		}
	}

	return true
}
