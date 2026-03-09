package cmd

import (
	"fmt"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

func setupHelpColors(cmd *cobra.Command) {
	output := termenv.DefaultOutput()

	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		if cmd.Parent() == nil {
			PrintBanner()
		}

		printDescription(cmd)
		printUsage(cmd, output)
		printAvailableCommands(cmd, output)
		printFlags(cmd, output)
		printHelpHint(cmd)

		return nil
	})

	cmd.SetHelpFunc(func(*cobra.Command, []string) {
		_ = cmd.Usage()
	})
}

func printDescription(cmd *cobra.Command) {
	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}

	if desc != "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), desc)
		_, _ = fmt.Fprintln(cmd.OutOrStdout())
	}
}

func printUsage(cmd *cobra.Command, output *termenv.Output) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), output.String("Usage:").Foreground(termenv.ANSICyan).Bold())

	if cmd.Runnable() {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s [flags]\n", cmd.CommandPath())
	}

	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s [command]\n", cmd.CommandPath())
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())
}

func printAvailableCommands(cmd *cobra.Command, output *termenv.Output) {
	if !cmd.HasAvailableSubCommands() {
		return
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), output.String("Available Commands:").Foreground(termenv.ANSICyan).Bold())

	maxLen := 0
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		if len(c.Name()) > maxLen {
			maxLen = len(c.Name())
		}
	}

	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		_, _ = fmt.Fprint(cmd.OutOrStdout(), output.String(fmt.Sprintf("  %s", c.Name())).Foreground(termenv.ANSIGreen))
		spacing := maxLen - len(c.Name()) + 2
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%*s", spacing, " ")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), c.Short)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())
}

func printFlags(cmd *cobra.Command, output *termenv.Output) {
	if !cmd.HasAvailableLocalFlags() && !cmd.HasAvailablePersistentFlags() {
		return
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), output.String("Flags:").Foreground(termenv.ANSICyan).Bold())

	if cmd.HasAvailableLocalFlags() {
		printFlagUsages(cmd, cmd.LocalFlags(), output)
	}

	if cmd.HasAvailableInheritedFlags() {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), output.String("\nGlobal Flags:").Foreground(termenv.ANSICyan).Bold())
		printFlagUsages(cmd, cmd.InheritedFlags(), output)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())
}

func printHelpHint(cmd *cobra.Command) {
	if cmd.HasAvailableSubCommands() {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Use \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	}
}

func printFlagUsages(cmd *cobra.Command, flags any, output *termenv.Output) {
	type FlagSet interface {
		FlagUsages() string
	}

	fs, ok := flags.(FlagSet)
	if !ok {
		return
	}

	usages := fs.FlagUsages()
	lines := strings.SplitSeq(usages, "\n")

	for line := range lines {
		if line == "" {
			continue
		}

		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			continue
		}

		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)

			continue
		}

		flagPart := parts[0]
		descPart := strings.TrimLeft(parts[1], " ")

		_, _ = fmt.Fprint(cmd.OutOrStdout(), output.String(flagPart).Foreground(termenv.ANSIGreen))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", descPart)
	}
}
