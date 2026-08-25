package scaffold

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"golang.org/x/mod/module"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/buildinfo"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/cliout"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/gopatch"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
	"github.com/coldsmirk/vef-framework-go/version"
)

var (
	// ErrProjectNameRequired reports a missing project name.
	ErrProjectNameRequired = errors.New("project name is required")
	// ErrDirectoryNotEmpty reports that the target directory already holds files.
	ErrDirectoryNotEmpty = errors.New("target directory is not empty")
	// ErrInvalidModulePath reports a go.mod module path the Go tool would reject.
	ErrInvalidModulePath = errors.New("invalid module path")
)

// exampleModule is the module a fresh project starts with, so `new resource`
// has somewhere to generate into and the boot sequence has something to serve.
const exampleModule = "example"

// projectView is what the project templates render from.
type projectView struct {
	// Name is the application name, used for the binary and vef.app.name.
	Name string
	// Module is the go.mod module path.
	Module string
	// FrameworkVersion pins the framework in go.mod. It is the version of the
	// CLI doing the scaffolding, which is the only definition that cannot go
	// stale: the tool generates against the API it was built from.
	FrameworkVersion string
	// GoVersion is the language version go.mod declares.
	GoVersion string
	// ExampleModule is the starter business module's name.
	ExampleModule string
	// WithExample reports whether the starter module is generated.
	WithExample bool
}

// ProjectRequest is one `new project` invocation.
type ProjectRequest struct {
	// Name is the application name.
	Name string
	// Module is the go.mod module path, defaulting to Name.
	Module string
	// Dir is the directory to create, defaulting to Name under the working
	// directory.
	Dir string
	// WithExample generates the starter business module.
	WithExample bool
	// SkipTidy leaves dependency resolution to the developer.
	SkipTidy bool
	// SkipGit leaves repository initialization to the developer.
	SkipGit bool
}

func projectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <name>",
		Short: "Scaffold a new VEF Framework project",
		Long: `Scaffold a VEF Framework application.

The layout is the framework's own convention: an entry point under cmd/server,
runtime configuration under configs, business modules under internal, and a
vef.yml recording the code-generation conventions the other new commands read.

Deliberately excluded are Dockerfiles, git hooks and CI pipelines: those encode
deployment and team decisions the framework has no opinion about. What is
included is what the framework itself decides — the lint configuration and the
task shortcuts.

Example:
  vef-cli new project acme-server
  vef-cli new project acme-server --module git.example.com/team/acme-server`,
		Args: cobra.MaximumNArgs(1),
		RunE: runProject,
	}

	cmd.Flags().StringP("name", "N", "", "Project name (defaults to the positional argument)")
	cmd.Flags().StringP("module", "m", "", "Go module path (default: the project name)")
	cmd.Flags().StringP("path", "p", "", "Directory to create the project in (default: ./<name>)")
	cmd.Flags().Bool("with-example", true, "Generate a starter business module")
	cmd.Flags().Bool("skip-tidy", false, "Skip running go mod tidy")
	cmd.Flags().Bool("skip-git", false, "Skip running git init")
	cmd.Flags().Bool("dry-run", false, "Print what would be written without touching the filesystem")

	return cmd
}

func runProject(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	req, err := projectRequest(cmd, args)
	if err != nil {
		return err
	}

	out := termenv.DefaultOutput()
	cliout.PrintLabeledLine(os.Stdout, out, "Creating project ", req.Name, termenv.ANSICyan)
	cliout.PrintLabeledLine(os.Stdout, out, "  Module: ", req.Module, termenv.ANSIBrightBlack)
	cliout.PrintLabeledLine(os.Stdout, out, "  Path: ", req.Dir, termenv.ANSIBrightBlack)

	// The plan reports paths relative to the project it targets, which for a
	// project that does not exist yet is the directory about to be created.
	plan := NewPlan(&project.Project{Root: req.Dir, ModulePath: req.Module, Config: project.DefaultConfig()})

	if err := GenerateProject(req, plan); err != nil {
		return err
	}

	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		plan.Preview(os.Stdout, out)
		cliout.PrintLabeledLine(os.Stdout, out, "Dry run: nothing was written", "", termenv.ANSIYellow)

		return nil
	}

	if err := plan.Apply(os.Stdout, out); err != nil {
		return err
	}

	return finalizeProject(cmd.Context(), req, out)
}

