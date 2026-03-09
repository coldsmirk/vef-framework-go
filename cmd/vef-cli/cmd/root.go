package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/buildinfo"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/create"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/modelschema"
)

var (
	Version string
	Date    string
)

var rootCmd = &cobra.Command{
	Use:   "vef-cli",
	Short: "VEF Framework CLI tool",
	Long:  `A command-line tool for VEF Framework to help with code generation and project setup.`,
}

// Init initializes version information from ldflags or runtime/debug.
func Init(ldflagsVersion, ldflagsDate string) {
	info := GetVersionInfo(ldflagsVersion, ldflagsDate)
	Version = info.Version
	Date = info.Date
}

// Execute runs the root command.
func Execute() error {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(Banner + fmt.Sprintf("\nVersion: %s | Built: %s\n", Version, Date))

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		return err
	}

	return nil
}

func init() {
	subCommands := []*cobra.Command{
		create.Command(),
		buildinfo.Command(),
		modelschema.Command(),
	}

	setupHelpColors(rootCmd)

	for _, cmd := range subCommands {
		setupHelpColors(cmd)
		rootCmd.AddCommand(cmd)
	}
}
