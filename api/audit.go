package api

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
)

const eventTypeAudit = "vef.api.request.audit"

// AuditEvent represents an API request audit log event.
type AuditEvent struct {
	// API identification
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Version  string `json:"version"`

	// User identification
	UserID    string `json:"userId"`
	UserAgent string `json:"userAgent"`

	// Request information
	RequestID     string         `json:"requestId"`
	RequestIP     string         `json:"requestIp"`
	RequestParams map[string]any `json:"requestParams"`
	RequestMeta   map[string]any `json:"requestMeta"`

	// Response information
	ResultCode    int    `json:"resultCode"`
	ResultMessage string `json:"resultMessage"`
	ResultData    any    `json:"resultData"`

	// Performance metrics
	ElapsedTime int64 `json:"elapsedTime"` // Elapsed time in milliseconds
}

// EventType implements event.Event.
func (*AuditEvent) EventType() string { return eventTypeAudit }

// SubscribeAuditEvent registers a typed handler for audit events.
func SubscribeAuditEvent(
	bus event.Bus,
	handler func(context.Context, *AuditEvent) error,
	opts ...event.SubscribeOption,
) (event.Unsubscribe, error) {
	return event.SubscribeTyped[*AuditEvent](bus, func(ctx context.Context, evt *AuditEvent, _ event.Envelope) error {
		return handler(ctx, evt)
	}, opts...)
}
