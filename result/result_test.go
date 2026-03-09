package result

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOk tests the Ok function.
func TestOk(t *testing.T) {
	t.Run("NoArguments", func(t *testing.T) {
		result := Ok()

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.NotEmpty(t, result.Message, "Should have default success message")
		assert.Nil(t, result.Data, "Should have nil data")
	})

	t.Run("WithDataOnly", func(t *testing.T) {
		data := map[string]any{"id": "123", "name": "test"}

		result := Ok(data)

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.NotEmpty(t, result.Message, "Should have default success message")
		assert.Equal(t, data, result.Data, "Should have provided data")
	})

	t.Run("WithMessageOptionOnly", func(t *testing.T) {
		result := Ok(WithMessage("custom success message"))

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.Equal(t, "custom success message", result.Message, "Should use custom message")
		assert.Nil(t, result.Data, "Should have nil data")
	})

	t.Run("WithDataAndMessage", func(t *testing.T) {
		data := map[string]any{"count": 42}

		result := Ok(data, WithMessage("operation completed"))

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.Equal(t, "operation completed", result.Message, "Should use custom message")
		assert.Equal(t, data, result.Data, "Should have provided data")
	})

	t.Run("WithStringData", func(t *testing.T) {
		result := Ok("simple string data")

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.NotEmpty(t, result.Message, "Should have default success message")
		assert.Equal(t, "simple string data", result.Data, "Should have string data")
	})

	t.Run("WithIntData", func(t *testing.T) {
		result := Ok(123)

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.NotEmpty(t, result.Message, "Should have default success message")
		assert.Equal(t, 123, result.Data, "Should have int data")
	})

	t.Run("WithStructData", func(t *testing.T) {
		type User struct {
			ID   string
			Name string
		}

		user := User{ID: "u123", Name: "John"}

		result := Ok(user)

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.NotEmpty(t, result.Message, "Should have default success message")
		assert.Equal(t, user, result.Data, "Should have struct data")
	})

	t.Run("WithSliceData", func(t *testing.T) {
		data := []string{"item1", "item2", "item3"}

		result := Ok(data)

		assert.Equal(t, OkCode, result.Code, "Should use success code")
		assert.NotEmpty(t, result.Message, "Should have default success message")
		assert.Equal(t, data, result.Data, "Should have slice data")
	})
}

// TestOkPanicCases tests panic scenarios in Ok function.
func TestOkPanicCases(t *testing.T) {
	t.Run("MultipleDataArguments", func(t *testing.T) {
		assert.Panics(t, func() {
			Ok("data1", "data2")
		}, "Should panic when multiple data arguments provided")
	})

	t.Run("DataAfterOption", func(t *testing.T) {
		assert.Panics(t, func() {
			Ok(WithMessage("message"), "data after option")
		}, "Should panic when data comes after option")
	})

	t.Run("MultipleDataWithOption", func(t *testing.T) {
		assert.Panics(t, func() {
			Ok("data1", "data2", WithMessage("message"))
		}, "Should panic when multiple data with option")
	})
}

