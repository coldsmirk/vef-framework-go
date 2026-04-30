package tabular

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ParseRowOptions controls cell-level normalization performed by ParseRow.
type ParseRowOptions struct {
	// TrimSpace strips leading/trailing whitespace from each source cell
	// before applying defaults.
	TrimSpace bool
}

// ParseRow walks a single source row and writes parsed cells into the row
// builder. The mapping translates source column index to schema column index.
// Cells are read in sorted source-index order so that per-row import errors
// are deterministic.
//
// rowNumber is the human-facing row number (1-based, including the header
// row) used when constructing ImportError entries. Empty cells fall back to
// the column Default; cells that remain empty are skipped so that adapters
// (e.g. MapAdapter) can distinguish absent values from explicit zeroes and
// enforce Required.
func ParseRow(
	cells []string,
	mapping map[int]int,
	schema *Schema,
	builder RowBuilder,
	parsers map[string]ValueParser,
	rowNumber int,
	opts ParseRowOptions,
) []ImportError {
	var errs []ImportError

	columns := schema.Columns()

	for _, srcIndex := range slices.Sorted(maps.Keys(mapping)) {
		column := columns[mapping[srcIndex]]

		var cellValue string
		if srcIndex < len(cells) {
			cellValue = cells[srcIndex]
			if opts.TrimSpace {
				cellValue = strings.TrimSpace(cellValue)
			}
		}

		if cellValue == "" && column.Default != "" {
			cellValue = column.Default
		}

		if cellValue == "" {
			continue
		}

		value, err := ResolveParser(column, parsers).Parse(cellValue, column.Type)
		if err != nil {
			errs = append(errs, ImportError{
				Row:    rowNumber,
				Column: column.Name,
				Field:  column.Key,
				Err:    fmt.Errorf("parse value: %w", err),
			})

			continue
		}

		if err := builder.Set(column, value); err != nil {
			errs = append(errs, ImportError{
				Row:    rowNumber,
				Column: column.Name,
				Field:  column.Key,
				Err:    err,
			})

			continue
		}
	}

	return errs
}

// IsEmptyRow reports whether every cell in the row is empty. When trimSpace
// is true, surrounding whitespace is ignored.
func IsEmptyRow(cells []string, trimSpace bool) bool {
	for _, cell := range cells {
		if trimSpace {
			cell = strings.TrimSpace(cell)
		}

		if cell != "" {
			return false
		}
	}

	return true
}
