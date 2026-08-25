package scaffold

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/muesli/termenv"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/gopatch"
	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/project"
)

// ErrFileExists reports that a generated file is already present. Scaffolding
// hands a file to the developer the moment it is written, so overwriting one
// has to be asked for.
var ErrFileExists = errors.New("file already exists (pass --force to overwrite)")

// actionKind labels what a planned change does to a path.
type actionKind string

const (
	actionCreate actionKind = "create"
	actionUpdate actionKind = "update"
	actionSkip   actionKind = "skip"
	// actionNote writes nothing; it carries something the developer has to do
	// by hand, so a step the generator could not take is still reported.
	actionNote actionKind = "note"
)

// change is one planned filesystem effect.
type change struct {
	path    string
	kind    actionKind
	content []byte
	// reason explains a skip, so a no-op run still tells the developer why.
	reason string
}

// Plan collects every effect a scaffolding command would have, so the whole
// command can be previewed with --dry-run and so a failure part-way through
// planning leaves nothing half-written on disk.
type Plan struct {
	project *project.Project
	changes []change
}

// NewPlan starts an empty plan for a project.
func NewPlan(proj *project.Project) *Plan {
	return &Plan{project: proj}
}

// AddFile plans a generated file. An existing path is skipped unless force is
// set, and an unchanged path is skipped either way so re-running a generator
// reports honestly instead of touching mtimes.
func (p *Plan) AddFile(path string, content []byte, force bool) {
	existing, err := os.ReadFile(path)

	switch {
	case err != nil:
		p.changes = append(p.changes, change{path: path, kind: actionCreate, content: content})
	case string(existing) == string(content):
		p.changes = append(p.changes, change{path: path, kind: actionSkip, reason: "already up to date"})
	case !force:
		p.changes = append(p.changes, change{path: path, kind: actionSkip, reason: ErrFileExists.Error()})
	default:
		p.changes = append(p.changes, change{path: path, kind: actionUpdate, content: content})
	}
}

// AddPatch plans an edit to an existing file. The patch runs now, against the
// file's current contents, so planning surfaces a malformed target before any
// write happens; a patch reporting no change is recorded as a skip.
func (p *Plan) AddPatch(path string, patch func(*gopatch.File) (bool, error)) error {
	return p.AddPatchOrCreate(path, "", patch)
}

// AddPatchOrCreate is AddPatch over a file that may not exist yet, starting
// from initial when it does not. It is what lets a generator target a module
// that has not been created: the registry files a resource must be appended to
// are the same files that define the module, so creating them on demand is the
// difference between "generate into a new module" working and failing with a
// missing-file error the developer has to resolve by hand.
func (p *Plan) AddPatchOrCreate(path, initial string, patch func(*gopatch.File) (bool, error)) error {
	kind := actionUpdate

	file, err := gopatch.Open(path)
	if errors.Is(err, os.ErrNotExist) && initial != "" {
		file, err, kind = gopatch.New(path, []byte(initial)), nil, actionCreate
	}

	if err != nil {
		return err
	}

	changed, err := patch(file)
	if err != nil {
		return err
	}

	if !changed && kind == actionUpdate {
		p.changes = append(p.changes, change{path: path, kind: actionSkip, reason: "already registered"})

		return nil
	}

	if err := file.Format(); err != nil {
		return err
	}

	p.changes = append(p.changes, change{path: path, kind: kind, content: file.Source()})

	return nil
}

// AddContent plans a file whose content the caller has already produced,
// overwriting what is there. Use it for a file the plan itself patched rather
// than rendered.
func (p *Plan) AddContent(path string, content []byte) {
	p.changes = append(p.changes, change{path: path, kind: actionUpdate, content: content})
}

// Note records something the generator could not do and the developer must,
// so a step it had to skip is reported rather than silently dropped.
func (p *Plan) Note(message string) {
	p.changes = append(p.changes, change{kind: actionNote, reason: message})
}

// Empty reports whether the plan would change nothing.
func (p *Plan) Empty() bool {
	for _, c := range p.changes {
		if c.kind != actionSkip && c.kind != actionNote {
			return false
		}
	}

	return true
}

// Apply writes every planned change, creating parent directories as needed,
// and reports the plan to out.
func (p *Plan) Apply(out io.Writer, output *termenv.Output) error {
	for _, c := range p.changes {
		if c.kind == actionSkip || c.kind == actionNote {
			p.report(out, output, c)

			continue
		}

		if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", c.path, err)
		}

		if err := os.WriteFile(c.path, c.content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", c.path, err)
		}

		p.report(out, output, c)
	}

	return nil
}

// Preview reports the plan without writing anything, printing the full content
// of each new file so --dry-run shows what would land rather than only where.
func (p *Plan) Preview(out io.Writer, output *termenv.Output) {
	for _, c := range p.changes {
		p.report(out, output, c)

		if c.kind == actionSkip || c.kind == actionNote {
			continue
		}

		_, _ = fmt.Fprintln(out, output.String(indent(string(c.content))).Foreground(termenv.ANSIBrightBlack))
	}
}

func (p *Plan) report(out io.Writer, output *termenv.Output, c change) {
	label, color := "  create ", termenv.ANSIGreen

	switch c.kind {
	case actionUpdate:
		label, color = "  update ", termenv.ANSIYellow
	case actionSkip:
		label, color = "  skip   ", termenv.ANSIBrightBlack
	case actionNote:
		_, _ = fmt.Fprintln(out, output.String("  TODO   "+c.reason).Foreground(termenv.ANSIYellow))

		return
	}

	line := label + p.project.Rel(c.path)
	if c.reason != "" {
		line += " (" + c.reason + ")"
	}

	_, _ = fmt.Fprintln(out, output.String(line).Foreground(color))
}

// indent shifts a rendered file so it reads as a block inside the plan output.
func indent(content string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "    │ " + line
	}

	return strings.Join(lines, "\n")
}