// TestResultResponse tests the Response method.
func TestResultResponse(t *testing.T) {
	t.Run("DefaultStatus", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(ctx fiber.Ctx) error {
			return Ok(map[string]any{"success": true}).Response(ctx)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Request should succeed")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusOK, resp.StatusCode, "Should return 200 status")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Should read response body")

		var result Result

		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Should unmarshal JSON response")

		assert.Equal(t, OkCode, result.Code, "Response should have success code")
		assert.NotEmpty(t, result.Message, "Response should have message")
		assert.NotNil(t, result.Data, "Response should have data")
	})

	t.Run("CustomStatus", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(ctx fiber.Ctx) error {
			return Ok(map[string]any{"created": true}).Response(ctx, fiber.StatusCreated)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Request should succeed")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusCreated, resp.StatusCode, "Should return 201 status")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Should read response body")

		var result Result

		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Should unmarshal JSON response")

		assert.Equal(t, OkCode, result.Code, "Response should have success code")
	})

	t.Run("ErrorWithCustomStatus", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(ctx fiber.Ctx) error {
			err := Err("not found", WithCode(ErrCodeNotFound), WithStatus(fiber.StatusNotFound))

			return Result{
				Code:    err.Code,
				Message: err.Message,
			}.Response(ctx, fiber.StatusNotFound)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Request should succeed")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode, "Should return 404 status")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Should read response body")

		var result Result

		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Should unmarshal JSON response")

		assert.Equal(t, ErrCodeNotFound, result.Code, "Response should have not found code")
	})

	t.Run("WithNilData", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(ctx fiber.Ctx) error {
			return Ok().Response(ctx)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Request should succeed")

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Should read response body")

		var result Result

		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Should unmarshal JSON response")

		assert.Nil(t, result.Data, "Response data should be nil")
	})

	t.Run("WithComplexData", func(t *testing.T) {
		type UserData struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		app := fiber.New()
		app.Post("/test", func(ctx fiber.Ctx) error {
			userData := UserData{
				ID:    "u123",
				Name:  "John Doe",
				Email: "john@example.com",
			}

			return Ok(userData).Response(ctx)
		})

		req := httptest.NewRequestWithContext(context.Background(), "POST", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Request should succeed")

		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Should read response body")

		var result struct {
			Code    int      `json:"code"`
			Message string   `json:"message"`
			Data    UserData `json:"data"`
		}

		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Should unmarshal JSON response")

		assert.Equal(t, "u123", result.Data.ID, "Should have correct user ID")
		assert.Equal(t, "John Doe", result.Data.Name, "Should have correct user name")
		assert.Equal(t, "john@example.com", result.Data.Email, "Should have correct user email")
	})
}

// TestResultIsOk tests the IsOk method.
func TestResultIsOk(t *testing.T) {
	t.Run("SuccessResult", func(t *testing.T) {
		result := Ok()

		assert.True(t, result.IsOk(), "Success result should return true")
	})

	t.Run("SuccessResultWithData", func(t *testing.T) {
		result := Ok(map[string]any{"test": "data"})

		assert.True(t, result.IsOk(), "Success result with data should return true")
	})

	t.Run("ErrorResult", func(t *testing.T) {
		err := Err("error message")
		result := Result{
			Code:    err.Code,
			Message: err.Message,
		}

		assert.False(t, result.IsOk(), "Error result should return false")
	})

	t.Run("CustomCodeResult", func(t *testing.T) {
		result := Result{
			Code:    1000,
			Message: "custom code",
		}

		assert.False(t, result.IsOk(), "Result with non-success code should return false")
	})
}

// TestWithMessage tests the WithMessage option.
func TestWithMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"SimpleMessage", "operation successful", "operation successful"},
		{"EmptyMessage", "", ""},
		{"LongMessage", "This is a very long message describing the operation result in detail", "This is a very long message describing the operation result in detail"},
		{"MessageWithSpecialChars", "成功：操作已完成！", "成功：操作已完成！"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Ok(WithMessage(tt.message))

			assert.Equal(t, tt.want, result.Message, "Should set correct message")
		})
	}
}

// TestResultStructure tests the Result struct fields.
func TestResultStructure(t *testing.T) {
	result := Result{
		Code:    OkCode,
		Message: "test message",
		Data:    map[string]any{"key": "value"},
	}

	assert.Equal(t, OkCode, result.Code, "Code field should be accessible")
	assert.Equal(t, "test message", result.Message, "Message field should be accessible")
	assert.NotNil(t, result.Data, "Data field should be accessible")
}

// TestResultJSONSerialization tests JSON serialization of Result.
func TestResultJSONSerialization(t *testing.T) {
	t.Run("WithData", func(t *testing.T) {
		result := Ok(map[string]any{"id": "123", "name": "test"})

		jsonBytes, err := json.Marshal(result)
		require.NoError(t, err, "Should marshal to JSON")

		var unmarshaled Result

		err = json.Unmarshal(jsonBytes, &unmarshaled)
		require.NoError(t, err, "Should unmarshal from JSON")

		assert.Equal(t, result.Code, unmarshaled.Code, "Code should be preserved")
		assert.Equal(t, result.Message, unmarshaled.Message, "Message should be preserved")
		assert.NotNil(t, unmarshaled.Data, "Data should be preserved")
	})

	t.Run("WithNilData", func(t *testing.T) {
		result := Ok()

		jsonBytes, err := json.Marshal(result)
		require.NoError(t, err, "Should marshal to JSON")

		var unmarshaled Result

		err = json.Unmarshal(jsonBytes, &unmarshaled)
		require.NoError(t, err, "Should unmarshal from JSON")

		assert.Equal(t, result.Code, unmarshaled.Code, "Code should be preserved")
		assert.Nil(t, unmarshaled.Data, "Nil data should be preserved")
	})

	t.Run("JSONFieldNames", func(t *testing.T) {
		result := Ok(map[string]any{"test": "data"})

		jsonBytes, err := json.Marshal(result)
		require.NoError(t, err, "Should marshal to JSON")

		var jsonMap map[string]any

		err = json.Unmarshal(jsonBytes, &jsonMap)
		require.NoError(t, err, "Should unmarshal to map")

		assert.Contains(t, jsonMap, "code", "JSON should have 'code' field")
		assert.Contains(t, jsonMap, "message", "JSON should have 'message' field")
		assert.Contains(t, jsonMap, "data", "JSON should have 'data' field")
	})
}

