package tabular

import (
	"fmt"
	"strings"
)

// MappingOptions controls header-to-schema resolution.
type MappingOptions struct {
	// TrimSpace trims leading/trailing whitespace from each header cell.
	TrimSpace bool
}

// BuildHeaderMapping resolves a header row against the schema and returns a
// map from source column index to schema column index. Unknown headers are
// skipped; duplicate non-empty headers produce ErrDuplicateColumnName.
func BuildHeaderMapping(
	headerRow []string, schema *Schema, opts MappingOptions,
) (map[int]int, error) {
	columns := schema.Columns()
	mapping := make(map[int]int)

	nameToSchemaIdx := make(map[string]int, len(columns))
	for i, column := range columns {
		nameToSchemaIdx[column.Name] = i
	}

	seen := make(map[string]bool, len(headerRow))

	for srcIndex, headerName := range headerRow {
		if opts.TrimSpace {
			headerName = strings.TrimSpace(headerName)
		}

		if headerName == "" {
			continue
		}

		if seen[headerName] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateColumnName, headerName)
		}

		seen[headerName] = true

		if schemaIndex, ok := nameToSchemaIdx[headerName]; ok {
			mapping[srcIndex] = schemaIndex
		}
	}

	return mapping, nil
}

// DefaultPositionalMapping returns a 1:1 index mapping suitable when the source
// has no header row.
func DefaultPositionalMapping(schema *Schema) map[int]int {
	mapping := make(map[int]int, schema.ColumnCount())
	for i := range schema.ColumnCount() {
		mapping[i] = i
	}

	return mapping
}
