package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/buildinfo"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/create"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/modelschema"
)

var version VersionInfo

var rootCmd = &cobra.Command{
	Use:   "vef-cli",
	Short: "VEF Framework CLI tool",
	Long:  `A command-line tool for VEF Framework to help with code generation and project setup.`,
}

// Init initializes version information from ldflags or runtime/debug.
func Init(ldflagsVersion, ldflagsDate string) {
	version = GetVersionInfo(ldflagsVersion, ldflagsDate)
}

// Execute runs the root command.
func Execute() error {
	rootCmd.Version = version.Version
	rootCmd.SetVersionTemplate(Banner + "\n" + version.String() + "\n")

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
