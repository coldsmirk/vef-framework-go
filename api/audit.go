package api

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
)

const (
	eventTypeAudit = "vef.api.request.audit"
)

// AuditEvent represents an API request audit log event.
type AuditEvent struct {
	event.BaseEvent

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

// AuditEventParams contains parameters for creating an AuditEvent.
type AuditEventParams struct {
	// API identification
	Resource string
	Action   string
	Version  string

	// User identification
	UserID    string
	UserAgent string

	// Request information
	RequestID     string
	RequestIP     string
	RequestParams map[string]any
	RequestMeta   map[string]any

	// Response information
	ResultCode    int
	ResultMessage string
	ResultData    any

	// Performance metrics
	ElapsedTime int64
}

// NewAuditEvent creates a new audit event with the given parameters.
func NewAuditEvent(params AuditEventParams) *AuditEvent {
	return &AuditEvent{
		BaseEvent:     event.NewBaseEvent(eventTypeAudit),
		Resource:      params.Resource,
		Action:        params.Action,
		Version:       params.Version,
		UserID:        params.UserID,
		UserAgent:     params.UserAgent,
		RequestID:     params.RequestID,
		RequestIP:     params.RequestIP,
		RequestParams: params.RequestParams,
		RequestMeta:   params.RequestMeta,
		ResultCode:    params.ResultCode,
		ResultMessage: params.ResultMessage,
		ResultData:    params.ResultData,
		ElapsedTime:   params.ElapsedTime,
	}
}

// SubscribeAuditEvent subscribes to audit events.
// Returns an unsubscribe function that can be called to remove the subscription.
func SubscribeAuditEvent(subscriber event.Subscriber, handler func(context.Context, *AuditEvent)) event.UnsubscribeFunc {
	return subscriber.Subscribe(eventTypeAudit, func(ctx context.Context, evt event.Event) {
		if auditEvt, ok := evt.(*AuditEvent); ok {
			handler(ctx, auditEvt)
		}
	})
}
