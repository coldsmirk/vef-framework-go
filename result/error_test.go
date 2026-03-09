package result

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErr tests the Err function.
func TestErr(t *testing.T) {
	t.Run("NoArguments", func(t *testing.T) {
		err := Err()

		assert.Equal(t, ErrCodeDefault, err.Code, "Should use default error code")
		assert.NotEmpty(t, err.Message, "Should have default message")
		assert.Equal(t, fiber.StatusOK, err.Status, "Should use default status 200")
	})

	t.Run("MessageOnly", func(t *testing.T) {
		err := Err("custom error message")

		assert.Equal(t, ErrCodeDefault, err.Code, "Should use default error code")
		assert.Equal(t, "custom error message", err.Message, "Should use provided message")
		assert.Equal(t, fiber.StatusOK, err.Status, "Should use default status 200")
	})

	t.Run("MessageWithCode", func(t *testing.T) {
		err := Err("not found", WithCode(ErrCodeNotFound))

		assert.Equal(t, ErrCodeNotFound, err.Code, "Should use custom code")
		assert.Equal(t, "not found", err.Message, "Should use provided message")
		assert.Equal(t, fiber.StatusOK, err.Status, "Should use default status 200")
	})

	t.Run("MessageWithStatus", func(t *testing.T) {
		err := Err("unauthorized", WithStatus(fiber.StatusUnauthorized))

		assert.Equal(t, ErrCodeDefault, err.Code, "Should use default error code")
		assert.Equal(t, "unauthorized", err.Message, "Should use provided message")
		assert.Equal(t, fiber.StatusUnauthorized, err.Status, "Should use custom status")
	})

	t.Run("MessageWithCodeAndStatus", func(t *testing.T) {
		err := Err(
			"access denied",
			WithCode(ErrCodeAccessDenied),
			WithStatus(fiber.StatusForbidden),
		)

		assert.Equal(t, ErrCodeAccessDenied, err.Code, "Should use custom code")
		assert.Equal(t, "access denied", err.Message, "Should use provided message")
		assert.Equal(t, fiber.StatusForbidden, err.Status, "Should use custom status")
	})

	t.Run("OnlyWithCode", func(t *testing.T) {
		err := Err(WithCode(ErrCodeBadRequest))

		assert.Equal(t, ErrCodeBadRequest, err.Code, "Should use custom code")
		assert.NotEmpty(t, err.Message, "Should have default message")
		assert.Equal(t, fiber.StatusOK, err.Status, "Should use default status 200")
	})

	t.Run("OnlyWithStatus", func(t *testing.T) {
		err := Err(WithStatus(fiber.StatusNotFound))

		assert.Equal(t, ErrCodeDefault, err.Code, "Should use default error code")
		assert.NotEmpty(t, err.Message, "Should have default message")
		assert.Equal(t, fiber.StatusNotFound, err.Status, "Should use custom status")
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		err := Err(
			WithCode(ErrCodeTooManyRequests),
			WithStatus(fiber.StatusTooManyRequests),
		)

		assert.Equal(t, ErrCodeTooManyRequests, err.Code, "Should use custom code")
		assert.NotEmpty(t, err.Message, "Should have default message")
		assert.Equal(t, fiber.StatusTooManyRequests, err.Status, "Should use custom status")
	})
}

// TestErrPanicCases tests panic scenarios in Err function.
func TestErrPanicCases(t *testing.T) {
	t.Run("MessageNotFirstArgument", func(t *testing.T) {
		assert.Panics(t, func() {
			Err(WithCode(ErrCodeBadRequest), "message after option")
		}, "Should panic when message is not the first argument")
	})

	t.Run("InvalidArgumentType", func(t *testing.T) {
		assert.Panics(t, func() {
			Err(123) // Invalid type
		}, "Should panic on invalid argument type")
	})

	t.Run("InvalidArgumentTypeAfterMessage", func(t *testing.T) {
		assert.Panics(t, func() {
			Err("message", 456) // Invalid type after message
		}, "Should panic on invalid argument type after message")
	})
}

// TestErrf tests the Errf function.
func TestErrf(t *testing.T) {
	t.Run("BasicFormatting", func(t *testing.T) {
		err := Errf("user %s not found", "john")

		assert.Equal(t, ErrCodeDefault, err.Code, "Should use default error code")
		assert.Equal(t, "user john not found", err.Message, "Should format message correctly")
		assert.Equal(t, fiber.StatusOK, err.Status, "Should use default status 200")
	})

	t.Run("FormattingWithCode", func(t *testing.T) {
		err := Errf("user %s not found", "jane", WithCode(ErrCodeNotFound))

		assert.Equal(t, ErrCodeNotFound, err.Code, "Should use custom code")
		assert.Equal(t, "user jane not found", err.Message, "Should format message correctly")
		assert.Equal(t, fiber.StatusOK, err.Status, "Should use default status 200")
	})

	t.Run("FormattingWithCodeAndStatus", func(t *testing.T) {
		err := Errf(
			"operation %s failed for user %d",
			"delete", 123,
			WithCode(ErrCodeAccessDenied),
			WithStatus(fiber.StatusForbidden),
		)

		assert.Equal(t, ErrCodeAccessDenied, err.Code, "Should use custom code")
		assert.Equal(t, "operation delete failed for user 123", err.Message, "Should format message correctly")
		assert.Equal(t, fiber.StatusForbidden, err.Status, "Should use custom status")
	})

	t.Run("MultipleFormatArgs", func(t *testing.T) {
		err := Errf(
			"failed to %s record %d in table %s",
			"update", 456, "users",
		)

		assert.Equal(t, "failed to update record 456 in table users", err.Message, "Should format multiple args correctly")
	})

	t.Run("MultipleFormatArgsWithOptions", func(t *testing.T) {
		err := Errf(
			"error code %d: %s",
			404, "resource not found",
			WithCode(ErrCodeNotFound),
			WithStatus(fiber.StatusNotFound),
		)

		assert.Equal(t, ErrCodeNotFound, err.Code, "Should use custom code")
		assert.Equal(t, "error code 404: resource not found", err.Message, "Should format message correctly")
		assert.Equal(t, fiber.StatusNotFound, err.Status, "Should use custom status")
	})
}

