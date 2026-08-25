package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newProject builds a project rooted at a temporary directory with the given
// module root, bypassing vef.yml so a case can state the root directly.
func newProject(t *testing.T, moduleRoot string) *Project {
	t.Helper()

	cfg := DefaultConfig()
	cfg.ModuleRoot = moduleRoot

	return &Project{Root: t.TempDir(), ModulePath: "acme", Config: cfg}
}

func TestCleanModule(t *testing.T) {
	t.Run("KeepsAPlainName", func(t *testing.T) {
		got, err := CleanModule("md")
		require.NoError(t, err, "an ordinary module name must be accepted")
		assert.Equal(t, "md", got, "a clean name must survive unchanged")
	})

	t.Run("KeepsANestedName", func(t *testing.T) {
		got, err := CleanModule("hr/emp")
		require.NoError(t, err, "a nested module must be accepted")
		assert.Equal(t, "hr/emp", got, "nesting must survive unchanged")
	})

	t.Run("NormalizesSurroundingNoise", func(t *testing.T) {
		got, err := CleanModule("  /hr/emp/  ")
		require.NoError(t, err, "surrounding whitespace and slashes are noise, not an error")
		assert.Equal(t, "hr/emp", got, "the normalized form must be the bare path")
	})

	// Each of these used to reach the generator and surface as a syntax error
	// inside a file the generator itself had just rendered.
	rejected := []struct {
		name   string
		module string
	}{
		{name: "Empty", module: ""},
		{name: "OnlyWhitespace", module: "   "},
		{name: "OnlySlashes", module: "//"},
		{name: "DoubledSeparatorLeavesAnEmptySegment", module: "hr//emp"},
		{name: "HyphenIsNotAPackageName", module: "hr-emp"},
		{name: "SpaceIsNotAPackageName", module: "hr emp"},
		{name: "DotSegment", module: "hr/."},
		{name: "ParentSegment", module: "../shared"},
		{name: "DigitLeadingSegment", module: "2fa"},
		{name: "KeywordIsNotAPackageName", module: "range"},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CleanModule(tc.module)
			require.ErrorIs(t, err, ErrInvalidModule,
				"CleanModule(%q) must be refused by name rather than deferred to a parse error in generated code", tc.module)
		})
	}
}

func TestImportPath(t *testing.T) {
	cases := []struct {
		name       string
		moduleRoot string
		module     string
		pkg        string
		want       string
	}{
		{name: "UnderAModuleRoot", moduleRoot: "internal", module: "md", pkg: "model", want: "acme/internal/md/model"},
		{name: "NestedModuleRoot", moduleRoot: "internal/modules", module: "md", pkg: "model", want: "acme/internal/modules/md/model"},
		{name: "NestedModule", moduleRoot: "internal", module: "hr/emp", pkg: "service", want: "acme/internal/hr/emp/service"},
		{name: "ModuleItself", moduleRoot: "internal", module: "md", pkg: "", want: "acme/internal/md"},
		// module_root: "." normalizes to empty, and joining it blindly produced
		// "acme//md/model" — an import path with an empty element.
		{name: "NoModuleRootDropsTheEmptySegment", moduleRoot: "", module: "md", pkg: "model", want: "acme/md/model"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proj := newProject(t, tc.moduleRoot)
			assert.Equal(t, tc.want, proj.ImportPath(tc.module, tc.pkg),
				"ImportPath(%q, %q) under module root %q must be a well-formed import path", tc.module, tc.pkg, tc.moduleRoot)
		})
	}
}

func TestDirectories(t *testing.T) {
	proj := newProject(t, "internal")

	t.Run("ModuleDir", func(t *testing.T) {
		assert.Equal(t, filepath.Join(proj.Root, "internal", "hr", "emp"), proj.ModuleDir("hr/emp"),
			"a nested module must map onto nested directories")
	})

	t.Run("PackageDir", func(t *testing.T) {
		assert.Equal(t, filepath.Join(proj.Root, "internal", "md", "model"), proj.PackageDir("md", "model"),
			"a package must sit inside its module's directory")
	})

	t.Run("ModuleFile", func(t *testing.T) {
		assert.Equal(t, filepath.Join(proj.Root, "internal", "md", "module.go"), proj.ModuleFile("md"),
			"module.go is the file registrations are appended to")
	})

	t.Run("NoModuleRootPutsModulesAtTheProjectRoot", func(t *testing.T) {
		rootless := newProject(t, "")
		assert.Equal(t, filepath.Join(rootless.Root, "md"), rootless.ModuleDir("md"),
			"an empty module root must not leave a stray separator in the path")
	})
}

