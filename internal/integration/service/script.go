package service

import (
	"github.com/coldsmirk/vef-framework-go/js"
)

// scriptName is the compilation unit name adapter scripts carry in stack
// traces and compile errors.
const scriptName = "adapter"

// CompileScript compiles an adapter script into the executable form the
// invoker runs: the body is wrapped in a function expression so a top-level
// return statement produces the invocation output. The wrapper shifts
// reported line numbers by one.
func CompileScript(script string) (*js.Program, error) {
	return js.Compile(scriptName, "(function () {\n"+script+"\n})()", true)
}
