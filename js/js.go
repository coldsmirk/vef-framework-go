package js

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

// Type aliases from goja for convenient access.
type (
	Value      = goja.Value
	Object     = goja.Object
	Program    = goja.Program
	AstProgram = ast.Program
)

// Function aliases from goja for script compilation and type checking.
var (
	Compile     = goja.Compile
	MustCompile = goja.MustCompile
	IsNaN       = goja.IsNaN
	IsString    = goja.IsString
	IsBigInt    = goja.IsBigInt
	IsNumber    = goja.IsNumber
	IsInfinity  = goja.IsInfinity
	IsUndefined = goja.IsUndefined
	IsNull      = goja.IsNull
)

// Parse parses JavaScript source code into an AST.
func Parse(name, src string) (*AstProgram, error) {
	return goja.Parse(name, src, parser.WithDisableSourceMaps)
}
