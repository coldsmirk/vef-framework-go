package cliout

import (
	"fmt"
	"io"

	"github.com/muesli/termenv"
)

// PrintLabeledLine writes a colored label to w. When value is empty the label
// is printed on its own line; otherwise the colored label is followed by the
// uncolored value on the same line.
//
// The destination is a parameter rather than os.Stdout because a command whose
// real output goes to stdout has to keep its progress off it. export-api -o -
// is that command: with the label hardcoded to stdout, the manifest arrived
// with a human-readable line in front of it and every pipe into jq failed.
func PrintLabeledLine(w io.Writer, output *termenv.Output, label, value string, color termenv.Color) {
	if value == "" {
		_, _ = fmt.Fprintln(w, output.String(label).Foreground(color))

		return
	}

	_, _ = fmt.Fprint(w, output.String(label).Foreground(color))
	_, _ = fmt.Fprintln(w, value)
}
