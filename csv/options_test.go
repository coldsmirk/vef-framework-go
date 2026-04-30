package csv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestImportOptions verifies each WithXxx ImportOption mutates the expected
// field on the underlying importConfig without touching unrelated fields.
func TestImportOptions(t *testing.T) {
	t.Run("WithImportDelimiter", func(t *testing.T) {
		cfg := importConfig{}
		WithImportDelimiter(';')(&cfg)
		assert.Equal(t, ';', cfg.delimiter, "WithImportDelimiter should set the delimiter rune")
	})

	t.Run("WithoutHeader", func(t *testing.T) {
		cfg := importConfig{hasHeader: true}
		WithoutHeader()(&cfg)
		assert.False(t, cfg.hasHeader, "WithoutHeader should clear the hasHeader flag")
	})

	t.Run("WithSkipRows", func(t *testing.T) {
		cfg := importConfig{}
		WithSkipRows(3)(&cfg)
		assert.Equal(t, 3, cfg.skipRows, "WithSkipRows should set the skipRows count")
	})

	t.Run("WithSkipRowsAcceptsNegative", func(t *testing.T) {
		cfg := importConfig{skipRows: 5}
		WithSkipRows(-1)(&cfg)
		assert.Equal(t, -1, cfg.skipRows, "WithSkipRows should pass negative values through verbatim")
	})

	t.Run("WithoutTrimSpace", func(t *testing.T) {
		cfg := importConfig{trimSpace: true}
		WithoutTrimSpace()(&cfg)
		assert.False(t, cfg.trimSpace, "WithoutTrimSpace should disable trimming")
	})

	t.Run("WithComment", func(t *testing.T) {
		cfg := importConfig{}
		WithComment('#')(&cfg)
		assert.Equal(t, '#', cfg.comment, "WithComment should set the comment rune")
	})

	t.Run("LaterOptionOverridesEarlier", func(t *testing.T) {
		cfg := importConfig{}
		WithImportDelimiter(';')(&cfg)
		WithImportDelimiter('|')(&cfg)
		assert.Equal(t, '|', cfg.delimiter, "Later WithImportDelimiter call should override the earlier one")
	})
}

// TestExportOptions verifies each WithXxx ExportOption mutates the expected
// field on the underlying exportConfig without touching unrelated fields.
func TestExportOptions(t *testing.T) {
	t.Run("WithExportDelimiter", func(t *testing.T) {
		cfg := exportConfig{}
		WithExportDelimiter('\t')(&cfg)
		assert.Equal(t, '\t', cfg.delimiter, "WithExportDelimiter should set the delimiter rune")
	})

	t.Run("WithoutWriteHeader", func(t *testing.T) {
		cfg := exportConfig{writeHeader: true}
		WithoutWriteHeader()(&cfg)
		assert.False(t, cfg.writeHeader, "WithoutWriteHeader should clear the writeHeader flag")
	})

	t.Run("WithCrlf", func(t *testing.T) {
		cfg := exportConfig{}
		WithCrlf()(&cfg)
		assert.True(t, cfg.useCRLF, "WithCrlf should enable Windows-style line endings")
	})

	t.Run("LaterOptionOverridesEarlier", func(t *testing.T) {
		cfg := exportConfig{}
		WithExportDelimiter(';')(&cfg)
		WithExportDelimiter('|')(&cfg)
		assert.Equal(t, '|', cfg.delimiter, "Later WithExportDelimiter call should override the earlier one")
	})
}
