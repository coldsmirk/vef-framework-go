package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
)

const eventTypeLogin = "vef.security.login"

// LoginEvent represents a user login event.
type LoginEvent struct {
	event.BaseEvent

	AuthType   string  `json:"authType"`
	UserID     *string `json:"userId"` // Populated on success
	Username   string  `json:"username"`
	LoginIP    string  `json:"loginIp"`
	UserAgent  string  `json:"userAgent"`
	TraceID    string  `json:"traceId"`
	IsOk       bool    `json:"isOk"`
	FailReason string  `json:"failReason"` // Populated on failure
	ErrorCode  int     `json:"errorCode"`
}

// LoginEventParams contains parameters for creating a LoginEvent.
type LoginEventParams struct {
	AuthType   string
	UserID     *string
	Username   string
	LoginIP    string
	UserAgent  string
	TraceID    string
	IsOk       bool
	FailReason string
	ErrorCode  int
}

// NewLoginEvent creates a new login event with the given parameters.
func NewLoginEvent(params LoginEventParams) *LoginEvent {
	return &LoginEvent{
		BaseEvent:  event.NewBaseEvent(eventTypeLogin),
		AuthType:   params.AuthType,
		UserID:     params.UserID,
		Username:   params.Username,
		LoginIP:    params.LoginIP,
		UserAgent:  params.UserAgent,
		TraceID:    params.TraceID,
		IsOk:       params.IsOk,
		FailReason: params.FailReason,
		ErrorCode:  params.ErrorCode,
	}
}

// SubscribeLoginEvent subscribes to login events.
// Returns an unsubscribe function that can be called to remove the subscription.
func SubscribeLoginEvent(subscriber event.Subscriber, handler func(context.Context, *LoginEvent)) event.UnsubscribeFunc {
	return subscriber.Subscribe(eventTypeLogin, func(ctx context.Context, evt event.Event) {
		if loginEvt, ok := evt.(*LoginEvent); ok {
			handler(ctx, loginEvt)
		}
	})
}
