package security

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/event"
)

// TestNewLoginEvent tests NewLoginEvent constructor.
func TestNewLoginEvent(t *testing.T) {
	t.Run("AllFieldsPopulated", func(t *testing.T) {
		userID := "user123"
		params := LoginEventParams{
			AuthType:   "password",
			UserID:     &userID,
			Username:   "alice",
			LoginIP:    "192.168.1.1",
			UserAgent:  "Mozilla/5.0",
			TraceID:    "trace-abc",
			IsOk:       true,
			FailReason: "",
			ErrorCode:  0,
		}

		evt := NewLoginEvent(params)

		assert.Equal(t, "password", evt.AuthType, "Should preserve AuthType")
		assert.Equal(t, &userID, evt.UserID, "Should preserve UserID")
		assert.Equal(t, "alice", evt.Username, "Should preserve Username")
		assert.Equal(t, "192.168.1.1", evt.LoginIP, "Should preserve LoginIP")
		assert.Equal(t, "Mozilla/5.0", evt.UserAgent, "Should preserve UserAgent")
		assert.Equal(t, "trace-abc", evt.TraceID, "Should preserve TraceID")
		assert.True(t, evt.IsOk, "Should preserve IsOk")
		assert.Empty(t, evt.FailReason, "Should preserve empty FailReason")
		assert.Equal(t, 0, evt.ErrorCode, "Should preserve ErrorCode")
	})

	t.Run("FailedLogin", func(t *testing.T) {
		params := LoginEventParams{
			AuthType:   "jwt",
			UserID:     nil,
			Username:   "bob",
			LoginIP:    "10.0.0.1",
			IsOk:       false,
			FailReason: "invalid password",
			ErrorCode:  401,
		}

		evt := NewLoginEvent(params)

		assert.False(t, evt.IsOk, "Should be false for failed login")
		assert.Nil(t, evt.UserID, "Should be nil for failed login")
		assert.Equal(t, "invalid password", evt.FailReason, "Should preserve FailReason")
		assert.Equal(t, 401, evt.ErrorCode, "Should preserve ErrorCode")
	})

	t.Run("ImplementsEventInterface", func(t *testing.T) {
		evt := NewLoginEvent(LoginEventParams{AuthType: "test"})

		var _ event.Event = evt
		assert.NotEmpty(t, evt.Type(), "Should have event type")
	})
}
