package event

import (
	"fmt"
	"path"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// router resolves event types to one or more transports. Rules are
// matched in declaration order; the first matching rule wins, allowing
// callers to express priority cleanly. Fan-out is expressed by listing
// multiple transport names in a single rule.
type router struct {
	rules    []compiledRule
	fallback []transport.Transport
}

type compiledRule struct {
	pattern    string
	transports []transport.Transport
}

func buildRouter(cfg *config.EventConfig, registry map[string]transport.Transport) (*router, error) {
	fallback, err := resolveTransports(registry, []string{cfg.EffectiveDefaultTransport()})
	if err != nil {
		return nil, err
	}

	rules := make([]compiledRule, 0, len(cfg.Routing))
	for _, rule := range cfg.Routing {
		// Pre-validate pattern syntax at config time so a typo in
		// path.Match metacharacters (unclosed '[', stray '\\') surfaces
		// during fx.Start rather than silently skipping rules at
		// dispatch time.
		if _, err := path.Match(rule.Pattern, "vef.routing.syntax.probe"); err != nil {
			return nil, fmt.Errorf("event: invalid routing pattern %q: %w", rule.Pattern, err)
		}

		ts, err := resolveTransports(registry, rule.Transports)
		if err != nil {
			return nil, err
		}

		rules = append(rules, compiledRule{
			pattern:    rule.Pattern,
			transports: ts,
		})
	}

	return &router{rules: rules, fallback: fallback}, nil
}

// Resolve returns the transports that should receive the given event
// type. Patterns were syntax-checked during buildRouter, so an error
// from path.Match here would indicate a corrupted rule — log loudly and
// skip the entry instead of returning a misleading match.
func (r *router) Resolve(eventType string) []transport.Transport {
	for _, rule := range r.rules {
		ok, err := path.Match(rule.pattern, eventType)
		if err != nil {
			busLogger.Warnf("router: path.Match pattern=%q event=%q: %v (skipping rule)",
				rule.pattern, eventType, err)

			continue
		}

		if ok {
			return rule.transports
		}
	}

	return r.fallback
}

func resolveTransports(registry map[string]transport.Transport, names []string) ([]transport.Transport, error) {
	out := make([]transport.Transport, 0, len(names))
	for _, name := range names {
		t, ok := registry[name]
		if !ok {
			return nil, &unknownTransportError{name: name}
		}

		out = append(out, t)
	}

	return out, nil
}

type unknownTransportError struct {
	name string
}

func (e *unknownTransportError) Error() string {
	return "event: unknown transport: " + e.name
}
