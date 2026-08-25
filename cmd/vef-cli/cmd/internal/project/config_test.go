package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfig puts a vef.yml in a fresh temporary project root and returns it.
func writeConfig(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ConfigFileName), []byte(body), 0o644),
		"writing the fixture %s must succeed", ConfigFileName)

	return root
}

func TestLoadConfig(t *testing.T) {
	t.Run("AMissingFileGeneratesUnderTheDefaults", func(t *testing.T) {
		cfg, err := LoadConfig(t.TempDir())
		require.NoError(t, err, "vef.yml is optional, so its absence must not be an error")
		assert.Equal(t, DefaultConfig(), cfg, "a project with no config must generate under the documented defaults")
	})

	t.Run("OverridesReplaceOnlyWhatTheyState", func(t *testing.T) {
		cfg, err := LoadConfig(writeConfig(t, "module_root: app\nresource:\n  audit: false\n"))
		require.NoError(t, err, "a well-formed config must load")

		assert.Equal(t, "app", cfg.ModuleRoot, "a stated module root must win")
		assert.False(t, cfg.Resource.Audit, "an explicit false must survive rather than fall back to the default true")
		assert.Equal(t, DefaultConfig().Resource.Name, cfg.Resource.Name, "an unstated key must keep its default")
		assert.Equal(t, DefaultOps, cfg.Resource.Ops, "an unstated operation set must keep its default")
	})

	t.Run("AnEmptyPermissionTemplateIsHonored", func(t *testing.T) {
		cfg, err := LoadConfig(writeConfig(t, `resource:
  permission: ""
`))
		require.NoError(t, err, "a well-formed config must load")
		assert.Empty(t, cfg.Resource.Permission,
			"blanking the permission template is how a project says it authorizes elsewhere, so it must not be restored")
	})

	t.Run("BlankedRequiredValuesFallBackToTheirDefaults", func(t *testing.T) {
		cfg, err := LoadConfig(writeConfig(t, `module_root: "  "
resource:
  name: ""
  ops: []
`))
		require.NoError(t, err, "a partially blanked config must load")

		defaults := DefaultConfig()
		assert.Equal(t, defaults.ModuleRoot, cfg.ModuleRoot, "a resource cannot be generated into nowhere")
		assert.Equal(t, defaults.Resource.Name, cfg.Resource.Name, "a resource cannot be generated with no name")
		assert.Equal(t, defaults.Resource.Ops, cfg.Resource.Ops, "a resource with no operations answers nothing")
	})

	t.Run("AMalformedFileIsAnError", func(t *testing.T) {
		_, err := LoadConfig(writeConfig(t, "module_root: [this is not a string\n"))
		require.Error(t, err,
			"silently generating against defaults the project deliberately overrode would produce code that looks right and is wrong everywhere")
	})

	t.Run("AnUnusableModuleRootIsRefused", func(t *testing.T) {
		_, err := LoadConfig(writeConfig(t, "module_root: ../shared\n"))
		require.ErrorIs(t, err, ErrInvalidModuleRoot,
			"a module root outside the Go module can never form a valid import path")
	})
}

func TestCleanModuleRoot(t *testing.T) {
	t.Run("Normalizes", func(t *testing.T) {
		cases := []struct {
			name  string
			input string
			want  string
		}{
			{name: "PlainDirectory", input: "internal", want: "internal"},
			{name: "NestedDirectory", input: "internal/modules", want: "internal/modules"},
			{name: "SurroundingSlashes", input: "/internal/", want: "internal"},
			{name: "SurroundingSpace", input: "  internal  ", want: "internal"},
			// filepath.Join swallows a dot segment, so these produced the right
			// directories and the import path "acme/./md/model", which the
			// compiler rejects as an invalid path element.
			{name: "LeadingDotSegment", input: "./internal", want: "internal"},
			{name: "BareDotMeansTheProjectRoot", input: ".", want: ""},
			{name: "InteriorDotSegment", input: "internal/./modules", want: "internal/modules"},
			{name: "Empty", input: "", want: ""},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := cleanModuleRoot(tc.input)
				require.NoError(t, err, "cleanModuleRoot(%q) must be accepted", tc.input)
				assert.Equal(t, tc.want, got,
					"cleanModuleRoot(%q) must produce a value usable as both a directory and import path elements", tc.input)
			})
		}
	})

	t.Run("Rejects", func(t *testing.T) {
		cases := []struct {
			name  string
			input string
		}{
			{name: "EscapesTheModule", input: "../shared"},
			{name: "EscapesTheModuleAfterCleaning", input: "internal/../.."},
			{name: "InvalidCharacter", input: "internal modules"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := cleanModuleRoot(tc.input)
				require.ErrorIs(t, err, ErrInvalidModuleRoot,
					"cleanModuleRoot(%q) must be refused rather than emitted into every generated import", tc.input)
			})
		}
	})
}
