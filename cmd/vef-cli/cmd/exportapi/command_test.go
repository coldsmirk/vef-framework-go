package exportapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const manifest = `{"framework":"v0.0.0","resources":[]}`

// writeManifest puts a manifest file in a temporary directory and returns its
// path.
func writeManifest(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "api-manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644), "writing the fixture manifest must succeed")

	return path
}

func TestCompare(t *testing.T) {
	output := termenv.DefaultOutput()

	t.Run("MatchingManifestPasses", func(t *testing.T) {
		require.NoError(t, compare(writeManifest(t, manifest), []byte(manifest), output),
			"a committed manifest matching the application must satisfy --check")
	})

	t.Run("DriftedManifestFails", func(t *testing.T) {
		err := compare(writeManifest(t, `{"framework":"v0.0.0","resources":[{"name":"md/holiday"}]}`), []byte(manifest), output)

		require.ErrorIs(t, err, ErrManifestOutdated, "a manifest that no longer matches must fail the check")
	})

	t.Run("MissingManifestFailsAsOutdated", func(t *testing.T) {
		err := compare(filepath.Join(t.TempDir(), "absent.json"), []byte(manifest), output)

		require.ErrorIs(t, err, ErrManifestOutdated,
			"a manifest that was never committed is as much a drift as one that changed")
		require.ErrorContains(t, err, "does not exist", "the message must distinguish absent from differing")
	})

	// A read that fails for any other reason is not drift, and calling it
	// "does not exist" sends someone hunting for a file that is sitting there.
	t.Run("AnUnreadableManifestIsNotReportedAsDrift", func(t *testing.T) {
		err := compare(t.TempDir(), []byte(manifest), output)

		require.Error(t, err, "a directory where the manifest should be must be reported")
		require.NotErrorIs(t, err, ErrManifestOutdated, "an unreadable path is a different failure from a drifted one")
		assert.NotContains(t, err.Error(), "does not exist", "the path does exist; it just cannot be read as a manifest")
	})
}

// TestCheckWithStdoutIsRefused pins that the CI gate cannot be written in a
// form that always passes: --check compares against a committed file, and
// combined with -o - there was nothing to compare, so it exited 0 while drifted.
func TestCheckWithStdoutIsRefused(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{"--check", "-o", "-"})
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))

	require.ErrorIs(t, cmd.Execute(), ErrCheckNeedsAFile,
		"a gate that silently compares nothing is worse than no gate")
}

func TestTruncate(t *testing.T) {
	t.Run("KeepsShortOutputWhole", func(t *testing.T) {
		require.Equal(t, "boom", truncate("  boom\n"), "surrounding whitespace is noise in a diagnostic")
	})

	t.Run("BoundsLongOutput", func(t *testing.T) {
		truncated := truncate(strings.Repeat("x", 900))

		require.Len(t, []rune(truncated), 401, "a long stray payload must not bury the error explaining it")
		require.True(t, strings.HasSuffix(truncated, "…"), "truncation must be visible")
	})

	// What gets truncated is whatever the application printed to stdout, and
	// the framework's default language is Simplified Chinese — so a byte-offset
	// cut lands mid-rune far more often than not, and the message explaining
	// the problem ends in mojibake.
	t.Run("CutsOnRuneBoundaries", func(t *testing.T) {
		truncated := truncate(strings.Repeat("配置加载完成", 200))

		assert.True(t, utf8.ValidString(truncated), "the excerpt must stay valid UTF-8")
		require.Len(t, []rune(truncated), 401, "the bound must count runes, not bytes")
		assert.True(t, strings.HasSuffix(truncated, "载…"), "the 400th rune must be kept whole, not split")
	})

	t.Run("KeepsMultibyteOutputUnderTheBoundWhole", func(t *testing.T) {
		short := strings.Repeat("配置", 10)

		assert.Equal(t, short, truncate(short),
			"a string of 20 runes is under the bound however many bytes it occupies")
	})
}

func TestCommandFlags(t *testing.T) {
	cmd := Command()

	t.Run("DefaultsToTheConventionalLayout", func(t *testing.T) {
		app, err := cmd.Flags().GetString("app")
		require.NoError(t, err, "the app flag must be declared")
		require.Equal(t, "./cmd/server", app, "the default must be the layout `new project` generates")

		output, err := cmd.Flags().GetString("output")
		require.NoError(t, err, "the output flag must be declared")
		require.Equal(t, "api-manifest.json", output, "the default output must be the file meant to be committed")
	})

	t.Run("CheckDefaultsOff", func(t *testing.T) {
		check, err := cmd.Flags().GetBool("check")
		require.NoError(t, err, "the check flag must be declared")
		require.False(t, check, "exporting must write by default; --check is the CI gate, opted into")
	})
}
