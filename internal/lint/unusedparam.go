package lint

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// UnusedParam reports parameters a function never references, and says how to
// spell them by whether any parameter is used at all.
//
// Go requires a parameter list to be either wholly named or wholly unnamed, so
// the two cases differ. When every parameter goes unused the whole list can
// drop its names and stand as types alone, which is both shorter and more
// honest than a row of blank identifiers. When some parameters are used the
// list must stay named, and the blank identifier is then the only way to mark
// the rest as ignored.
var UnusedParam = &analysis.Analyzer{
	Name: "unusedparam",
	Doc:  "report unused parameters that should be unnamed, or blank when the list keeps its names",
	Run:  runUnusedParam,
}

func runUnusedParam(pass *analysis.Pass) (any, error) {
	for file, decl := range funcDecls(pass) {
		if err := reportUnusedParams(pass, file, decl); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func reportUnusedParams(pass *analysis.Pass, file *ast.File, decl *ast.FuncDecl) error {
	params := decl.Type.Params
	if params == nil {
		return nil
	}

	var named, unused []*ast.Ident

	for _, field := range params.List {
		for _, name := range field.Names {
			named = append(named, name)

			if !isReferenced(pass, name, decl.Body) {
				unused = append(unused, name)
			}
		}
	}

	switch {
	case len(named) == 0, len(unused) == 0:
		return nil
	case len(unused) == len(named):
		return reportWholeListUnused(pass, file, params)
	default:
		reportBlankableParams(pass, unused)

		return nil
	}
}

// reportWholeListUnused proposes dropping every name from a parameter list no
// part of which is read. The proposal is withheld when a comment sits between a
// name and its type, since the rewrite would delete it.
func reportWholeListUnused(pass *analysis.Pass, file *ast.File, params *ast.FieldList) error {
	edits := make([]analysis.TextEdit, 0, len(params.List))
	annotated := false

	for _, field := range params.List {
		if len(field.Names) == 0 {
			continue
		}

		if holdsComment(file, field.Names[0].Pos(), field.Type.End()) {
			annotated = true

			break
		}

		// A field groups several names onto one type, so "_, _ string"
		// declares two parameters. Deleting the names alone would take every
		// parameter of the group but the last down with them and silently
		// change the signature's arity, so the field is rewritten as one copy
		// of its type per name it declared.
		text, err := nodeText(pass.Fset, field.Type)
		if err != nil {
			return err
		}

		edits = append(edits, analysis.TextEdit{
			Pos:     field.Names[0].Pos(),
			End:     field.Type.End(),
			NewText: []byte(strings.Repeat(text+", ", len(field.Names)-1) + text),
		})
	}

	diagnostic := analysis.Diagnostic{
		Pos:     params.Pos(),
		End:     params.End(),
		Message: "every parameter is unused; omit the names and keep only the types",
	}

	if annotated {
		diagnostic.Message += " (no fix offered: a comment sits inside the parameter list)"
	} else {
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message:   "omit every parameter name",
			TextEdits: edits,
		}}
	}

	pass.Report(diagnostic)

	return nil
}

// reportBlankableParams proposes the blank identifier for the unused
// parameters of a list that keeps its names because others are used.
func reportBlankableParams(pass *analysis.Pass, unused []*ast.Ident) {
	for _, name := range unused {
		if name.Name == "_" {
			continue
		}

		pass.Report(analysis.Diagnostic{
			Pos:     name.Pos(),
			End:     name.End(),
			Message: fmt.Sprintf("parameter %q is unused; rename it to _", name.Name),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message:   "rename to _",
				TextEdits: []analysis.TextEdit{{Pos: name.Pos(), End: name.End(), NewText: []byte("_")}},
			}},
		})
	}
}

// nodeText renders an AST node back to the source it stands for.
func nodeText(fset *token.FileSet, node ast.Node) (string, error) {
	var buf bytes.Buffer

	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "", err
	}

	return buf.String(), nil
}
