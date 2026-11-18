package calendar

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
)

// ChangedEvent invalidates the listed years, or the whole cache if empty.
type ChangedEvent struct {
	Years []int `json:"years"`
}

// EventType identifies calendar invalidations.
func (*ChangedEvent) EventType() string { return "vef.calendar.changed" }

// PublishChangedEvent notifies consumers after the source calendar changes.
func PublishChangedEvent(ctx context.Context, bus event.Bus, years ...int) error {
	return bus.Publish(ctx, &ChangedEvent{Years: years})
}
