package monitor

import "github.com/coldsmirk/vef-framework-go/event"

// EventStreamsInfo is the monitor payload for cross-process event stream
// state. Enabled reports whether a stream inspector is available (the
// redis_stream transport is on); when false, Streams is empty. Operators
// watch Groups for orphans — a group with growing lag and only idle
// consumers is a subscriber that was removed or renamed without
// decommissioning its consumer group.
type EventStreamsInfo struct {
	Enabled bool               `json:"enabled"`
	Streams []event.StreamInfo `json:"streams"`
}
