package exportapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/cliout"
)

// envExportAPI mirrors vef.EnvExportAPI. It is duplicated rather than imported
// because this command runs the application as a subprocess: the constant it
// must set is the one that binary was compiled against, which may be a
// different framework version from the one that built this CLI.
const envExportAPI = "VEF_EXPORT_API"

var (
	// ErrManifestOutdated reports that --check found a manifest differing from
	// what the application currently exposes.
	ErrManifestOutdated = errors.New("the committed API manifest is out of date, re-run without --check")
	// ErrManifestInvalid reports that the application did not produce a
	// manifest on stdout.
	ErrManifestInvalid = errors.New("the application did not produce a valid API manifest")
	// ErrCheckNeedsAFile reports --check combined with stdout output, which has
	// nothing committed to compare against.
	ErrCheckNeedsAFile = errors.New("--check compares against a committed file, so it cannot be used with -o -")
)

// Command returns the export-api cobra command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export-api",
		Short: "Write the application's API manifest",
		Long: `Describe the application's API surface as data.

The manifest is produced by the application itself: this command runs its main
package with VEF_EXPORT_API set, so what it reports is what the binary really
registers — resource names, actions, permissions, audit flags, rate limits and
parameter shapes — rather than what a source scan guesses. Building the graph
does not start it, so no database, no listener and no scheduled job is involved
and the command is safe in a build pipeline.

The output is sorted and carries no timestamp, so it is meant to be committed:
its diff is the diff of your API contract. --check fails when the committed
file no longer matches, which is the gate that keeps the two in step.

Example:
  vef-cli export-api --app ./cmd/server -o api-manifest.json
  vef-cli export-api --app ./cmd/server -o api-manifest.json --check`,
		RunE: run,
	}

	cmd.Flags().String("app", "./cmd/server", "Main package of the application to describe")
	cmd.Flags().StringP("output", "o", "api-manifest.json", "File to write the manifest to, or - for stdout")
	cmd.Flags().Bool("check", false, "Fail if the existing file differs instead of rewriting it")

	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	// Past flag validation, so a failure from here is a failure, not a misuse:
	// its reason should not be buried under the usage text.
	cmd.SilenceUsage = true

	app, _ := cmd.Flags().GetString("app")
	output, _ := cmd.Flags().GetString("output")
	check, _ := cmd.Flags().GetBool("check")

	// Refused before doing any work, and refused rather than ignored: a CI job
	// written as `export-api --check -o -` would otherwise compare nothing and
	// pass every time, which is the one failure a gate must not have.
	if check && output == "-" {
		return ErrCheckNeedsAFile
	}

	out := termenv.DefaultOutput()
	cliout.PrintLabeledLine(os.Stderr, out, "Exporting API manifest from ", app, termenv.ANSICyan)

	manifest, err := runExport(cmd, app)
	if err != nil {
		return err
	}

	if output == "-" {
		_, err := os.Stdout.Write(manifest)

		return err
	}

	if check {
		return compare(output, manifest, out)
	}

	if err := os.WriteFile(output, manifest, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}

	cliout.PrintLabeledLine(os.Stderr, out, "Wrote ", output, termenv.ANSIGreen)

	return nil
}

// runExport builds and runs the application in export mode, returning the
// manifest it wrote to stdout.
func runExport(cmd *cobra.Command, app string) ([]byte, error) {
	var manifest, diagnostics bytes.Buffer

	// The application writes the manifest to stdout and its own progress to
	// stderr, so the two never interleave in the captured bytes.
	run := exec.CommandContext(cmd.Context(), "go", "run", app)

	run.Env = append(os.Environ(), envExportAPI+"=-")
	run.Stdout = &manifest
	run.Stderr = &diagnostics

	if err := run.Run(); err != nil {
		return nil, fmt.Errorf("run %s in export mode: %w\n%s", app, err, diagnostics.String())
	}

	// Anything the application prints to stdout of its own accord lands in the
	// same bytes as the manifest — a stray Println in a constructor, a
	// dependency announcing itself on init. Checking that what came back is
	// actually JSON turns that into one clear error instead of a corrupted
	// file, or a --check that compares against garbage.
	if !json.Valid(manifest.Bytes()) {
		return nil, fmt.Errorf("%w: %s wrote something other than a manifest to stdout:\n%s",
			ErrManifestInvalid, app, truncate(manifest.String()))
	}

	return manifest.Bytes(), nil
}

// truncate bounds a diagnostic excerpt so a large stray payload does not bury
// the error explaining it.
//
// The bound counts runes, not bytes. What is being truncated is whatever the
// application printed to stdout, and the framework's default language is
// Simplified Chinese, so cutting at a byte offset lands mid-rune far more often
// than not — leaving the message that explains the problem ending in mojibake.
func truncate(output string) string {
	const limit = 400

	output = strings.TrimSpace(output)

	runes := []rune(output)
	if len(runes) <= limit {
		return output
	}

	return string(runes[:limit]) + "…"
}

// compare implements --check.
func compare(path string, manifest []byte, out *termenv.Output) error {
	existing, err := os.ReadFile(path)

	switch {
	// Never committed is as much a drift as changed, and says so.
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s does not exist", ErrManifestOutdated, path)

	// Anything else — a permission denied, a directory in the way — is not
	// drift, and reporting it as such sends someone hunting for a file that is
	// sitting right there.
	case err != nil:
		return fmt.Errorf("read %s: %w", path, err)
	}

	if !bytes.Equal(existing, manifest) {
		return fmt.Errorf("%w: %s", ErrManifestOutdated, path)
	}

	cliout.PrintLabeledLine(os.Stderr, out, "Up to date: ", path, termenv.ANSIGreen)

	return nil
}
