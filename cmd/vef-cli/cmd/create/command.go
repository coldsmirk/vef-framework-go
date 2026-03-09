package create

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var ErrNotImplemented = errors.New("vef-cli create is not implemented yet, please generate the project manually")

// Command returns the create cobra command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new VEF Framework project",
		Long: `Create a new VEF Framework project with the standard structure and configuration files.

This command will set up a complete project skeleton including:
  - Application configuration
  - Directory structure
  - Sample code and resources
  - Dependencies (go.mod)`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, _ := cmd.Flags().GetString("name")
			path, _ := cmd.Flags().GetString("path")
			module, _ := cmd.Flags().GetString("module")

			_, _ = fmt.Println("Creating new VEF project...")
			_, _ = fmt.Printf("  Project name: %s\n", name)
			_, _ = fmt.Printf("  Path: %s\n", path)
			_, _ = fmt.Printf("  Module: %s\n", module)

			return ErrNotImplemented
		},
	}

	cmd.Flags().StringP("name", "n", "", "Project name (required)")
	cmd.Flags().StringP("path", "p", ".", "Directory path to create the project")
	cmd.Flags().StringP("module", "m", "", "Go module name (e.g., github.com/user/project)")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}
