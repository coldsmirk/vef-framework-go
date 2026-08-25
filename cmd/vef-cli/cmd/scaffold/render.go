package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/coldsmirk/vef-framework-go/cmd/vef-cli/cmd/internal/gopatch"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// render executes a template and returns gofmt-formatted Go source with the
// requested imports inserted into their gci groups.
//
// Imports are added through the same patcher that edits existing files rather
// than written by the template: grouping them correctly is fiddly enough that
// having two implementations means having one that is subtly wrong.
func render(name string, data any, imports []gopatch.Import, groups gopatch.ImportGroups) ([]byte, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}

	file := gopatch.New(name, buf.Bytes())

	for _, imported := range imports {
		if _, err := file.EnsureNamedImport(imported.Alias, imported.Path, groups); err != nil {
			return nil, fmt.Errorf("render %s: %w", name, err)
		}
	}

	if err := file.Format(); err != nil {
		return nil, err
	}

	return file.Source(), nil
}
