package gopatch

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"github.com/coldsmirk/go-collections"
)

// ErrImportNameConflict reports that a file already imports the required path
// under a name the generated code does not reference.
var ErrImportNameConflict = errors.New("import is bound to a different name")

// ImportGroups names the prefixes that separate a project's import groups.
// They mirror the gci sections the framework and its consumers lint against:
// standard, external, framework, then the project's own module.
type ImportGroups struct {
	// Framework is the framework's module path.
	Framework string
	// Local is the project's own module path.
	Local string
}

// group indexes, in the order the groups appear in a formatted file.
const (
	groupStandard = iota
	groupExternal
	groupFramework
	groupLocal
)

// EnsureImport adds path to the file's import block when it is not already
// imported, inserting it in sorted position inside its own group so the result
// satisfies gci without a follow-up fix pass.
func (f *File) EnsureImport(path string, groups ImportGroups) (bool, error) {
	return f.EnsureNamedImport("", path, groups)
}

// EnsureNamedImport is EnsureImport with an explicit local name, which a
// generated file needs whenever it reaches into two packages sharing a final
// segment — the audit user model living in another module's model package
// beside the entity's own is the ordinary case.
func (f *File) EnsureNamedImport(alias, path string, groups ImportGroups) (bool, error) {
	fset, file, err := f.parse()
	if err != nil {
		return false, err
	}

	decl := findImportDecl(file)
	if decl == nil {
		return f.insertFirstImport(fset, file, alias, path)
	}

	specs := importSpecs(decl)
	for _, spec := range specs {
		if importPath(spec) != path {
			continue
		}

		// The path is present, but only under the name the caller expects is
		// it actually satisfied. A blank, dot, or differently-aliased import
		// binds something else — or nothing — so reporting "already imported"
		// would let the caller emit a selector that resolves to no package.
		// The file still parses, so gofmt would not catch it and the plan
		// would write uncompilable code.
		if bound := boundName(spec, path); bound != expectedName(alias, path) {
			return false, fmt.Errorf("%w: %s is imported as %q but %q is needed",
				ErrImportNameConflict, path, bound, expectedName(alias, path))
		}

		return false, nil
	}

	if !decl.Lparen.IsValid() {
		return f.expandSingleImport(fset, decl, specs, alias, path, groups)
	}

	f.insertImportSpec(fset, decl, specs, alias, path, groups)

	return true, nil
}

// importLine renders one import block entry.
func importLine(alias, path string) string {
	if alias == "" {
		return "\t" + strconv.Quote(path) + "\n"
	}

	return "\t" + alias + " " + strconv.Quote(path) + "\n"
}

// insertImportSpec splices a new import line into an existing parenthesized
// block, opening a new group when the path's group has no members yet.
func (f *File) insertImportSpec(
	fset *token.FileSet,
	decl *ast.GenDecl,
	specs []*ast.ImportSpec,
	alias, path string,
	groups ImportGroups,
) {
	target := classifyImport(path, groups)
	line := importLine(alias, path)

	// Sorted position inside the group, when the group already exists.
	var lastInGroup *ast.ImportSpec

	for _, spec := range specs {
		switch group := classifyImport(importPath(spec), groups); {
		case group != target:
			continue
		case importPath(spec) > path:
			f.spliceAtLineStart(fset, specStart(spec), line)

			return
		default:
			lastInGroup = spec
		}
	}

	if lastInGroup != nil {
		f.spliceAfterLine(fset, lastInGroup.End(), line)

		return
	}

	// The group is empty: open it next to the neighboring groups.
	for _, spec := range specs {
		if classifyImport(importPath(spec), groups) > target {
			f.spliceAtLineStart(fset, spec.Pos(), line+"\n")

			return
		}
	}

	// An empty block gets the import alone; a populated one gets a blank line
	// first so the new group stays visually separated.
	if len(specs) == 0 {
		f.spliceAtLineStart(fset, decl.Rparen, line)

		return
	}

	f.spliceAtLineStart(fset, decl.Rparen, "\n"+line)
}

// expandSingleImport rewrites a one-line `import "x"` declaration into a block
// holding both the existing import and the new one.
func (f *File) expandSingleImport(
	fset *token.FileSet,
	decl *ast.GenDecl,
	specs []*ast.ImportSpec,
	alias, path string,
	groups ImportGroups,
) (bool, error) {
	entries := make([]importEntry, 0, len(specs)+1)

	for _, spec := range specs {
		entries = append(entries, importEntry{alias: specAlias(spec), path: importPath(spec)})
	}

	entries = append(entries, importEntry{alias: alias, path: path})

	block := renderImportBlock(entries, groups)
	f.splice(fset.Position(decl.Pos()).Offset, fset.Position(decl.End()).Offset, block)

	return true, nil
}

// insertFirstImport adds an import block to a file that has none, right after
// the package clause.
func (f *File) insertFirstImport(fset *token.FileSet, file *ast.File, alias, path string) (bool, error) {
	block := renderImportBlock([]importEntry{{alias: alias, path: path}}, ImportGroups{})

	offset := fset.Position(file.Name.End()).Offset
	f.splice(offset, offset, "\n\n"+block)

	return true, nil
}

// Import is one import a generated file needs: a path and an optional local
// name for the case where two packages share a final segment.
type Import struct {
	// Alias is the local name, empty when the package name is used as-is.
	Alias string
	// Path is the import path.
	Path string
}

