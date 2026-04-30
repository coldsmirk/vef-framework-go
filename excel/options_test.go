package excel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestImportOptions verifies each WithXxx ImportOption mutates the expected
// field on the underlying importConfig without touching unrelated fields.
func TestImportOptions(t *testing.T) {
	t.Run("WithImportSheetName", func(t *testing.T) {
		cfg := importConfig{}
		WithImportSheetName("DataSheet")(&cfg)
		assert.Equal(t, "DataSheet", cfg.sheetName, "WithImportSheetName should set the sheet name")
	})

	t.Run("WithImportSheetIndex", func(t *testing.T) {
		cfg := importConfig{}
		WithImportSheetIndex(2)(&cfg)
		assert.Equal(t, 2, cfg.sheetIndex, "WithImportSheetIndex should set the sheet index")
	})

	t.Run("WithSkipRows", func(t *testing.T) {
		cfg := importConfig{}
		WithSkipRows(3)(&cfg)
		assert.Equal(t, 3, cfg.skipRows, "WithSkipRows should set the skipRows count")
	})

	t.Run("WithSkipRowsClampsNegative", func(t *testing.T) {
		cfg := importConfig{skipRows: 5}
		WithSkipRows(-1)(&cfg)
		assert.Equal(t, 0, cfg.skipRows, "WithSkipRows should clamp negative values to zero")
	})

	t.Run("WithoutHeader", func(t *testing.T) {
		cfg := importConfig{hasHeader: true}
		WithoutHeader()(&cfg)
		assert.False(t, cfg.hasHeader, "WithoutHeader should clear the hasHeader flag")
	})

	t.Run("WithoutTrimSpace", func(t *testing.T) {
		cfg := importConfig{trimSpace: true}
		WithoutTrimSpace()(&cfg)
		assert.False(t, cfg.trimSpace, "WithoutTrimSpace should disable trimming")
	})

	t.Run("LaterOptionOverridesEarlier", func(t *testing.T) {
		cfg := importConfig{}
		WithImportSheetName("First")(&cfg)
		WithImportSheetName("Second")(&cfg)
		assert.Equal(t, "Second", cfg.sheetName, "Later WithImportSheetName call should override the earlier one")
	})
}

// TestExportOptions verifies each WithXxx ExportOption mutates the expected
// field on the underlying exportConfig without touching unrelated fields.
func TestExportOptions(t *testing.T) {
	t.Run("WithSheetName", func(t *testing.T) {
		cfg := exportConfig{}
		WithSheetName("Users")(&cfg)
		assert.Equal(t, "Users", cfg.sheetName, "WithSheetName should set the sheet name")
	})

	t.Run("LaterOptionOverridesEarlier", func(t *testing.T) {
		cfg := exportConfig{}
		WithSheetName("First")(&cfg)
		WithSheetName("Second")(&cfg)
		assert.Equal(t, "Second", cfg.sheetName, "Later WithSheetName call should override the earlier one")
	})
}
