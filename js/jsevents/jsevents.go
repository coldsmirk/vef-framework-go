package jsevents

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/js"
)

// Name is the library identifier and the global binding installed into the
// runtime.
const Name = "events"

// lib exposes domain-event publishing as the global "events" object:
//
//	events.publish('report.generated', { reportId: id })
//
// The payload is JSON-encoded and published as an event.RawPayload, the
// bus's canonical type-plus-JSON carrier, so Go subscribers decode it with
// event.SubscribeTyped as usual. Publishing is deliberately the only verb —
// subscriptions are long-lived and belong to the host, not to a per-execution
// runtime. Scripts run outside any transaction, so delivery follows the
// configured route's plain (non event.WithTx) semantics.
type lib struct {
	bus          event.Bus
	allowedTypes []string
}

// New builds the events library over bus. See WithAllowedTypes for
// restricting the publishable type namespace.
func New(bus event.Bus, opts ...Option) js.Lib {
	var cfg libConfig

	for _, opt := range opts {
		opt(&cfg)
	}

	return &lib{bus: bus, allowedTypes: cfg.allowedTypes}
}

func (*lib) Name() string {
	return Name
}

func (l *lib) Install(rt *js.Runtime) error {
	return rt.Set(Name, map[string]any{
		"publish": func(eventType string, payload any) error {
			if eventType == "" {
				return ErrEmptyEventType
			}

			if !l.typeAllowed(eventType) {
				return fmt.Errorf("%w: %s", ErrEventTypeNotAllowed, eventType)
			}

			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}

			return l.bus.Publish(rt.Context(), event.RawPayload{Type: eventType, Body: body})
		},
	})
}

// typeAllowed reports whether eventType passes the configured allowlist; an
// absent allowlist allows everything.
func (l *lib) typeAllowed(eventType string) bool {
	if l.allowedTypes == nil {
		return true
	}

	for _, pattern := range l.allowedTypes {
		if matchType(pattern, eventType) {
			return true
		}
	}

	return false
}

// matchType matches eventType against one allowlist pattern: "*" matches
// everything, "prefix.*" matches descendants of prefix, anything else matches
// exactly.
func matchType(pattern, eventType string) bool {
	if pattern == "*" {
		return true
	}

	if prefix, ok := strings.CutSuffix(pattern, ".*"); ok {
		return strings.HasPrefix(eventType, prefix+".")
	}

	return pattern == eventType
}
