package tabular

import "fmt"

// ImportRowsOptions controls how ImportRows interprets a materialized table.
type ImportRowsOptions struct {
	// SkipRows discards this many leading rows before the header or first data
	// row is read.
	SkipRows int
	// HasHeader treats the first non-skipped row as a header that maps source
	// columns to schema columns by name. When false, columns map positionally.
	HasHeader bool
	// TrimSpace strips surrounding whitespace during header matching, empty-row
	// detection, and cell parsing.
	TrimSpace bool
}

// ImportRows parses a fully materialized table into the adapter's row model.
// It is the shared core behind the csv and excel importers: each reads its
// source into [][]string and delegates here so the header resolution, row
// numbering, empty-row skipping, parsing, and per-row validation live in one
// place.
//
// parsers is the importer's named-parser registry (referenced by Column.Parser);
// ImportRows resolves it against the schema once before the row loop. The
// returned value is the adapter writer's aggregated result (e.g. []T); the
// []ImportError slice carries per-row failures while error is reserved for
// fatal conditions that abort the whole import (no data rows or an unresolvable
// header).
func ImportRows(
	rows [][]string,
	adapter RowAdapter,
	parsers map[string]ValueParser,
	opts ImportRowsOptions,
) (any, []ImportError, error) {
	minRows := opts.SkipRows
	if opts.HasHeader {
		minRows++
	}

	if len(rows) <= minRows {
		return nil, nil, fmt.Errorf("%w (total rows: %d, skip rows: %d, has header: %v)",
			ErrNoDataRowsFound, len(rows), opts.SkipRows, opts.HasHeader)
	}

	schema := adapter.Schema()
	dataStartIndex := opts.SkipRows

	var columnMapping ColumnMapping

	if opts.HasHeader {
		rawMapping, err := BuildHeaderMapping(rows[opts.SkipRows], schema, MappingOptions{TrimSpace: opts.TrimSpace})
		if err != nil {
			return nil, nil, fmt.Errorf("build column mapping: %w", err)
		}

		columnMapping = NewColumnMapping(rawMapping)
		dataStartIndex++
	} else {
		columnMapping = NewColumnMapping(DefaultPositionalMapping(schema))
	}

	dataRows := rows[dataStartIndex:]
	writer := adapter.Writer(len(dataRows))

	parseOpts := ParseRowOptions{TrimSpace: opts.TrimSpace}
	resolvedParsers := ResolveParsers(schema, parsers)

	var importErrors []ImportError

	for rowIndex, row := range dataRows {
		// 1-based row number that accounts for skipped rows and the header row,
		// matching what a user sees in a spreadsheet or text editor.
		rowNumber := dataStartIndex + rowIndex + 1

		if IsEmptyRow(row, opts.TrimSpace) {
			continue
		}

		builder := writer.NewRow()

		rowErrors := ParseRow(row, columnMapping, schema, builder, resolvedParsers, rowNumber, parseOpts)
		if len(rowErrors) > 0 {
			importErrors = append(importErrors, rowErrors...)

			continue
		}

		if err := writer.Commit(builder); err != nil {
			importErrors = append(importErrors, ImportError{
				Row: rowNumber,
				Err: fmt.Errorf("validation failed: %w", err),
			})

			continue
		}
	}

	return writer.Build(), importErrors, nil
}