// TestErrfPanicCases tests panic scenarios in Errf function.
func TestErrfPanicCases(t *testing.T) {
	t.Run("NoFormatArguments", func(t *testing.T) {
		assert.Panics(t, func() {
			Errf("static message without format")
		}, "Should panic when no format arguments provided")
	})

	t.Run("OptionBeforeFormatArgs", func(t *testing.T) {
		assert.Panics(t, func() {
			Errf("user %s not found", WithCode(ErrCodeNotFound), "john")
		}, "Should panic when option comes before format arguments")
	})

	t.Run("MixedFormatArgsAndOptions", func(t *testing.T) {
		assert.Panics(t, func() {
			Errf("error %d: %s", 500, WithCode(ErrCodeUnknown), "server error")
		}, "Should panic when format args and options are mixed")
	})
}

// TestAsErr tests the AsErr function.
func TestAsErr(t *testing.T) {
	t.Run("ValidError", func(t *testing.T) {
		original := Err("test error", WithCode(ErrCodeBadRequest))

		result, ok := AsErr(original)

		require.True(t, ok, "Should successfully convert Error type")
		assert.Equal(t, ErrCodeBadRequest, result.Code, "Should preserve error code")
		assert.Equal(t, "test error", result.Message, "Should preserve message")
	})

	t.Run("WrappedError", func(t *testing.T) {
		original := Err("inner error", WithCode(ErrCodeNotFound))
		wrapped := fmt.Errorf("wrapped: %w", original)

		result, ok := AsErr(wrapped)

		require.True(t, ok, "Should successfully unwrap and convert Error type")
		assert.Equal(t, ErrCodeNotFound, result.Code, "Should preserve error code")
		assert.Equal(t, "inner error", result.Message, "Should preserve message")
	})

	t.Run("StandardError", func(t *testing.T) {
		standardErr := errors.New("standard error")

		result, ok := AsErr(standardErr)

		assert.False(t, ok, "Should fail to convert standard error")
		assert.Equal(t, Error{}, result, "Should return empty Error")
	})

	t.Run("NilError", func(t *testing.T) {
		var err error

		result, ok := AsErr(err)

		assert.False(t, ok, "Should fail to convert nil error")
		assert.Equal(t, Error{}, result, "Should return empty Error")
	})
}

// TestIsRecordNotFound tests the IsRecordNotFound function.
func TestIsRecordNotFound(t *testing.T) {
	t.Run("RecordNotFoundError", func(t *testing.T) {
		result := IsRecordNotFound(ErrRecordNotFound)

		assert.True(t, result, "Should identify ErrRecordNotFound")
	})

	t.Run("WrappedRecordNotFoundError", func(t *testing.T) {
		wrapped := fmt.Errorf("wrapped: %w", ErrRecordNotFound)

		result := IsRecordNotFound(wrapped)

		assert.True(t, result, "Should identify wrapped ErrRecordNotFound")
	})

	t.Run("OtherError", func(t *testing.T) {
		otherErr := Err("other error")

		result := IsRecordNotFound(otherErr)

		assert.False(t, result, "Should not identify other errors as record not found")
	})

	t.Run("StandardError", func(t *testing.T) {
		standardErr := errors.New("standard error")

		result := IsRecordNotFound(standardErr)

		assert.False(t, result, "Should not identify standard error as record not found")
	})

	t.Run("NilError", func(t *testing.T) {
		var err error

		result := IsRecordNotFound(err)

		assert.False(t, result, "Should not identify nil error as record not found")
	})
}

// TestErrorError tests the Error method of Error type.
func TestErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  Error
		want string
	}{
		{
			name: "SimpleMessage",
			err:  Err("simple error"),
			want: "simple error",
		},
		{
			name: "FormattedMessage",
			err:  Errf("error code %d", 404),
			want: "error code 404",
		},
		{
			name: "MessageWithOptions",
			err:  Err("access denied", WithCode(ErrCodeAccessDenied)),
			want: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()

			assert.Equal(t, tt.want, result, "Error() should return the message")
		})
	}
}

