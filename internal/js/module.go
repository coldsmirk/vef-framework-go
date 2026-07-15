package js

import (
	"time"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	ilogx "github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jscache"
	"github.com/coldsmirk/vef-framework-go/js/jsconsole"
	"github.com/coldsmirk/vef-framework-go/js/jscrypto"
	"github.com/coldsmirk/vef-framework-go/js/jsevents"
	"github.com/coldsmirk/vef-framework-go/js/jshttp"
	"github.com/coldsmirk/vef-framework-go/js/jssql"
	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// scriptHTTPTimeout is the default per-request timeout of the built-in http
// library. It is the only restriction applied by default; everything else
// (body size, host allowlist, private-network guard) is left open for the
// application to tighten via an override.
const scriptHTTPTimeout = 30 * time.Second

// logger is the framework logger of the scripting engine; loggers follow the
// repo-wide package-level convention rather than DI (there is no logx.Logger
// in the FX graph).
var logger = ilogx.Named("js")

// Module wires the JavaScript scripting engine: the framework capability
// libraries are seeded at safe defaults across two tiers — the safe utilities
// (console, crypto, cache) are always installed, while the side-effecting
// capabilities (events, http, sql) stay opt-in per runtime — and any
// application library (group "vef:js:libs", registered via vef.ProvideJSLib)
// overrides a same-named default (keeping its tier) or adds a new opt-in
// library.
var Module = fx.Module(
	"vef:js",
	fx.Provide(
		fx.Annotate(
			func(bus event.Bus, db orm.DB, kind config.DBKind, appLibs []js.Lib) (*js.Engine, error) {
				return NewEngine(logger, bus, db, kind, appLibs)
			},
			fx.ParamTags(``, ``, ``, `group:"vef:js:libs"`),
		),
	),
)

// NewEngine builds the application-wide engine, seeding the always-on utility
// libraries and the opt-in capability libraries at safe defaults, then
// overlaying the application-provided libraries. logger receives the script
// console output.
func NewEngine(logger logx.Logger, bus event.Bus, db orm.DB, kind config.DBKind, appLibs []js.Lib) (*js.Engine, error) {
	alwaysOn := []js.Lib{
		jsconsole.New(logger),
		jscrypto.New(),
		jscache.New(cache.NewMemory[any](), jscache.WithKeyPrefix("js:")),
	}

	optIn := []js.Lib{
		jsevents.New(bus),
		jshttp.New(jshttp.WithTimeout(scriptHTTPTimeout)),
		jssql.New(db, kind),
	}

	base, catalog := overlay(alwaysOn, optIn, appLibs)

	return js.NewEngine(js.WithBaseLibs(base...), js.WithLibs(catalog...))
}

// overlay applies the application libraries onto the framework defaults. An
// application library whose name matches a default replaces it in place,
// preserving that default's tier; any other application library joins the
// opt-in catalog. A nil entry is passed through so NewEngine reports it rather
// than silently dropping it.
func overlay(alwaysOn, optIn, appLibs []js.Lib) (base, catalog []js.Lib) {
	base = append([]js.Lib(nil), alwaysOn...)
	catalog = append([]js.Lib(nil), optIn...)

	baseIndex, catalogIndex := indexByName(base), indexByName(catalog)

	for _, lib := range appLibs {
		if lib == nil {
			catalog = append(catalog, lib)

			continue
		}

		name := lib.Name()

		if i, ok := baseIndex[name]; ok {
			base[i] = lib

			continue
		}

		if i, ok := catalogIndex[name]; ok {
			catalog[i] = lib

			continue
		}

		catalogIndex[name] = len(catalog)
		catalog = append(catalog, lib)
	}

	return base, catalog
}

// indexByName maps each library's name to its position in libs.
func indexByName(libs []js.Lib) map[string]int {
	index := make(map[string]int, len(libs))
	for i, lib := range libs {
		index[lib.Name()] = i
	}

	return index
}
