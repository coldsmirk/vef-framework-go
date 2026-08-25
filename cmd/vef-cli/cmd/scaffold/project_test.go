package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requestFor runs projectCommand's flag parsing over args and resolves the
// request, which is where a name that cannot become a project is refused.
func requestFor(t *testing.T, args ...string) (ProjectRequest, error) {
	t.Helper()

	cmd := projectCommand()
	cmd.RunE = func(*cobra.Command, []string) error { return nil }
	cmd.SetArgs(args)
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))

	require.NoError(t, cmd.Execute(), "parsing the flags must succeed")

	return projectRequest(cmd, cmd.Flags().Args())
}

func TestProjectRequest(t *testing.T) {
	t.Run("DefaultsTheModuleAndDirectoryToTheName", func(t *testing.T) {
		req, err := requestFor(t, "acme-server")
		require.NoError(t, err, "an ordinary project name must be accepted")

		assert.Equal(t, "acme-server", req.Name, "the positional argument names the project")
		assert.Equal(t, "acme-server", req.Module, "the module path defaults to the project name")
		assert.True(t, filepath.IsAbs(req.Dir), "the target directory must be resolved to an absolute path")
		assert.Equal(t, "acme-server", filepath.Base(req.Dir), "the project lands in a directory named after it")
	})

	t.Run("AnExplicitModulePathWins", func(t *testing.T) {
		req, err := requestFor(t, "acme-server", "--module", "git.example.com/team/acme-server")
		require.NoError(t, err, "a qualified module path must be accepted")
		assert.Equal(t, "git.example.com/team/acme-server", req.Module, "an explicit module path must survive")
	})

	t.Run("AMissingNameIsRefused", func(t *testing.T) {
		_, err := requestFor(t)
		require.ErrorIs(t, err, ErrProjectNameRequired, "there is nothing to name the project after")
	})

	// go.mod is rendered from a template and nothing downstream validates it:
	// the only step that would notice is `go mod tidy`, whose failure is
	// deliberately reported as skippable. So an invalid path used to print
	// "Done. Next:" over a directory that cannot compile.
	t.Run("AnInvalidModulePathIsRefusedBeforeAnythingIsWritten", func(t *testing.T) {
		cases := []struct {
			name   string
			module string
		}{
			{name: "Space", module: "my app"},
			{name: "TrailingSlash", module: "acme/"},
			{name: "DotSegment", module: "acme/./server"},
			{name: "Empty", module: " "},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := requestFor(t, "acme", "--module", tc.module)
				require.ErrorIs(t, err, ErrInvalidModulePath,
					"module path %q would produce a go.mod the Go tool rejects", tc.module)
			})
		}
	})

	t.Run("ANameThatIsNotAModulePathIsRefusedToo", func(t *testing.T) {
		// The name doubles as the module path when --module is absent, so the
		// check has to catch it there as well.
		_, err := requestFor(t, "my app")
		require.ErrorIs(t, err, ErrInvalidModulePath, "a name standing in for the module path must clear the same bar")
	})

	t.Run("ANonEmptyTargetDirectoryIsRefused", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("keep me"), 0o644),
			"seeding the occupied directory must succeed")

		_, err := requestFor(t, "acme", "--path", dir)
		require.ErrorIs(t, err, ErrDirectoryNotEmpty, "scaffolding must never land on top of existing work")
	})

	t.Run("AnAbsentTargetDirectoryIsFine", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "not-yet")

		req, err := requestFor(t, "acme", "--path", dir)
		require.NoError(t, err, "the directory being created is the normal case")
		assert.Equal(t, dir, req.Dir, "the requested path must be honored")
	})
}

func TestGenerateProject(t *testing.T) {
	plan, root := newTestPlan(t)

	req := ProjectRequest{Name: "acme", Module: "acme", Dir: root, WithExample: true}
	require.NoError(t, GenerateProject(req, plan), "planning a project must succeed")

	report := applyPlan(t, plan)

	for _, path := range []string{
		"go.mod", "vef.yml", "Taskfile.yml", ".golangci.yml", ".gitignore",
		"cmd/server/main.go", "configs/application.toml",
		"internal/vef/module.go", "internal/example/module.go", "internal/example/model/models.go",
	} {
		assert.Contains(t, report, path, "the standard layout must include %s", path)
	}

	t.Run("NoGeneratedFileCarriesAnEmptyImportBlock", func(t *testing.T) {
		// gofmt keeps an empty import declaration, so a template placeholder
		// that nothing was inserted into ships verbatim.
		require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
				return err
			}

			source, err := os.ReadFile(path)
			require.NoError(t, err, "reading %s must succeed", path)
			assert.NotContains(t, string(source), "import ()", "%s must not ship an empty import block", path)

			return nil
		}), "walking the generated project must succeed")
	})

	t.Run("WithoutTheExampleModule", func(t *testing.T) {
		bare, bareRoot := newTestPlan(t)
		require.NoError(t, GenerateProject(ProjectRequest{Name: "acme", Module: "acme", Dir: bareRoot}, bare),
			"planning a project without the starter module must succeed")

		assert.NotContains(t, applyPlan(t, bare), "internal/example", "--with-example=false must generate no starter module")
	})
}