// importEntry is one import declaration line.
type importEntry struct {
	alias string
	path  string
}

// renderImportBlock renders a complete import declaration with the entries
// grouped and sorted.
func renderImportBlock(entries []importEntry, groups ImportGroups) string {
	grouped := map[int][]importEntry{}
	for _, entry := range entries {
		group := classifyImport(entry.path, groups)
		grouped[group] = append(grouped[group], entry)
	}

	var (
		block   strings.Builder
		written bool
	)

	block.WriteString("import (\n")

	for group := groupStandard; group <= groupLocal; group++ {
		members := grouped[group]
		if len(members) == 0 {
			continue
		}

		if written {
			block.WriteString("\n")
		}

		for _, entry := range sortedUnique(members) {
			block.WriteString(importLine(entry.alias, entry.path))
		}

		written = true
	}

	block.WriteString(")")

	return block.String()
}

// spliceAtLineStart inserts text at the beginning of the line holding pos.
func (f *File) spliceAtLineStart(fset *token.FileSet, pos token.Pos, text string) {
	start := lineStartOffset(f.src, fset.Position(pos).Offset)
	f.splice(start, start, text)
}

// spliceAfterLine inserts text immediately after the line holding pos.
func (f *File) spliceAfterLine(fset *token.FileSet, pos token.Pos, text string) {
	offset := fset.Position(pos).Offset
	for offset < len(f.src) && f.src[offset] != '\n' {
		offset++
	}

	if offset < len(f.src) {
		offset++
	}

	f.splice(offset, offset, text)
}

// specStart returns where an import spec's own text begins, counting a comment
// written above it. Inserting at the spec's Pos would land between that comment
// and the import it documents, silently reattaching it to the new entry.
func specStart(spec *ast.ImportSpec) token.Pos {
	if spec.Doc != nil {
		return spec.Doc.Pos()
	}

	return spec.Pos()
}

// boundName is the identifier an import spec actually binds.
func boundName(spec *ast.ImportSpec, path string) string {
	if spec.Name != nil {
		return spec.Name.Name
	}

	return packageName(path)
}

// expectedName is the identifier the caller will reference the import by.
func expectedName(alias, path string) string {
	if alias != "" {
		return alias
	}

	return packageName(path)
}

// packageName guesses the identifier an unaliased import binds. It is the
// path's last segment, which is right for every package this tool generates
// against and for the framework's own; a package whose name differs from its
// directory would be reported as a conflict, which is a safe direction to err.
func packageName(path string) string {
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		return path[slash+1:]
	}

	return path
}

// classifyImport places a path in the group order a formatted file uses.
func classifyImport(path string, groups ImportGroups) int {
	switch {
	case groups.Local != "" && underPrefix(path, groups.Local):
		return groupLocal
	case groups.Framework != "" && underPrefix(path, groups.Framework):
		return groupFramework
	case isStandard(path):
		return groupStandard
	default:
		return groupExternal
	}
}

// underPrefix reports whether path is prefix itself or a package inside it,
// so a module named acme does not capture acmelabs/pkg.
func underPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// standardRoots are the standard library's top-level import path segments.
//
// The usual shortcut — "no dot in the first segment means standard library" —
// misfiles every dotless module path, and those are common in exactly the
// projects this tool generates into: a go.mod saying `module smp-server`
// imports smp-server/internal/... , which the shortcut would sort into the
// standard group. Matching the real root set costs one literal and cannot
// misclassify.
var standardRoots = collections.NewHashSetFrom(
	"archive", "arena", "bufio", "builtin", "bytes", "cmp", "compress", "container",
	"context", "crypto", "database", "debug", "embed", "encoding", "errors", "expvar",
	"flag", "fmt", "go", "hash", "html", "image", "index", "io", "iter", "log", "maps",
	"math", "mime", "net", "os", "path", "plugin", "reflect", "regexp", "runtime",
	"slices", "sort", "strconv", "strings", "structs", "sync", "syscall", "testing",
	"text", "time", "unicode", "unique", "unsafe", "weak",
)

// isStandard reports whether a path belongs to the standard library.
func isStandard(path string) bool {
	first, _, _ := strings.Cut(path, "/")

	return standardRoots.Contains(first)
}

func findImportDecl(file *ast.File) *ast.GenDecl {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if ok && gen.Tok == token.IMPORT {
			return gen
		}
	}

	return nil
}

func importSpecs(decl *ast.GenDecl) []*ast.ImportSpec {
	specs := make([]*ast.ImportSpec, 0, len(decl.Specs))

	for _, spec := range decl.Specs {
		if imported, ok := spec.(*ast.ImportSpec); ok {
			specs = append(specs, imported)
		}
	}

	return specs
}

func importPath(spec *ast.ImportSpec) string {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return spec.Path.Value
	}

	return path
}

// specAlias returns an import spec's explicit local name, empty when it has
// none.
func specAlias(spec *ast.ImportSpec) string {
	if spec.Name == nil {
		return ""
	}

	return spec.Name.Name
}

// sortedUnique orders entries by import path and drops duplicate paths.
func sortedUnique(entries []importEntry) []importEntry {
	seen := make(map[string]bool, len(entries))
	out := make([]importEntry, 0, len(entries))

	for _, entry := range entries {
		if !seen[entry.path] {
			seen[entry.path] = true

			out = append(out, entry)
		}
	}

	slices.SortFunc(out, func(a, b importEntry) int { return strings.Compare(a.path, b.path) })

	return out
}
