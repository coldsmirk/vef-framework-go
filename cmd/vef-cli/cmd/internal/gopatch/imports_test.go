package gopatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var testGroups = ImportGroups{
	Framework: "github.com/coldsmirk/vef-framework-go",
	Local:     "acme",
}

// patchImport applies EnsureImport to src and returns the formatted result.
func patchImport(t *testing.T, src, path string) (string, bool) {
	t.Helper()

	file := New("module.go", []byte(src))

	changed, err := file.EnsureImport(path, testGroups)
	require.NoError(t, err, "EnsureImport(%q) must succeed", path)
	require.NoError(t, file.Format(), "the patched file must still be valid Go")

	return string(file.Source()), changed
}

const groupedImports = `package md

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/orm"

	"acme/internal/md/resource"
)

var Module = vef.Module("app:md")
`

func TestEnsureImport(t *testing.T) {
	t.Run("SortsIntoAnExistingGroup", func(t *testing.T) {
		got, changed := patchImport(t, groupedImports, "acme/internal/md/payload")

		require.True(t, changed, "A missing import must be reported as a change")
		require.Contains(t, got, "\t\"acme/internal/md/payload\"\n\t\"acme/internal/md/resource\"\n",
			"The new local import must sort before the existing one inside its own group")
	})

	t.Run("AppendsToTheEndOfItsGroup", func(t *testing.T) {
		got, _ := patchImport(t, groupedImports, "github.com/coldsmirk/vef-framework-go/schema")

		require.Contains(t, got, "\t\"github.com/coldsmirk/vef-framework-go/orm\"\n\t\"github.com/coldsmirk/vef-framework-go/schema\"\n",
			"A framework import sorting last must land at the end of the framework group")
	})

	t.Run("IsIdempotent", func(t *testing.T) {
		got, changed := patchImport(t, groupedImports, "github.com/coldsmirk/vef-framework-go/orm")

		require.False(t, changed, "An already-imported path must not be reported as a change")
		require.Equal(t, groupedImports, got, "An already-imported path must leave the file untouched")
	})

	t.Run("OpensAMissingGroupInOrder", func(t *testing.T) {
		src := `package md

import (
	"context"

	"acme/internal/md/resource"
)
`

		got, changed := patchImport(t, src, "github.com/coldsmirk/vef-framework-go/orm")

		require.True(t, changed, "A missing import must be reported as a change")
		require.Equal(t, `package md

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/orm"

	"acme/internal/md/resource"
)
`, got, "A framework import must open its own group between standard and local")
	})

	t.Run("OpensATrailingGroup", func(t *testing.T) {
		src := "package md\n\nimport (\n\t\"context\"\n)\n"

		got, _ := patchImport(t, src, "acme/internal/md/model")

		require.Equal(t, "package md\n\nimport (\n\t\"context\"\n\n\t\"acme/internal/md/model\"\n)\n",
			got, "A local import must open a new trailing group")
	})

	t.Run("ExpandsASingleImportDeclaration", func(t *testing.T) {
		src := "package md\n\nimport \"context\"\n"

		got, changed := patchImport(t, src, "acme/internal/md/model")

		require.True(t, changed, "Expanding a single import must be reported as a change")
		require.Equal(t, "package md\n\nimport (\n\t\"context\"\n\n\t\"acme/internal/md/model\"\n)\n",
			got, "A one-line import must become a grouped block")
	})

	t.Run("AddsTheFirstImportBlock", func(t *testing.T) {
		src := "package md\n\nvar Module = 1\n"

		got, changed := patchImport(t, src, "context")

		require.True(t, changed, "Adding the first import must be reported as a change")
		require.Equal(t, "package md\n\nimport (\n\t\"context\"\n)\n\nvar Module = 1\n",
			got, "A file with no imports must gain a block after the package clause")
	})
}

func TestClassifyImport(t *testing.T) {
	cases := []struct {
		name string
		path string
		want int
	}{
		{name: "Standard", path: "context", want: groupStandard},
		{name: "StandardNested", path: "net/http", want: groupStandard},
		{name: "External", path: "github.com/gofiber/fiber/v3", want: groupExternal},
		{name: "FrameworkRoot", path: "github.com/coldsmirk/vef-framework-go", want: groupFramework},
		{name: "FrameworkPackage", path: "github.com/coldsmirk/vef-framework-go/orm", want: groupFramework},
		{name: "Local", path: "acme/internal/md/model", want: groupLocal},
		{name: "LocalLookalikeIsExternal", path: "acmelabs/pkg", want: groupExternal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, classifyImport(tc.path, testGroups),
				"classifyImport(%q) must place the path in the gci section it belongs to", tc.path)
		})
	}
}

func TestEnsureImportNameConflicts(t *testing.T) {
	t.Run("AnAliasedPathIsAConflictNotASatisfiedImport", func(t *testing.T) {
		file := New("module.go", []byte("package md\n\nimport (\n\tres \"acme/internal/md/resource\"\n)\n"))

		_, err := file.EnsureImport("acme/internal/md/resource", testGroups)
		require.ErrorIs(t, err, ErrImportNameConflict,
			"the path is imported but bound to res, so code referencing resource. would not compile — and the file would still parse, so nothing downstream would catch it")
	})

	t.Run("ABlankImportIsAConflict", func(t *testing.T) {
		file := New("module.go", []byte("package md\n\nimport (\n\t_ \"acme/internal/md/resource\"\n)\n"))

		_, err := file.EnsureImport("acme/internal/md/resource", testGroups)
		require.ErrorIs(t, err, ErrImportNameConflict, "a blank import binds nothing")
	})

	t.Run("TheMatchingAliasIsSatisfied", func(t *testing.T) {
		file := New("module.go", []byte("package md\n\nimport (\n\tsysmodel \"acme/internal/sys/model\"\n)\n"))

		changed, err := file.EnsureNamedImport("sysmodel", "acme/internal/sys/model", testGroups)
		require.NoError(t, err, "an import already bound to the requested alias is satisfied")
		require.False(t, changed, "nothing needs adding")
	})
}

// TestEnsureImportKeepsACommentWithItsImport pins that a new entry sorting
// before a documented import goes above the comment, not between the comment
// and the import it describes.
func TestEnsureImportKeepsACommentWithItsImport(t *testing.T) {
	got, changed := patchImport(t, `package md

import (
	"context"

	// resource wires the module's API resources.
	"acme/internal/md/resource"
)
`, "acme/internal/md/payload")

	require.True(t, changed, "the import must be added")
	require.Equal(t, `package md

import (
	"context"

	"acme/internal/md/payload"
	// resource wires the module's API resources.
	"acme/internal/md/resource"
)
`, got, "the comment must stay attached to the import it documents")
}