// projectRequest resolves the command's flags and positional argument, and
// refuses a target directory that already holds files.
func projectRequest(cmd *cobra.Command, args []string) (ProjectRequest, error) {
	name, _ := cmd.Flags().GetString("name")
	if name == "" && len(args) > 0 {
		name = args[0]
	}

	if strings.TrimSpace(name) == "" {
		return ProjectRequest{}, ErrProjectNameRequired
	}

	req := ProjectRequest{Name: name}
	req.Module, _ = cmd.Flags().GetString("module")
	req.Dir, _ = cmd.Flags().GetString("path")
	req.WithExample, _ = cmd.Flags().GetBool("with-example")
	req.SkipTidy, _ = cmd.Flags().GetBool("skip-tidy")
	req.SkipGit, _ = cmd.Flags().GetBool("skip-git")

	if req.Module == "" {
		req.Module = name
	}

	// The module path is the one generated value the whole project is built on
	// and the one nothing downstream checks: go.mod is rendered from a template,
	// and the only step that would notice is `go mod tidy`, whose failure is
	// deliberately reported as skippable. Without this a `new project "my app"`
	// printed "Done. Next:" over a directory that cannot compile.
	//
	// CheckImportPath, not CheckPath: the latter also demands a dot in the first
	// element because it judges paths that have to resolve to a repository, and
	// a project's module defaults to its own name — so it would reject the most
	// ordinary invocation there is, `new project acme-server`.
	if err := module.CheckImportPath(req.Module); err != nil {
		return ProjectRequest{}, fmt.Errorf("%w: %w", ErrInvalidModulePath, err)
	}

	if req.Dir == "" {
		req.Dir = name
	}

	absolute, err := filepath.Abs(req.Dir)
	if err != nil {
		return ProjectRequest{}, fmt.Errorf("resolve %s: %w", req.Dir, err)
	}

	req.Dir = absolute

	entries, err := os.ReadDir(req.Dir)
	if err == nil && len(entries) > 0 {
		return ProjectRequest{}, fmt.Errorf("%w: %s", ErrDirectoryNotEmpty, req.Dir)
	}

	return req, nil
}

// GenerateProject plans every file a fresh project starts with.
func GenerateProject(req ProjectRequest, plan *Plan) error {
	view := projectView{
		Name:             req.Name,
		Module:           req.Module,
		FrameworkVersion: version.VEFVersion,
		GoVersion:        goLanguageVersion(),
		ExampleModule:    exampleModule,
		WithExample:      req.WithExample,
	}

	groups := gopatch.ImportGroups{Framework: frameworkModule, Local: req.Module}

	goFiles := []scaffoldFile{
		{filepath.Join("cmd", "server", "main.go"), "project_main.go.tmpl", projectMainImports(req)},
		{filepath.Join("internal", "vef", "module.go"), "project_vef_module.go.tmpl", plainImports([]string{frameworkModule})},
	}

	if req.WithExample {
		examplePath := filepath.Join("internal", exampleModule)
		goFiles = append(goFiles,
			scaffoldFile{
				filepath.Join(examplePath, "module.go"), "project_example_module.go.tmpl",
				plainImports([]string{frameworkModule}),
			},
			scaffoldFile{filepath.Join(examplePath, "model", "models.go"), "project_example_models.go.tmpl", nil},
		)
	}

	for _, file := range goFiles {
		source, err := render(file.template, view, file.imports, groups)
		if err != nil {
			return err
		}

		plan.AddFile(filepath.Join(req.Dir, file.path), source, false)
	}

	textFiles := map[string]string{
		"go.mod": "project_go.mod.tmpl",
		filepath.Join("configs", "application.toml"): "project_application.toml.tmpl",
		project.ConfigFileName:                       "project_vef.yml.tmpl",
		".gitignore":                                 "project_gitignore.tmpl",
		".golangci.yml":                              "project_golangci.yml.tmpl",
		"Taskfile.yml":                               "project_taskfile.yml.tmpl",
	}

	for _, path := range sortedKeys(textFiles) {
		var buf bytes.Buffer
		if err := templates.ExecuteTemplate(&buf, textFiles[path], view); err != nil {
			return fmt.Errorf("render %s: %w", textFiles[path], err)
		}

		plan.AddFile(filepath.Join(req.Dir, path), buf.Bytes(), false)
	}

	return nil
}

