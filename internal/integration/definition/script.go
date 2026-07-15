package definition

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

// CompileEnvelopeRequestScript compiles a system-level request envelope
// script: the body sees the outgoing request as `request` and a top-level
// return statement yields the request to put on the wire. Unlike adapter
// scripts the wrapper is not invoked — it evaluates to a function the http
// library calls once per outbound request.
func CompileEnvelopeRequestScript(script string) (*js.Program, error) {
	return compileEnvelopeScript("request", script)
}

// CompileEnvelopeResponseScript compiles a system-level response envelope
// script: the body sees the completed response as `response` and a top-level
// return statement yields what the adapter's call returns.
func CompileEnvelopeResponseScript(script string) (*js.Program, error) {
	return compileEnvelopeScript("response", script)
}

// compileEnvelopeScript wraps an envelope body in a function expression over
// its single named parameter. The wrapper shifts reported line numbers by
// one, matching CompileScript.
func compileEnvelopeScript(param, script string) (*js.Program, error) {
	return js.Compile("envelope:"+param, "(function ("+param+") {\n"+script+"\n})", true)
}
