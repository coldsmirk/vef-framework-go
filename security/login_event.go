package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
)

const eventTypeLogin = "vef.security.login"

// LoginEvent represents a user login attempt — successful or otherwise.
//
// A login may span several requests: the authentication itself, then one
// resolve_challenge per challenge. AuthType is the login mechanism on every
// event of the login, and ChallengeType tells the challenge steps apart.
type LoginEvent struct {
	AuthType      string  `json:"authType"`
	UserID        *string `json:"userId"` // Populated on success
	Username      string  `json:"username"`
	LoginIP       string  `json:"loginIp"`
	UserAgent     string  `json:"userAgent"`
	TraceID       string  `json:"traceId"`
	IsOk          bool    `json:"isOk"`
	FailReason    string  `json:"failReason"` // Populated on failure
	ErrorCode     int     `json:"errorCode"`
	ChallengeType string  `json:"challengeType"` // The challenge being resolved; empty when raised by the authentication
}

// EventType implements event.Event.
func (*LoginEvent) EventType() string { return eventTypeLogin }

// LoginEventParams contains parameters for creating a LoginEvent.
type LoginEventParams struct {
	AuthType      string
	UserID        *string
	Username      string
	LoginIP       string
	UserAgent     string
	TraceID       string
	IsOk          bool
	FailReason    string
	ErrorCode     int
	ChallengeType string
}

// NewLoginEvent creates a new login event with the given parameters.
func NewLoginEvent(params LoginEventParams) *LoginEvent {
	return &LoginEvent{
		AuthType:      params.AuthType,
		UserID:        params.UserID,
		Username:      params.Username,
		LoginIP:       params.LoginIP,
		UserAgent:     params.UserAgent,
		TraceID:       params.TraceID,
		IsOk:          params.IsOk,
		FailReason:    params.FailReason,
		ErrorCode:     params.ErrorCode,
		ChallengeType: params.ChallengeType,
	}
}

// SubscribeLoginEvent registers a typed handler for login events. The
// returned Unsubscribe detaches the subscription.
func SubscribeLoginEvent(
	bus event.Bus,
	handler func(context.Context, *LoginEvent) error,
	opts ...event.SubscribeOption,
) (event.Unsubscribe, error) {
	return event.SubscribeTyped[*LoginEvent](bus, func(ctx context.Context, evt *LoginEvent, _ event.Envelope) error {
		return handler(ctx, evt)
	}, opts...)
}