func TestModulePackageNameAndDomain(t *testing.T) {
	proj := newProject(t, "internal")

	t.Run("PackageNameIsTheLastSegment", func(t *testing.T) {
		assert.Equal(t, "emp", proj.ModulePackageName("hr/emp"), "a nested module's package is its leaf")
		assert.Equal(t, "md", proj.ModulePackageName("md"), "a flat module is its own package")
	})

	t.Run("DomainIsTheFirstSegment", func(t *testing.T) {
		assert.Equal(t, "hr", proj.Domain("hr/emp"), "permission tokens scope to the top-level domain")
		assert.Equal(t, "md", proj.Domain("md"), "a flat module is its own domain")
	})
}

func TestTemplates(t *testing.T) {
	proj := newProject(t, "internal")

	t.Run("ResourceNameUsesTheFullModulePath", func(t *testing.T) {
		assert.Equal(t, "hr/emp/holiday", proj.ResourceName("hr/emp", "holiday"),
			"a resource name must address the module it lives in")
	})

	t.Run("PermissionTokenScopesToTheDomain", func(t *testing.T) {
		assert.Equal(t, "hr.holiday.create", proj.PermissionToken("hr/emp", "holiday", "create"),
			"hr/emp and hr/ctr must issue tokens under one domain")
	})

	t.Run("AnEmptyPermissionTemplateGeneratesNoToken", func(t *testing.T) {
		proj.Config.Resource.Permission = ""
		assert.Empty(t, proj.PermissionToken("md", "holiday", "create"),
			"a project that authorizes elsewhere must get no permission at all")
	})
}

func TestRel(t *testing.T) {
	proj := newProject(t, "internal")

	t.Run("InsideTheProject", func(t *testing.T) {
		assert.Equal(t, "internal/md/model/holiday.go",
			proj.Rel(filepath.Join(proj.Root, "internal", "md", "model", "holiday.go")),
			"a path inside the project must render relative and slash-separated")
	})

	t.Run("OutsideTheProjectIsUnchanged", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "elsewhere.go")
		assert.Equal(t, outside, proj.Rel(outside), "a path outside the project has no meaningful relative form")
	})
}

func TestLocate(t *testing.T) {
	t.Run("FindsTheModuleRootFromANestedDirectory", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module acme\n\ngo 1.26.0\n"), 0o644),
			"writing the fixture go.mod must succeed")

		nested := filepath.Join(root, "internal", "md", "service")
		require.NoError(t, os.MkdirAll(nested, 0o755), "creating the nested directory must succeed")

		proj, err := Locate(nested)
		require.NoError(t, err, "Locate must walk up to the module root")

		resolved, err := filepath.EvalSymlinks(proj.Root)
		require.NoError(t, err, "resolving the located root must succeed")
		expected, err := filepath.EvalSymlinks(root)
		require.NoError(t, err, "resolving the fixture root must succeed")

		assert.Equal(t, expected, resolved, "the located root must be the directory holding go.mod")
		assert.Equal(t, "acme", proj.ModulePath, "the module path must come from go.mod")
	})

	t.Run("ReportsADirectoryThatIsNotInAModule", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := findModuleRoot(dir); err == nil {
			t.Skip("TMPDIR lives inside a Go module, so there is no directory here to test against")
		}

		_, err := Locate(dir)
		require.ErrorIs(t, err, ErrNotAGoModule, "generating outside a Go module must be refused")
	})

	t.Run("ReportsAGoModWithNoModulePath", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("go 1.26.0\n"), 0o644),
			"writing the malformed fixture must succeed")

		_, err := Locate(root)
		require.ErrorIs(t, err, ErrNotAGoModule, "a go.mod declaring no module gives the generators no import prefix")
	})
}
