package event

import (
	"context"
	"encoding/json"
	"maps"
	"time"

	"github.com/coldsmirk/vef-framework-go/id"
)

// Event represents the base interface for all events in the system.
// All custom events should implement this interface to be compatible with the event bus.
type Event interface {
	// ID returns a unique identifier for this specific event instance.
	ID() string
	// Type returns a unique string identifier for the event type.
	// This is used for routing and filtering events.
	Type() string
	// Source returns the source that generated this event.
	Source() string
	// Time returns when the event occurred.
	Time() time.Time
	// Meta returns the metadata for the event.
	Meta() map[string]string
}

// HandlerFunc represents a function that can handle events.
// The handler receives the event and a context for cancellation/timeout control.
type HandlerFunc func(ctx context.Context, event Event)

// Publisher defines the interface for publishing events to the event bus.
type Publisher interface {
	// Publish sends an event to all registered subscribers asynchronously.
	Publish(event Event)
}

// UnsubscribeFunc is a function that can be called to unsubscribe from an event.
type UnsubscribeFunc func()

// Subscriber defines the interface for subscribing to events.
type Subscriber interface {
	// Subscribe registers a handler for events of a specific type.
	// Returns an unsubscribe function that can be called to remove the subscription.
	Subscribe(eventType string, handler HandlerFunc) UnsubscribeFunc
}

// Bus combines Publisher and Subscriber interfaces along with lifecycle management.
type Bus interface {
	Publisher
	Subscriber

	// Start initializes the event bus and begins processing events.
	Start() error
	// Shutdown gracefully shuts down the event bus.
	Shutdown(ctx context.Context) error
}

// Middleware defines an interface for event processing middleware.
// Middleware can intercept and modify events before they reach handlers.
type Middleware interface {
	// Process is called for each event before it's delivered to handlers.
	// It can modify the event, context, or prevent delivery by returning an error.
	Process(ctx context.Context, event Event, next MiddlewareFunc) error
}

// MiddlewareFunc is a function type for middleware processing.
type MiddlewareFunc func(ctx context.Context, event Event) error

// BaseEvent provides a default implementation of the Event interface.
// Custom events can embed this struct to inherit the base functionality.
// Fields are unexported to prevent modification after creation.
type BaseEvent struct {
	typ    string
	id     string
	source string
	time   time.Time
	meta   map[string]string
}

func (e BaseEvent) Type() string {
	return e.typ
}

func (e BaseEvent) Time() time.Time {
	return e.time
}

func (e BaseEvent) ID() string {
	return e.id
}

func (e BaseEvent) Source() string {
	return e.source
}

func (e BaseEvent) Meta() map[string]string {
	result := make(map[string]string, len(e.meta))
	maps.Copy(result, e.meta)

	return result
}

// BaseEventOption defines an option for configuring BaseEvent creation.
type BaseEventOption func(*BaseEvent)

func WithSource(source string) BaseEventOption {
	return func(e *BaseEvent) {
		e.source = source
	}
}

// WithMeta adds a metadata key-value pair.
func WithMeta(key, value string) BaseEventOption {
	return func(e *BaseEvent) {
		e.meta[key] = value
	}
}

// MarshalJSON implements custom JSON marshaling for BaseEvent.
func (e BaseEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string            `json:"type"`
		ID       string            `json:"id"`
		Source   string            `json:"source"`
		Time     time.Time         `json:"time"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}{
		Type:     e.typ,
		ID:       e.id,
		Source:   e.source,
		Time:     e.time,
		Metadata: e.meta,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for BaseEvent.
func (e *BaseEvent) UnmarshalJSON(data []byte) error {
	var temp struct {
		Type     string            `json:"type"`
		ID       string            `json:"id"`
		Source   string            `json:"source"`
		Time     time.Time         `json:"time"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	e.typ = temp.Type
	e.id = temp.ID
	e.source = temp.Source
	e.time = temp.Time

	e.meta = temp.Metadata
	if e.meta == nil {
		e.meta = make(map[string]string)
	}

	return nil
}

// NewBaseEvent creates a new BaseEvent with the specified type.
// Optional source and metadata can be set using WithSource and WithMeta options.
func NewBaseEvent(eventType string, opts ...BaseEventOption) BaseEvent {
	event := BaseEvent{
		typ:    eventType,
		id:     id.GenerateUUID(),
		source: "",
		time:   time.Now(),
		meta:   make(map[string]string),
	}

	for _, opt := range opts {
		opt(&event)
	}

	return event
}
