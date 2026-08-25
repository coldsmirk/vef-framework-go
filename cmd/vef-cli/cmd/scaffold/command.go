package scaffold

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/cliout"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
)

// ErrInvalidSearchSpec reports a malformed --search entry.
var ErrInvalidSearchSpec = errors.New("invalid --search entry, expected column:operator")

// Command returns the `new` command group: the scaffolding half of the CLI.
//
// The split from the generate-* commands is the contract with the developer.
// Everything under `new` is written once and then belongs to them — no
// generated-file header, no expectation that it stays machine-shaped — while
// the generate-* commands own their output forever and overwrite it on every
// run.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Scaffold a project, an API resource, or a service",
		Long: `Scaffold VEF Framework code.

Everything this command writes is yours from the moment it lands: it carries no
"do not edit" marker and is never regenerated. Re-running a generator over
existing code reports what already exists instead of overwriting it, unless
--force says otherwise.`,
	}

	cmd.AddCommand(resourceCommand(), serviceCommand(), projectCommand())

	return cmd
}

func resourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource",
		Short: "Generate a model, payload and API resource from a database table",
		Long: `Generate a CRUD API resource from an existing database table.

The table is inspected through the framework's own schema service, so column
comments become label tags, nullability becomes pointers and omitempty, a
declared character bound becomes a max= validation rule, and a table carrying
the framework's audit columns embeds the matching orm mixin instead of
re-declaring them.

Three files are written — model, payload and resource — and two registries are
updated: the model package's variable block and the module's fx option list.
Both registrations are part of the job because forgetting either produces an
application that compiles and answers 404.

Conventions come from vef.yml in the project root; see that file for the
resource-name and permission templates, the default operation set, and the
audit user model.

Example:
  vef-cli new resource --table md_allowance_item --module md
  vef-cli new resource --table sys_user --module sys --ops find_page,create --dry-run`,
		RunE: runResource,
	}

	cmd.Flags().StringP("table", "t", "", "Database table to derive the entity from (required)")
	cmd.Flags().StringP("module", "m", "", "Business module to generate into, may be nested (required)")
	cmd.Flags().String("entity", "", "Entity name override, snake_case (default: the table name without the module prefix)")
	cmd.Flags().String("alias", "", "Table alias override (default: the initials of the table's words)")
	cmd.Flags().StringSlice("ops", nil, "CRUD operations to embed (default: the project's configured set)")
	cmd.Flags().StringSlice("search", nil, "Search criteria as column:operator; 'none' generates an empty search payload")
	cmd.Flags().String("source", "", "Data source to inspect (default: primary)")
	cmd.Flags().String("config", "", "Path to application.toml (default: <project>/configs/application.toml)")
	cmd.Flags().Bool("force", false, "Overwrite generated files that already exist")
	cmd.Flags().BoolP("dry-run", "n", false, "Print what would be written without touching the filesystem")

	_ = cmd.MarkFlagRequired("table")
	_ = cmd.MarkFlagRequired("module")

	return cmd
}

func runResource(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	proj, out, err := openProject()
	if err != nil {
		return err
	}

	table, _ := cmd.Flags().GetString("table")
	rawModule, _ := cmd.Flags().GetString("module")
	ops, _ := cmd.Flags().GetStringSlice("ops")
	rawSearch, _ := cmd.Flags().GetStringSlice("search")

	module, err := project.CleanModule(rawModule)
	if err != nil {
		return err
	}

	search, err := parseSearchSpec(rawSearch)
	if err != nil {
		return err
	}

	req := ResourceRequest{Table: table, Module: module, Ops: ops, Search: search}
	req.Entity, _ = cmd.Flags().GetString("entity")
	req.Alias, _ = cmd.Flags().GetString("alias")
	req.Source, _ = cmd.Flags().GetString("source")
	req.ConfigPath, _ = cmd.Flags().GetString("config")
	req.Force, _ = cmd.Flags().GetBool("force")

	cliout.PrintLabeledLine(os.Stdout, out, "Generating resource from table ", table, termenv.ANSICyan)

	plan := NewPlan(proj)
	if err := GenerateResource(cmd.Context(), proj, req, plan); err != nil {
		return err
	}

	return finish(cmd, plan, out)
}

func serviceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Generate a service and register it with its module",
		Long: `Generate a business service skeleton.

The service follows the framework's convention: a struct holding only injected
dependencies, a constructor returning it, and methods that take the request's
context and orm.DB rather than holding a connection. It is registered with the
module's fx option list.

Example:
  vef-cli new service --name Notification --module sys
  vef-cli new service --name Roster --module sch --deps "bus:event.Bus,notifier:push.Notifier"`,
		RunE: runService,
	}

	cmd.Flags().StringP("name", "n", "", "Service name in PascalCase, with or without the Service suffix (required)")
	cmd.Flags().StringP("module", "m", "", "Business module to generate into, may be nested (required)")
	cmd.Flags().StringSlice("deps", nil, "Injected dependencies as field:Type, for example bus:event.Bus")
	cmd.Flags().Bool("force", false, "Overwrite the service file if it already exists")
	cmd.Flags().Bool("dry-run", false, "Print what would be written without touching the filesystem")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("module")

	return cmd
}

func runService(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	proj, out, err := openProject()
	if err != nil {
		return err
	}

	name, _ := cmd.Flags().GetString("name")
	rawModule, _ := cmd.Flags().GetString("module")
	rawDeps, _ := cmd.Flags().GetStringSlice("deps")

	module, err := project.CleanModule(rawModule)
	if err != nil {
		return err
	}

	deps, err := parseDependencies(rawDeps)
	if err != nil {
		return err
	}

	req := ServiceRequest{Name: name, Module: module, Deps: deps}
	req.Force, _ = cmd.Flags().GetBool("force")

	cliout.PrintLabeledLine(os.Stdout, out, "Generating service ", name, termenv.ANSICyan)

	plan := NewPlan(proj)
	if err := GenerateService(proj, req, plan); err != nil {
		return err
	}

	return finish(cmd, plan, out)
}

// openProject resolves the project the command runs in and the terminal output
// it reports through.
func openProject() (*project.Project, *termenv.Output, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve working directory: %w", err)
	}

	proj, err := project.Locate(cwd)
	if err != nil {
		return nil, nil, err
	}

	return proj, termenv.DefaultOutput(), nil
}

// finish previews or applies the plan and reports the outcome.
func finish(cmd *cobra.Command, plan *Plan, out *termenv.Output) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if dryRun {
		plan.Preview(os.Stdout, out)
		cliout.PrintLabeledLine(os.Stdout, out, "Dry run: nothing was written", "", termenv.ANSIYellow)

		return nil
	}

	if err := plan.Apply(os.Stdout, out); err != nil {
		return err
	}

	if plan.Empty() {
		cliout.PrintLabeledLine(os.Stdout, out, "Nothing to do: everything is already in place", "", termenv.ANSIBrightBlack)

		return nil
	}

	cliout.PrintLabeledLine(os.Stdout, out, "Done", "", termenv.ANSIGreen)

	return nil
}

// parseSearchSpec turns --search entries into the column-to-operator map the
// entity builder takes. The literal "none" selects an empty search payload,
// which a wide table wants; nil (the flag unset) leaves the default in place.
func parseSearchSpec(entries []string) (map[string]searchOperator, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	if len(entries) == 1 && strings.EqualFold(entries[0], "none") {
		return map[string]searchOperator{}, nil
	}

	spec := make(map[string]searchOperator, len(entries))

	for _, entry := range entries {
		column, operator, found := strings.Cut(entry, ":")
		if !found {
			return nil, fmt.Errorf("%w: %s", ErrInvalidSearchSpec, entry)
		}

		switch searchOperator(operator) {
		case searchContains, searchEquals:
			spec[strings.TrimSpace(column)] = searchOperator(operator)
		default:
			return nil, fmt.Errorf("%w: %s (operator must be %s or %s)", ErrInvalidSearchSpec, entry, searchContains, searchEquals)
		}
	}

	return spec, nil
}
