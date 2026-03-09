package middleware

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/result"
)

// TestBuildLoggerName tests the buildLoggerName helper function.
func TestBuildLoggerName(t *testing.T) {
	tests := []struct {
		name     string
		a, b, c  string
		expected string
	}{
		{"NormalParts", "resource", "action", "v1", "resource:action@v1"},
		{"EmptyParts", "", "", "", ":@"},
		{"PartialParts", "user", "", "v2", "user:@v2"},
		{"SpecialChars", "sys/user", "create_user", "v1", "sys/user:create_user@v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLoggerName(tt.a, tt.b, tt.c)
			assert.Equal(t, tt.expected, result, "Should build correct logger name")
		})
	}
}

// TestPermTokenFromOperation tests the permTokenFromOperation helper function.
func TestPermTokenFromOperation(t *testing.T) {
	t.Run("WithPermToken", func(t *testing.T) {
		op := &api.Operation{
			Auth: &api.AuthConfig{
				Options: map[string]any{
					shared.AuthOptionPermToken: "sys:user:read",
				},
			},
		}

		token := permTokenFromOperation(op)
		assert.Equal(t, "sys:user:read", token, "Should extract perm token")
	})

	t.Run("WithoutPermToken", func(t *testing.T) {
		op := &api.Operation{
			Auth: &api.AuthConfig{
				Options: map[string]any{},
			},
		}

		token := permTokenFromOperation(op)
		assert.Empty(t, token, "Should return empty when no perm token")
	})

	t.Run("NilOptions", func(t *testing.T) {
		op := &api.Operation{
			Auth: &api.AuthConfig{
				Options: nil,
			},
		}

		token := permTokenFromOperation(op)
		assert.Empty(t, token, "Should return empty when options is nil")
	})

	t.Run("WrongType", func(t *testing.T) {
		op := &api.Operation{
			Auth: &api.AuthConfig{
				Options: map[string]any{
					shared.AuthOptionPermToken: 123,
				},
			},
		}

		token := permTokenFromOperation(op)
		assert.Empty(t, token, "Should return empty when perm token is not a string")
	})
}

// TestExtractErrorInfo tests the extractErrorInfo helper function.
func TestExtractErrorInfo(t *testing.T) {
	t.Run("ResultError", func(t *testing.T) {
		err := result.Err("bad request", result.WithCode(result.ErrCodeBadRequest))
		code, msg := extractErrorInfo(err)

		assert.Equal(t, result.ErrCodeBadRequest, code, "Should extract result error code")
		assert.Equal(t, "bad request", msg, "Should extract result error message")
	})

	t.Run("FiberError", func(t *testing.T) {
		err := fiber.ErrNotFound
		code, msg := extractErrorInfo(err)

		assert.NotZero(t, code, "Should have a mapped code")
		assert.NotEmpty(t, msg, "Should have a message")
	})

	t.Run("GenericError", func(t *testing.T) {
		err := errors.New("something went wrong")
		code, msg := extractErrorInfo(err)

		assert.Equal(t, result.ErrCodeUnknown, code, "Should return unknown code")
		assert.Equal(t, "something went wrong", msg, "Should use error message")
	})
}

// TestNewRateLimit tests NewRateLimit constructor and its methods.
func TestNewRateLimit(t *testing.T) {
	rl := NewRateLimit()

	assert.NotNil(t, rl, "RateLimit should not be nil")
	assert.Equal(t, "ratelimit", rl.Name(), "Name should be 'ratelimit'")
	assert.Equal(t, -70, rl.Order(), "Order should be -70")
}

// TestNewContextual tests NewContextual constructor and its methods.
func TestNewContextual(t *testing.T) {
	ctx := NewContextual(nil)

	assert.NotNil(t, ctx, "Contextual should not be nil")
	assert.Equal(t, "contextual", ctx.Name(), "Name should be 'contextual'")
	assert.Equal(t, -90, ctx.Order(), "Order should be -90")
}

// TestNewAuth tests NewAuth constructor and its methods.
func TestNewAuth(t *testing.T) {
	auth := NewAuth(nil, nil)

	assert.NotNil(t, auth, "Auth should not be nil")
	assert.Equal(t, "auth", auth.Name(), "Name should be 'auth'")
	assert.Equal(t, -100, auth.Order(), "Order should be -100")
}

// TestNewDataPermission tests NewDataPermission constructor and its methods.
func TestNewDataPermission(t *testing.T) {
	dp := NewDataPermission(nil)

	assert.NotNil(t, dp, "DataPermission should not be nil")
	assert.Equal(t, "data_permission", dp.Name(), "Name should be 'data_permission'")
	assert.Equal(t, -80, dp.Order(), "Order should be -80")
}

// TestNewAudit tests NewAudit constructor and its methods.
func TestNewAudit(t *testing.T) {
	audit := NewAudit(nil)

	assert.NotNil(t, audit, "Audit should not be nil")
	assert.Equal(t, "audit", audit.Name(), "Name should be 'audit'")
	assert.Equal(t, -60, audit.Order(), "Order should be -60")
}
