package cmd

import (
	"fmt"

	"github.com/muesli/termenv"
)

// Banner is the VEF CLI ASCII art logo.
const Banner = `
██╗   ██╗███████╗███████╗     ██████╗██╗     ██╗
██║   ██║██╔════╝██╔════╝    ██╔════╝██║     ██║
██║   ██║█████╗  █████╗      ██║     ██║     ██║
╚██╗ ██╔╝██╔══╝  ██╔══╝      ██║     ██║     ██║
 ╚████╔╝ ███████╗██║         ╚██████╗███████╗██║
  ╚═══╝  ╚══════╝╚═╝          ╚═════╝╚══════╝╚═╝
`

// PrintBanner prints the banner and version info.
func PrintBanner() {
	output := termenv.DefaultOutput()

	_, _ = fmt.Print(output.String(Banner).Foreground(termenv.ANSICyan).Bold())

	versionInfo := version.String() + "\n\n"
	_, _ = fmt.Print(output.String(versionInfo).Foreground(termenv.ANSIBrightBlack))
}
