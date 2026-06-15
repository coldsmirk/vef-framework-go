package tabular

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newImportRowsSchema(t *testing.T) *Schema {
	t.Helper()

	schema, err := NewSchemaFromSpecs([]ColumnSpec{
		{Key: "id", Name: "ID", Type: reflect.TypeFor[int](), Required: true},
		{Key: "name", Name: "Name", Type: reflect.TypeFor[string]()},
	})
	require.NoError(t, err, "Building the ImportRows fixture schema should succeed")

	return schema
}

// TestImportRows covers the shared import core that the csv and excel importers
// delegate to: header vs positional mapping, leading-row skipping, 1-based row
// numbering, empty-row skipping, parse and validation error accumulation, and
// the fatal no-data-rows guard.
func TestImportRows(t *testing.T) {
	t.Run("HeaderMappingProducesRows", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		rows := [][]string{
			{"ID", "Name"},
			{"1", "Alice"},
			{"2", "Bob"},
		}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: true})
		require.NoError(t, err, "A well-formed header table should not raise a fatal error")
		assert.Empty(t, importErrors, "Valid rows should not accumulate per-row errors")

		records, ok := result.([]map[string]any)
		require.True(t, ok, "MapAdapter should build []map[string]any")
		require.Len(t, records, 2, "Both data rows should be imported")
		assert.Equal(t, 1, records[0]["id"], "First row id should parse to int")
		assert.Equal(t, "Alice", records[0]["name"], "First row name should map by header")
		assert.Equal(t, "Bob", records[1]["name"], "Second row name should map by header")
	})

	t.Run("PositionalMappingWithoutHeader", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		rows := [][]string{
			{"1", "Alice"},
			{"2", "Bob"},
		}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: false})
		require.NoError(t, err, "Positional mapping should not raise a fatal error")
		assert.Empty(t, importErrors, "Valid positional rows should not accumulate per-row errors")

		records := result.([]map[string]any)
		require.Len(t, records, 2, "Both rows should be imported positionally")
		assert.Equal(t, 1, records[0]["id"], "Column 0 should map to the first schema column")
		assert.Equal(t, "Alice", records[0]["name"], "Column 1 should map to the second schema column")
	})

	t.Run("SkipRowsDiscardsLeadingRows", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		rows := [][]string{
			{"junk title row"},
			{"ID", "Name"},
			{"1", "Alice"},
		}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{SkipRows: 1, HasHeader: true})
		require.NoError(t, err, "Skipping a leading row before the header should succeed")
		assert.Empty(t, importErrors, "The single valid data row should import cleanly")

		records := result.([]map[string]any)
		require.Len(t, records, 1, "Only the data row after the header should be imported")
		assert.Equal(t, "Alice", records[0]["name"], "The surviving row should be the data row")
	})

	t.Run("EmptyRowsAreSkipped", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		rows := [][]string{
			{"ID", "Name"},
			{"1", "Alice"},
			{"  ", ""},
			{"2", "Bob"},
		}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: true, TrimSpace: true})
		require.NoError(t, err, "A whitespace-only row should not be fatal")
		assert.Empty(t, importErrors, "A skipped empty row should not produce an error")

		records := result.([]map[string]any)
		require.Len(t, records, 2, "The blank row should be skipped, leaving two rows")
	})

	t.Run("ParseErrorCarriesOneBasedRowNumber", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		rows := [][]string{
			{"ID", "Name"},
			{"1", "Alice"},
			{"not-an-int", "Bob"},
		}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: true})
		require.NoError(t, err, "A bad cell should be a per-row error, not fatal")
		require.Len(t, importErrors, 1, "The unparseable id should produce exactly one row error")
		assert.Equal(t, 3, importErrors[0].Row,
			"Row number should be 1-based and include the header row (header=1, first data=2, bad=3)")
		assert.Equal(t, "ID", importErrors[0].Column, "The error should name the failing column")

		records := result.([]map[string]any)
		assert.Len(t, records, 1, "Only the valid row should be committed")
	})

	t.Run("ValidationErrorOnRequiredMiss", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		// Row 3 omits the required id (positional row with only a name cell).
		rows := [][]string{
			{"ID", "Name"},
			{"1", "Alice"},
			{"", "Bob"},
		}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: true})
		require.NoError(t, err, "A missing required value should be a per-row error, not fatal")
		require.Len(t, importErrors, 1, "The required-id miss should produce one row error")
		assert.ErrorIs(t, importErrors[0].Err, ErrRequiredMissing, "Commit failure should wrap ErrRequiredMissing")
		assert.Equal(t, 3, importErrors[0].Row, "The validation error should report the 1-based row number")

		records := result.([]map[string]any)
		assert.Len(t, records, 1, "Only the complete row should be committed")
	})

	t.Run("NoDataRowsIsFatal", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		// Header present but no data rows follow it.
		rows := [][]string{{"ID", "Name"}}

		result, importErrors, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: true})
		require.Error(t, err, "A header with no data rows should be fatal")
		assert.ErrorIs(t, err, ErrNoDataRowsFound, "The fatal error should wrap ErrNoDataRowsFound")
		assert.Nil(t, result, "No result is produced on a fatal error")
		assert.Nil(t, importErrors, "No per-row errors are produced on a fatal error")
	})

	t.Run("DuplicateHeaderIsFatal", func(t *testing.T) {
		adapter := NewMapAdapter(newImportRowsSchema(t))

		rows := [][]string{
			{"ID", "ID"},
			{"1", "2"},
		}

		_, _, err := ImportRows(rows, adapter, nil, ImportRowsOptions{HasHeader: true})
		require.Error(t, err, "A duplicate header should abort the whole import")
		assert.ErrorIs(t, err, ErrDuplicateHeaderName, "The fatal error should wrap ErrDuplicateHeaderName")
	})
}
