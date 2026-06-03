package zen

import "github.com/coldsmirk/vef-framework-go/internal/expression"

// Module enables the Zen-backed expression engine: add it to vef.Run(...) to
// register the engine plus its API handler resolver and mold field transformer.
// Importing this package opts the application into the CGO dependency (zen-go),
// so applications that do not need expressions stay pure Go.
var Module = expression.Module
