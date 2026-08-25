package vef

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apiexport"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// EnvExportAPI names the environment variable that turns a normal Run into an
// API manifest export: set it to a file path, or to "-" for stdout, and the
// application describes its API surface and exits instead of serving.
//
// It is an environment variable rather than a flag because Run does not own
// the process's flag set — a host's main may already define its own — and
// because it sits beside the framework's existing VEF_* switches.
const EnvExportAPI = "VEF_EXPORT_API"

// ErrEngineNotInspectable reports an API engine that does not implement
// api.EngineInspector, which means something replaced the framework's own.
var ErrEngineNotInspectable = errors.New("vef: the API engine does not support inspection")

// ExportAPI writes the application's API manifest to w and returns without
// starting it.
//
// The graph is built but never started, which is what makes the export usable
// in a container build or a CI job: resources register during construction, so
// the whole API surface is known before a single lifecycle hook runs, and the
// data source is opened lazily. Nothing connects, migrates or listens.
//
// It is for a process that exits afterwards, which is how Run uses it. A few
// constructors start their own goroutines rather than registering a lifecycle
// hook — the in-memory caches behind the session store and login guard each
// run a GC ticker — and since the graph is never started, it is never stopped
// either, so those are not reclaimed. Calling this repeatedly inside a
// long-running process accumulates them.
func ExportAPI(w io.Writer, options ...fx.Option) error {
	var (
		manifest apiexport.Manifest
		captured error
	)

	app := fx.New(append(
		exportOptions(options...),
		fx.Invoke(func(engine api.Engine) {
			inspector, ok := engine.(api.EngineInspector)
			if !ok {
				captured = ErrEngineNotInspectable

				return
			}

			manifest = apiexport.Build(inspector.Operations())
		}),
	)...)

	if err := app.Err(); err != nil {
		return fmt.Errorf("build application graph: %w", err)
	}

	if captured != nil {
		return captured
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	return encoder.Encode(manifest)
}

// exportOptions assembles the same graph Run does, minus the invoke that starts
// the HTTP server. Sharing bootOptions would start it; duplicating the module
// list would let the exported surface drift from the served one, so the module
// order comes from the same assembler and only the trailing invoke differs.
func exportOptions(options ...fx.Option) []fx.Option {
	return append(baseOptions(options...), fx.NopLogger)
}

// exportAPIIfRequested performs the manifest export when the environment asks
// for it, reporting whether it handled the run.
func exportAPIIfRequested(options ...fx.Option) bool {
	target := os.Getenv(EnvExportAPI)
	if target == "" {
		return false
	}

	// Module loggers write to stdout, which is also where a manifest bound for
	// "-" goes: boot chatter would land in the middle of the JSON. This entry
	// point owns the process, so it may silence them — a genuine failure still
	// surfaces, as the returned error, on stderr. An explicit VEF_LOG_LEVEL
	// wins, which is how an export that goes wrong is debugged.
	if os.Getenv(config.EnvLogLevel) == "" {
		ilogx.SetLevel(logx.LevelPanic)
	}

	if err := exportAPITo(target, options...); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		// This is a process mode, not a library call: an export that failed must
		// leave a non-zero status for the pipeline that asked for it, and there
		// is no caller left to hand the error to — Run does not return one.
		os.Exit(1) //nolint:revive // deliberate: the export mode owns the process
	}

	return true
}

// exportAPITo writes the manifest to a path, or to stdout for "-".
func exportAPITo(target string, options ...fx.Option) error {
	if target == "-" {
		return ExportAPI(os.Stdout, options...)
	}

	// The destination comes from the operator's own environment, naming where
	// they want their manifest; there is no untrusted input on this path.
	file, err := os.Create(target) //nolint:gosec // operator-supplied output path
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}

	defer func() { _ = file.Close() }()

	if err := ExportAPI(file, options...); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "API manifest written to %s\n", target)

	return nil
}
