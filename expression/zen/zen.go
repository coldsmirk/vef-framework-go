package zen

import (
	"github.com/coldsmirk/vef-framework-go/expression"
	internalzen "github.com/coldsmirk/vef-framework-go/internal/expression/zen"
)

// New returns an Engine backed by the gorules Zen expression engine. It is a
// thin facade over internal/expression/zen; importing this package opts the
// application into the CGO dependency (zen-go).
func New() expression.Engine {
	return internalzen.New()
}

// Module enables the Zen-backed expression engine: add it to vef.Run(...) to
// register the engine plus its API handler resolver and mold field transformer.
var Module = internalzen.Module