// scaffoldFile is one generated Go source file of a new project.
type scaffoldFile struct {
	path     string
	template string
	imports  []gopatch.Import
}

func sortedKeys(files map[string]string) []string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	return keys
}

// projectMainImports lists what the generated entry point imports.
//
// The application's own wiring package is called vef too — that is where the
// build-info generator writes — so it is aliased, exactly as a developer would
// have to alias it the moment they added a second module.
func projectMainImports(req ProjectRequest) []gopatch.Import {
	imports := []gopatch.Import{
		{Path: frameworkModule},
		{Alias: "ivef", Path: req.Module + "/internal/vef"},
	}

	if req.WithExample {
		imports = append(imports, gopatch.Import{Path: req.Module + "/internal/" + exampleModule})
	}

	return imports
}

// finalizeProject runs the optional post-creation steps. Each failure is
// reported rather than fatal: the project on disk is complete and usable, and
// a missing toolchain should not read as a failed scaffold.
func finalizeProject(ctx context.Context, req ProjectRequest, out *termenv.Output) error {
	// The wiring module reports build metadata, and that metadata is generated
	// rather than committed — so it has to exist before the project compiles for
	// the first time. Generating it here rather than telling the developer to
	// run go generate is the difference between a scaffold that builds and one
	// that greets them with an undefined identifier.
	buildInfo := filepath.Join(req.Dir, "internal", "vef", "build_info.go")
	if err := buildinfo.Generate(buildInfo, "vef"); err != nil {
		return fmt.Errorf("generate build metadata: %w", err)
	}

	cliout.PrintLabeledLine(os.Stdout, out, "  create internal/vef/build_info.go", "", termenv.ANSIGreen)

	if !req.SkipGit {
		runOptional(ctx, out, req.Dir, "git", "init", "--quiet")
	}

	if !req.SkipTidy {
		runOptional(ctx, out, req.Dir, "go", "mod", "tidy")
	}

	cliout.PrintLabeledLine(os.Stdout, out, "Done. Next:", "", termenv.ANSIGreen)
	cliout.PrintLabeledLine(os.Stdout, out, "  1. ", "point vef.data_sources.primary in configs/application.toml at your database", termenv.ANSIBrightBlack)
	cliout.PrintLabeledLine(os.Stdout, out, "  2. ", "vef-cli new resource --table <table> --module "+exampleModule, termenv.ANSIBrightBlack)
	cliout.PrintLabeledLine(os.Stdout, out, "  3. ", "go run ./cmd/server", termenv.ANSIBrightBlack)

	return nil
}

// runOptional executes a post-creation step, reporting a failure without
// aborting.
func runOptional(ctx context.Context, out *termenv.Output, dir, name string, args ...string) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	if output, err := cmd.CombinedOutput(); err != nil {
		cliout.PrintLabeledLine(os.Stdout, out, "  skipped "+name+" "+strings.Join(args, " ")+": ",
			strings.TrimSpace(string(output)), termenv.ANSIYellow)

		return
	}

	cliout.PrintLabeledLine(os.Stdout, out, "  ran "+name+" "+strings.Join(args, " "), "", termenv.ANSIBrightBlack)
}

// goLanguageVersion is the language version a generated go.mod declares.
//
// It comes from the toolchain that built the CLI, truncated to major.minor.
// That is always at least what the framework itself requires, since the CLI was
// compiled from the framework's own source, and a developer on an older
// toolchain is not blocked: the default GOTOOLCHAIN=auto fetches a matching one.
func goLanguageVersion() string {
	declared := strings.TrimPrefix(strings.Fields(runtime.Version())[0], "go")

	parts := strings.Split(declared, ".")
	if len(parts) < 2 {
		return declared
	}

	return strings.Join(parts[:2], ".")
}
