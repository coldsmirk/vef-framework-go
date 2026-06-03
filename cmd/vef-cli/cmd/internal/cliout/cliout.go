package cliout

import (
	"fmt"

	"github.com/muesli/termenv"
)

// PrintLabeledLine writes a colored label to output. When value is empty the
// label is printed on its own line; otherwise the colored label is followed by
// the uncolored value on the same line.
func PrintLabeledLine(output *termenv.Output, label, value string, color termenv.Color) {
	if value == "" {
		_, _ = fmt.Println(output.String(label).Foreground(color))

		return
	}

	_, _ = fmt.Print(output.String(label).Foreground(color))
	_, _ = fmt.Println(value)
}
