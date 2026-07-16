package js

import (
	"fmt"

	"github.com/coldsmirk/go-collections"
)

// Engine holds an immutable, validated set of libraries and stamps out
// runtimes. It is safe for concurrent use; one engine typically serves the
// whole application, with NewRuntime called wherever a script executes.
//
// The engine distinguishes two library tiers: the baseline (the standard
// libraries plus any always-on libraries registered via WithBaseLibs,
// installed into every runtime) and the catalog (registered via WithLibs,
// installed only into runtimes that activate them through EnableLibs).
type Engine struct {
	baseline []Lib
	catalog  map[string]Lib
}

// engineConfig collects the settings resolved from EngineOptions.
type engineConfig struct {
	baseLibs    []Lib
	libs        []Lib
	skipStdLibs bool
}

// EngineOption customizes engine construction.
type EngineOption func(*engineConfig)

// WithBaseLibs registers always-on libraries: they install into every runtime
// alongside the standard libraries, without an EnableLibs opt-in. Reserve this
// for safe, ubiquitous utilities; gate capabilities with side effects behind
// the catalog (WithLibs) instead.
func WithBaseLibs(libs ...Lib) EngineOption {
	return func(c *engineConfig) {
		c.baseLibs = append(c.baseLibs, libs...)
	}
}

// WithLibs registers libraries into the engine catalog. Catalog libraries are
// opt-in: each runtime activates the ones it needs via EnableLibs.
func WithLibs(libs ...Lib) EngineOption {
	return func(c *engineConfig) {
		c.libs = append(c.libs, libs...)
	}
}

// WithoutStdLibs builds a bare engine whose runtimes start without the
// built-in standard library bundle (BigNumber, dayjs, fxp, radashi, z, and
// the URL / URLSearchParams polyfills). Libraries added through WithBaseLibs
// are unaffected.
func WithoutStdLibs() EngineOption {
	return func(c *engineConfig) {
		c.skipStdLibs = true
	}
}

// NewEngine builds an engine from the given options, validating the library
// set eagerly: a nil library, an empty name, or a name collision (across the
// standard, always-on, and catalog libraries) fails construction.
func NewEngine(opts ...EngineOption) (*Engine, error) {
	var cfg engineConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	engine := &Engine{catalog: make(map[string]Lib, len(cfg.libs))}
	if !cfg.skipStdLibs {
		engine.baseline = append(engine.baseline, stdLibs...)
	}

	engine.baseline = append(engine.baseline, cfg.baseLibs...)

	names := collections.NewHashSet[string]()
	for _, lib := range engine.baseline {
		name, err := validatedName(lib)
		if err != nil {
			return nil, err
		}

		if names.Contains(name) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateLib, name)
		}

		names.Add(name)
	}

	for _, lib := range cfg.libs {
		name, err := validatedName(lib)
		if err != nil {
			return nil, err
		}

		if names.Contains(name) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateLib, name)
		}

		names.Add(name)
		engine.catalog[name] = lib
	}

	return engine, nil
}

// validatedName returns lib's name, rejecting a nil library or an empty name.
func validatedName(lib Lib) (string, error) {
	if lib == nil {
		return "", fmt.Errorf("%w: nil lib", ErrInvalidLib)
	}

	name := lib.Name()
	if name == "" {
		return "", fmt.Errorf("%w: empty lib name", ErrInvalidLib)
	}

	return name, nil
}

// NewRuntime creates a fresh Runtime carrying the engine baseline plus the
// catalog libraries activated through EnableLibs. The returned Runtime is not
// safe for concurrent use; create one per goroutine and discard it after use.
func (e *Engine) NewRuntime(opts ...RuntimeOption) (*Runtime, error) {
	var cfg runtimeConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	rt := newRuntime(cfg)

	for _, lib := range e.baseline {
		if err := installLib(rt, lib); err != nil {
			return nil, err
		}
	}

	enabled := collections.NewHashSetWithCapacity[string](len(cfg.enabledLibs))
	for _, name := range cfg.enabledLibs {
		if enabled.Contains(name) {
			continue
		}

		enabled.Add(name)

		lib, ok := e.catalog[name]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrLibNotFound, name)
		}

		if err := installLib(rt, lib); err != nil {
			return nil, err
		}
	}

	return rt, nil
}

// installLib installs lib into rt, contextualizing failures with the lib name.
func installLib(rt *Runtime, lib Lib) error {
	if err := lib.Install(rt); err != nil {
		return fmt.Errorf("js: install lib %s: %w", lib.Name(), err)
	}

	return nil
}
