package gopatch

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

var (
	// ErrVarNotFound reports that the file declares no package-level variable
	// under the requested name.
	ErrVarNotFound = errors.New("package-level variable not found")
	// ErrNotACall reports that the named variable is not initialized from a
	// function call, so there is no argument list to append to.
	ErrNotACall = errors.New("variable is not initialized from a call expression")
)

// File is a Go source file patched in memory.
//
// Every mutation splices text at a position located through the AST rather
// than re-printing the parsed file. Re-printing would rewrite parts of the
// file the generator never touched — and these are files the developer owns,
// where a one-line addition must read as a one-line diff.
type File struct {
	// Path is where the file was read from, used in error messages and as the
	// filename the parser reports positions against.
	Path string

	src []byte
}

// Open reads a Go source file for patching.
func Open(path string) (*File, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return &File{Path: path, src: src}, nil
}

// New wraps in-memory source for patching, for tests and for a file the caller
// has already rendered.
func New(path string, src []byte) *File {
	return &File{Path: path, src: src}
}

// Source returns the current buffer.
func (f *File) Source() []byte {
	return f.src
}

// AppendCallArg appends arg to the argument list of the call initializing the
// package-level variable varName — the shape both `var Module = vef.Module(...)`
// registration blocks take.
//
// It is idempotent: an argument already present (compared with whitespace
// normalized) leaves the file untouched and reports false, so re-running a
// generator over an existing entity is a no-op rather than a duplicate
// registration that fails at boot.
func (f *File) AppendCallArg(varName, arg string) (bool, error) {
	fset, file, err := f.parse()
	if err != nil {
		return false, err
	}

	call, err := findVarCall(fset, file, varName)
	if err != nil {
		return false, err
	}

	for _, existing := range call.Args {
		if renderNode(fset, existing) == normalizeExpr(arg) {
			return false, nil
		}
	}

	f.insertIntoList(fset, call.Lparen, call.Rparen, arg, argumentList)

	return true, nil
}

// AppendCallArgIn appends arg to the argument list of a call to callee made
// inside the named function — the shape an application's entry point takes,
// where the module list is an argument list rather than a variable.
//
// It reports false when the file declares no such function or the function
// makes no such call, so a caller can degrade to telling the developer what to
// add rather than failing over a project that wires itself differently.
func (f *File) AppendCallArgIn(funcName, callee, arg string) (bool, error) {
	fset, file, err := f.parse()
	if err != nil {
		return false, err
	}

	call := findCallInFunc(fset, file, funcName, callee)
	if call == nil {
		return false, nil
	}

	for _, existing := range call.Args {
		if renderNode(fset, existing) == normalizeExpr(arg) {
			return false, nil
		}
	}

	f.insertIntoList(fset, call.Lparen, call.Rparen, arg, argumentList)

	return true, nil
}

// AppendVarSpec appends `name = value` to the file's first parenthesized
// package-level var block — the `var ( XxxModel = new(Xxx) ... )` registry
// shape. A file with no such block gets one appended.
//
// Like AppendCallArg it is idempotent on the declared name.
func (f *File) AppendVarSpec(name, value string) (bool, error) {
	fset, file, err := f.parse()
	if err != nil {
		return false, err
	}

	block := findVarBlock(file)
	if block == nil {
		f.appendText(fmt.Sprintf("\nvar (\n\t%s = %s\n)\n", name, value))

		return true, nil
	}

	for _, spec := range block.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for _, declared := range value.Names {
			if declared.Name == name {
				return false, nil
			}
		}
	}

	f.insertIntoList(fset, block.Lparen, block.Rparen, name+" = "+value, declarationList)

	return true, nil
}

// Format runs gofmt over the buffer, which realigns anything the insertions
// disturbed (a var block's assignment column, most visibly).
func (f *File) Format() error {
	formatted, err := format.Source(f.src)
	if err != nil {
		return fmt.Errorf("format %s: %w", f.Path, err)
	}

	f.src = formatted

	return nil
}

// listStyle describes how a parenthesized list separates its entries: an
// argument list is comma-separated and may collapse onto one line, while a
// declaration block separates by newline only.
type listStyle struct {
	terminator string
	inlinable  bool
}