// TestWithCode tests the WithCode option.
func TestWithCode(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"DefaultCode", ErrCodeDefault, ErrCodeDefault},
		{"BadRequest", ErrCodeBadRequest, ErrCodeBadRequest},
		{"NotFound", ErrCodeNotFound, ErrCodeNotFound},
		{"Unauthorized", ErrCodeUnauthenticated, ErrCodeUnauthenticated},
		{"CustomCode", 9999, 9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Err(WithCode(tt.code))

			assert.Equal(t, tt.want, err.Code, "Should set correct error code")
		})
	}
}

// TestWithStatus tests the WithStatus option.
func TestWithStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   int
	}{
		{"StatusOK", fiber.StatusOK, fiber.StatusOK},
		{"StatusBadRequest", fiber.StatusBadRequest, fiber.StatusBadRequest},
		{"StatusUnauthorized", fiber.StatusUnauthorized, fiber.StatusUnauthorized},
		{"StatusForbidden", fiber.StatusForbidden, fiber.StatusForbidden},
		{"StatusNotFound", fiber.StatusNotFound, fiber.StatusNotFound},
		{"StatusInternalServerError", fiber.StatusInternalServerError, fiber.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Err(WithStatus(tt.status))

			assert.Equal(t, tt.want, err.Status, "Should set correct HTTP status")
		})
	}
}

// TestErrorStructure tests the Error struct fields.
func TestErrorStructure(t *testing.T) {
	err := Error{
		Code:    ErrCodeBadRequest,
		Message: "test message",
		Status:  fiber.StatusBadRequest,
	}

	assert.Equal(t, ErrCodeBadRequest, err.Code, "Code field should be accessible")
	assert.Equal(t, "test message", err.Message, "Message field should be accessible")
	assert.Equal(t, fiber.StatusBadRequest, err.Status, "Status field should be accessible")
}

// TestPredefinedErrors tests that predefined errors are properly configured.
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    Error
		code   int
		status int
	}{
		{"ErrRecordNotFound", ErrRecordNotFound, ErrCodeRecordNotFound, fiber.StatusOK},
		{"ErrRecordAlreadyExists", ErrRecordAlreadyExists, ErrCodeRecordAlreadyExists, fiber.StatusOK},
		{"ErrTokenExpired", ErrTokenExpired, ErrCodeTokenExpired, fiber.StatusUnauthorized},
		{"ErrTokenInvalid", ErrTokenInvalid, ErrCodeTokenInvalid, fiber.StatusUnauthorized},
		{"ErrTokenNotValidYet", ErrTokenNotValidYet, ErrCodeTokenNotValidYet, fiber.StatusUnauthorized},
		{"ErrTokenInvalidIssuer", ErrTokenInvalidIssuer, ErrCodeTokenInvalidIssuer, fiber.StatusUnauthorized},
		{"ErrTokenInvalidAudience", ErrTokenInvalidAudience, ErrCodeTokenInvalidAudience, fiber.StatusUnauthorized},
		{"ErrTokenMissingSubject", ErrTokenMissingSubject, ErrCodeTokenMissingSubject, fiber.StatusUnauthorized},
		{"ErrTokenMissingTokenType", ErrTokenMissingTokenType, ErrCodeTokenMissingTokenType, fiber.StatusUnauthorized},
		{"ErrAppIDRequired", ErrAppIDRequired, ErrCodeAppIDRequired, fiber.StatusUnauthorized},
		{"ErrTimestampRequired", ErrTimestampRequired, ErrCodeTimestampRequired, fiber.StatusUnauthorized},
		{"ErrSignatureRequired", ErrSignatureRequired, ErrCodeSignatureRequired, fiber.StatusUnauthorized},
		{"ErrTimestampInvalid", ErrTimestampInvalid, ErrCodeTimestampInvalid, fiber.StatusUnauthorized},
		{"ErrSignatureExpired", ErrSignatureExpired, ErrCodeSignatureExpired, fiber.StatusUnauthorized},
		{"ErrSignatureInvalid", ErrSignatureInvalid, ErrCodeSignatureInvalid, fiber.StatusUnauthorized},
		{"ErrExternalAppNotFound", ErrExternalAppNotFound, ErrCodeExternalAppNotFound, fiber.StatusUnauthorized},
		{"ErrExternalAppDisabled", ErrExternalAppDisabled, ErrCodeExternalAppDisabled, fiber.StatusUnauthorized},
		{"ErrIPNotAllowed", ErrIPNotAllowed, ErrCodeIPNotAllowed, fiber.StatusUnauthorized},
		{"ErrUnauthenticated", ErrUnauthenticated, ErrCodeUnauthenticated, fiber.StatusUnauthorized},
		{"ErrAccessDenied", ErrAccessDenied, ErrCodeAccessDenied, fiber.StatusForbidden},
		{"ErrUnknown", ErrUnknown, ErrCodeUnknown, fiber.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code, "Predefined error should have correct code")
			assert.Equal(t, tt.status, tt.err.Status, "Predefined error should have correct status")
			assert.NotEmpty(t, tt.err.Message, "Predefined error should have a message")
		})
	}
}
