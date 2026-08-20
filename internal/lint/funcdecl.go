package lint

import (
	"go/ast"
	"iter"

	"golang.org/x/tools/go/analysis"
)

// funcDecls yields every function and method declaration that has a body.
//
// A declaration without one is an assembly or linkname stub whose parameters no
// Go code can reference, so "unused" says nothing about it. Generated files are
// skipped because their author is a program that would overwrite any fix on its
// next run. Function literals are deliberately out of scope: a callback's
// signature is dictated by whoever calls it, and naming a parameter it does not
// read is often how the closure documents which slot it is filling.
func funcDecls(pass *analysis.Pass) iter.Seq[*ast.FuncDecl] {
	return func(yield func(*ast.FuncDecl) bool) {
		for _, file := range pass.Files {
			if ast.IsGenerated(file) {
				continue
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}

				if !yield(fn) {
					return
				}
			}
		}
	}
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