var (
	argumentList    = listStyle{terminator: ",", inlinable: true}
	declarationList = listStyle{}
)

// insertIntoList splices an entry into a parenthesized list delimited by lparen
// and rparen.
//
// A multi-line list takes the entry as its own line indented like the closing
// parenthesis plus one tab; a single-line argument list takes it inline.
// Callers always append rather than sort: registration order in these blocks
// is meaningful to a reader, who follows it as the record of what the module
// gained and when.
func (f *File) insertIntoList(fset *token.FileSet, lparen, rparen token.Pos, entry string, style listStyle) {
	closeOffset := fset.Position(rparen).Offset

	if fset.Position(lparen).Line == fset.Position(rparen).Line {
		if style.inlinable {
			f.splice(closeOffset, closeOffset, ", "+entry)

			return
		}

		// A collapsed declaration block (var ()) has no line of its own to
		// indent against; gofmt reflows what this opens up.
		f.splice(closeOffset, closeOffset, "\n\t"+entry+style.terminator+"\n")

		return
	}

	lineStart := lineStartOffset(f.src, closeOffset)

	// The closing parenthesis usually sits alone on its line, and reusing the
	// text before it as indentation keeps the entry aligned with its siblings.
	// When something else shares that line — a comment written just before the
	// paren — copying it would duplicate it, since the insertion is zero-width
	// and leaves the original line intact. Push the entry onto its own line
	// instead and let gofmt align it.
	prefix := string(f.src[lineStart:closeOffset])
	if strings.TrimSpace(prefix) != "" {
		f.splice(closeOffset, closeOffset, "\n\t"+entry+style.terminator+"\n")

		return
	}

	f.splice(lineStart, lineStart, prefix+"\t"+entry+style.terminator+"\n")
}

// splice replaces src[start:end] with text.
func (f *File) splice(start, end int, text string) {
	patched := make([]byte, 0, len(f.src)+len(text))
	patched = append(patched, f.src[:start]...)
	patched = append(patched, text...)
	patched = append(patched, f.src[end:]...)

	f.src = patched
}

func (f *File) appendText(text string) {
	f.splice(len(f.src), len(f.src), text)
}

func (f *File) parse() (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, f.Path, f.src, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", f.Path, err)
	}

	return fset, file, nil
}

// findVarCall locates the call expression initializing a package-level var.
func findVarCall(fset *token.FileSet, file *ast.File, varName string) (*ast.CallExpr, error) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}

		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range value.Names {
				if name.Name != varName || i >= len(value.Values) {
					continue
				}

				call, ok := value.Values[i].(*ast.CallExpr)
				if !ok {
					return nil, fmt.Errorf("%w: %s in %s", ErrNotACall, varName, fset.Position(name.Pos()))
				}

				return call, nil
			}
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrVarNotFound, varName)
}

// findCallInFunc returns the first call to callee inside the named function.
func findCallInFunc(fset *token.FileSet, file *ast.File, funcName, callee string) *ast.CallExpr {
	var found *ast.CallExpr

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName || fn.Body == nil {
			continue
		}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || found != nil {
				return found == nil
			}

			if renderNode(fset, call.Fun) == normalizeExpr(callee) {
				found = call
			}

			return found == nil
		})
	}

	return found
}

// findVarBlock returns the file's first parenthesized package-level var block.
func findVarBlock(file *ast.File) *ast.GenDecl {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if ok && gen.Tok == token.VAR && gen.Lparen.IsValid() {
			return gen
		}
	}

	return nil
}

// lineStartOffset returns the offset of the first byte on the line containing
// offset.
func lineStartOffset(src []byte, offset int) int {
	start := bytes.LastIndexByte(src[:offset], '\n')

	return start + 1
}

// renderNode prints an AST node back to normalized source so an existing
// argument can be compared against the one a generator wants to add.
func renderNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}

	return normalizeExpr(buf.String())
}

// normalizeExpr collapses whitespace so a multi-line argument and its
// single-line equivalent compare equal.
func normalizeExpr(expr string) string {
	return strings.Join(strings.Fields(expr), "")
}
