package lint

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// UnusedRecv reports a method that names a receiver it never references.
//
// Go lets a receiver be declared by type alone, and that spelling states the
// fact directly: the method does not read what it is called on. Any name
// invites the reader to look for a use that is not there, and the blank
// identifier is no better — it spends a slot saying "ignored" where saying
// nothing is available.
var UnusedRecv = &analysis.Analyzer{
	Name: "unusedrecv",
	Doc:  "report method receivers that go unused and should therefore be unnamed",
	Run:  runUnusedRecv,
}

func runUnusedRecv(pass *analysis.Pass) (any, error) {
	for decl := range funcDecls(pass) {
		reportUnusedRecv(pass, decl)
	}

	return nil, nil
}

func reportUnusedRecv(pass *analysis.Pass, decl *ast.FuncDecl) {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return
	}

	// A receiver list holds exactly one field with at most one name, so the
	// grouping that complicates parameters cannot arise here.
	field := decl.Recv.List[0]
	if len(field.Names) == 0 {
		return
	}

	name := field.Names[0]
	if isReferenced(pass, name, decl.Body) {
		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     name.Pos(),
		End:     name.End(),
		Message: fmt.Sprintf("receiver %q is unused; omit the name and keep only the type", name.Name),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "omit the receiver name",
			TextEdits: []analysis.TextEdit{{Pos: name.Pos(), End: field.Type.Pos()}},
		}},
	})
}
