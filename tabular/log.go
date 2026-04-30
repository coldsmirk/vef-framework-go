package tabular

import "github.com/coldsmirk/vef-framework-go/internal/logx"

// logger is the package-scoped logger used by parser fallbacks and resolver
// warnings. It is package-internal; callers should rely on the framework
// logger directly.
var logger = logx.Named("tabular")
