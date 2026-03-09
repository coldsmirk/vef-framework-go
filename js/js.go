package js

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

// Type aliases from goja for convenient access.
type (
	Runtime    = goja.Runtime
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

// New creates a new JavaScript runtime with preloaded standard libraries.
//
// The runtime is configured with:
//   - Source maps disabled for better performance
//   - JSON struct tag mapping for Go-JavaScript interop
//   - Global libraries: dayjs, Big, utils, validator
//
// Returns an error if any library fails to load.
//
// WARNING: The returned Runtime is NOT thread-safe. Each goroutine should
// create its own runtime instance.
func New() (*Runtime, error) {
	vm := goja.New()
	vm.SetParserOptions(parser.WithDisableSourceMaps)
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	libraries := []*Program{
		compiledDayJs,
		compiledBigJs,
		compiledUtilsJs,
		compiledValidatorJs,
	}

	for _, lib := range libraries {
		if _, err := vm.RunProgram(lib); err != nil {
			return nil, err
		}
	}

	return vm, nil
}

// Parse parses JavaScript source code into an AST.
func Parse(name, src string) (*AstProgram, error) {
	return goja.Parse(name, src, parser.WithDisableSourceMaps)
}
