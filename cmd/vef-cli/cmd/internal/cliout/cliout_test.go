package cliout

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func TestPrintLabeledLine(t *testing.T) {
	output := termenv.DefaultOutput()

	t.Run("WritesTheLabelAndValueToTheGivenWriter", func(t *testing.T) {
		var buf bytes.Buffer
		PrintLabeledLine(&buf, output, "Wrote ", "api-manifest.json", termenv.ANSIGreen)

		assert.Contains(t, buf.String(), "Wrote ", "the label must be written")
		assert.Contains(t, buf.String(), "api-manifest.json", "the value must follow the label")
		assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("\n")), "label and value share one line")
	})

	t.Run("AnEmptyValueLeavesTheLabelOnItsOwnLine", func(t *testing.T) {
		var buf bytes.Buffer
		PrintLabeledLine(&buf, output, "Done", "", termenv.ANSIGreen)

		assert.Contains(t, buf.String(), "Done", "the label must be written")
		assert.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("\n")), "a lone label still terminates its line")
	})

	// The destination is a parameter precisely so a command whose stdout carries
	// data can keep its progress off it; writing anywhere else would put the
	// human-readable line back in front of `export-api -o -`'s manifest.
	t.Run("NothingLeaksPastTheWriter", func(t *testing.T) {
		var first, second bytes.Buffer

		PrintLabeledLine(&first, output, "Exporting from ", "./cmd/server", termenv.ANSICyan)
		PrintLabeledLine(&second, output, "Done", "", termenv.ANSIGreen)

		assert.NotContains(t, first.String(), "Done", "each call must write only to the writer it was given")
		assert.NotContains(t, second.String(), "Exporting", "each call must write only to the writer it was given")
	})
}