// TestOkWithVariousDataTypes tests Ok with different data types.
func TestOkWithVariousDataTypes(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{"NilData", nil},
		{"BoolData", true},
		{"IntData", 42},
		{"Int64Data", int64(9999999999)},
		{"Float64Data", 3.14159},
		{"StringData", "hello world"},
		{"SliceData", []int{1, 2, 3, 4, 5}},
		{"MapData", map[string]any{"key1": "value1", "key2": 123}},
		{"StructData", struct {
			ID   string
			Name string
		}{ID: "123", Name: "Test"}},
		{"PointerData", &struct{ Value int }{Value: 100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Ok(tt.data)

			assert.Equal(t, OkCode, result.Code, "Should have success code")
			assert.NotEmpty(t, result.Message, "Should have message")
			assert.Equal(t, tt.data, result.Data, "Should preserve data")
		})
	}
}

// TestOkMessageCustomization tests message customization.
func TestOkMessageCustomization(t *testing.T) {
	t.Run("DefaultMessage", func(t *testing.T) {
		result := Ok(map[string]any{"data": "value"})

		assert.NotEmpty(t, result.Message, "Should have default message")
		assert.Equal(t, OkCode, result.Code, "Should have success code")
	})

	t.Run("CustomMessage", func(t *testing.T) {
		result := Ok(
			map[string]any{"data": "value"},
			WithMessage("Custom operation completed successfully"),
		)

		assert.Equal(t, "Custom operation completed successfully", result.Message, "Should have custom message")
		assert.Equal(t, OkCode, result.Code, "Should have success code")
	})

	t.Run("CustomMessageWithoutData", func(t *testing.T) {
		result := Ok(WithMessage("Success without data"))

		assert.Equal(t, "Success without data", result.Message, "Should have custom message")
		assert.Nil(t, result.Data, "Should have nil data")
	})
}

// TestResultResponseIntegration tests integration between Ok/Err and Response.
func TestResultResponseIntegration(t *testing.T) {
	app := fiber.New()

	app.Post("/success", func(ctx fiber.Ctx) error {
		return Ok(map[string]any{"status": "success"}).Response(ctx)
	})

	app.Post("/success-custom", func(ctx fiber.Ctx) error {
		return Ok(
			map[string]any{"status": "created"},
			WithMessage("Resource created"),
		).Response(ctx, fiber.StatusCreated)
	})

	app.Post("/error", func(ctx fiber.Ctx) error {
		err := Err("operation failed", WithCode(ErrCodeBadRequest))

		return Result{
			Code:    err.Code,
			Message: err.Message,
		}.Response(ctx)
	})

	t.Run("SuccessEndpoint", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "POST", "/success", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusOK, resp.StatusCode, "Should return 200")

		var result Result

		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, OkCode, result.Code, "Should have success code")
		assert.NotNil(t, result.Data, "Should have data")
	})

	t.Run("SuccessCustomEndpoint", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "POST", "/success-custom", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusCreated, resp.StatusCode, "Should return 201")

		var result Result

		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, OkCode, result.Code, "Should have success code")
		assert.Equal(t, "Resource created", result.Message, "Should have custom message")
	})

	t.Run("ErrorEndpoint", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), "POST", "/error", nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "Should not return error")

		defer resp.Body.Close()

		assert.Equal(t, fiber.StatusOK, resp.StatusCode, "Should return 200 for business error")

		var result Result

		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err, "Should not return error")

		assert.Equal(t, ErrCodeBadRequest, result.Code, "Should have error code")
		assert.Equal(t, "operation failed", result.Message, "Should have error message")
	})
}
