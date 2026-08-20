package lint

import (
	"go/ast"
	"go/token"
	"iter"

	"golang.org/x/tools/go/analysis"
)

// funcDecls yields every function and method declaration that has a body,
// paired with the file it was declared in.
//
// A declaration without one is an assembly or linkname stub whose parameters no
// Go code can reference, so "unused" says nothing about it. Generated files are
// skipped because their author is a program that would overwrite any fix on its
// next run. Function literals are deliberately out of scope: a callback's
// signature is dictated by whoever calls it, and naming a parameter it does not
// read is often how the closure documents which slot it is filling.
//
// The file comes along because every fix these rules propose deletes a span of
// source, and a span may hold a comment (see holdsComment).
func funcDecls(pass *analysis.Pass) iter.Seq2[*ast.File, *ast.FuncDecl] {
	return func(yield func(*ast.File, *ast.FuncDecl) bool) {
		for _, file := range pass.Files {
			if ast.IsGenerated(file) {
				continue
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}

				if !yield(file, fn) {
					return
				}
			}
		}
	}
}

// holdsComment reports whether any comment in file falls inside [pos, end).
//
// Both rules fix by replacing a span that runs from an identifier to its type,
// and a comment written between the two lives inside that span. Rewriting it
// away would delete something a person wrote to explain the very declaration
// being changed, and the comment has nowhere to survive: once the name is gone
// there is nothing left for it to annotate. So the span is checked first and
// the fix withheld — the diagnostic still stands, the edit is simply left to a
// human who can decide where the prose belongs.
func holdsComment(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.End() > pos && group.Pos() < end {
			return true
		}
	}

	return false
}

// isReferenced reports whether the object declared by name is read or written
// anywhere in body.
//
// Resolution runs over type information rather than identifier text, so an
// inner declaration that reuses the name does not disguise an unused parameter
// as a used one. The blank identifier declares no object and therefore can
// never be referenced.
func isReferenced(pass *analysis.Pass, name *ast.Ident, body *ast.BlockStmt) bool {
	if name.Name == "_" {
		return false
	}

	object := pass.TypesInfo.Defs[name]
	if object == nil {
		return false
	}

	referenced := false

	ast.Inspect(body, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok && pass.TypesInfo.Uses[ident] == object {
			referenced = true
		}

		return !referenced
	})

	return referenced
}
