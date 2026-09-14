package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/result"
)

func protectedBodyConfig(encoding config.APIBodyEncoding) config.APIBodyEncodingConfig {
	keySize := 32
	if encoding == config.APIBodyEncodingSM4GCMBase64 {
		keySize = 16
	}

	return config.APIBodyEncodingConfig{
		Enabled:  true,
		Encoding: encoding,
		Key:      base64.StdEncoding.EncodeToString(make([]byte, keySize)),
	}
}

func newProtectedBodyApp(t *testing.T, encoding config.APIBodyEncoding) (*fiber.App, *protectedBodyCodec) {
	t.Helper()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			status := fiber.StatusInternalServerError
			code := result.ErrCodeUnknown
			message := "unknown"

			if resultErr, ok := result.AsErr(err); ok {
				status = resultErr.Status
				code = resultErr.Code
				message = resultErr.Message
			} else if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
				status = fiberErr.Code
				message = fiberErr.Message
			}

			return ctx.Status(status).JSON(result.Result{Code: code, Message: message})
		},
	})

	bodyConfig := protectedBodyConfig(encoding)
	middleware, err := NewBodyEncodingMiddleware(
		&config.AppConfig{BodyLimit: "32mib"},
		&config.APIConfig{BodyEncoding: bodyConfig},
	)
	require.NoError(t, err, "protected body middleware should initialize")
	middleware.Apply(app)

	codec, err := newProtectedBodyCodec(&bodyConfig)
	require.NoError(t, err, "test codec should initialize")

	app.Post("/api", func(ctx fiber.Ctx) error {
		ctx.Set("X-Seen-Encoding", ctx.Get(api.HeaderXBodyEncoding))

		return ctx.Type("json").Send(bytes.Clone(ctx.Body()))
	})
	app.Get("/api/error", func(fiber.Ctx) error {
		return api.ErrInvalidRequestParams
	})
	app.Post("/other", func(ctx fiber.Ctx) error {
		return ctx.Type("json").Send(bytes.Clone(ctx.Body()))
	})

	return app, codec
}

func sendProtectedBody(
	t *testing.T,
	app *fiber.App,
	method string,
	path string,
	body []byte,
	encoding string,
) *http.Response {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	if encoding != "" {
		req.Header.Set(api.HeaderXBodyEncoding, encoding)
	}

	response, err := app.Test(req)
	require.NoError(t, err, "fiber test request should not fail")

	return response
}

func readProtectedResponse(t *testing.T, response *http.Response, codec *protectedBodyCodec) []byte {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err, "reading response body should not fail")

	decoded, err := codec.decode(body, 1<<20)
	require.NoError(t, err, "response body should decrypt")

	return decoded
}

func TestProtectedBodyCodec(t *testing.T) {
	raw := []byte(`{"code":0,"message":"ok","data":{"value":"café"}}`)

	for _, encoding := range []config.APIBodyEncoding{
		config.APIBodyEncodingAESGCMBase64,
		config.APIBodyEncodingSM4GCMBase64,
	} {
		t.Run(string(encoding), func(t *testing.T) {
			cfg := protectedBodyConfig(encoding)
			codec, err := newProtectedBodyCodec(&cfg)
			require.NoError(t, err, "codec should initialize")

			encoded, err := codec.encode(raw)
			require.NoError(t, err, "body should encrypt")
			assert.NotEqual(t, raw, encoded, "wire body should not contain plaintext JSON")

			decoded, err := codec.decode(encoded, 1<<20)
			require.NoError(t, err, "body should decrypt")
			assert.Equal(t, raw, decoded, "decrypted body should match the original JSON")

			encoded[len(encoded)/2] ^= 1
			_, err = codec.decode(encoded, 1<<20)
			assert.ErrorIs(t, err, api.ErrBodyDecodeFailed, "authenticated encryption should reject tampering")
		})
	}
}

func TestProtectedBodyMiddleware(t *testing.T) {
	raw := []byte(`{"resource":"system/user","action":"create","params":{"name":"alice"}}`)

	t.Run("BidirectionalAES", func(t *testing.T) {
		app, codec := newProtectedBodyApp(t, config.APIBodyEncodingAESGCMBase64)
		encoded, err := codec.encode(raw)
		require.NoError(t, err, "request body should encrypt")

		response := sendProtectedBody(t, app, http.MethodPost, "/api", encoded, string(codec.encoding))

		assert.Equal(t, http.StatusOK, response.StatusCode, "encrypted request should dispatch")
		assert.Equal(t, string(codec.encoding), response.Header.Get(api.HeaderXBodyEncoding),
			"encrypted response should advertise its encoding")
		assert.Empty(t, response.Header.Get("X-Seen-Encoding"), "request marker should be stripped downstream")
		assert.Equal(t, raw, readProtectedResponse(t, response, codec), "handler response should round-trip encrypted")
	})

	t.Run("MissingEncoding", func(t *testing.T) {
		app, codec := newProtectedBodyApp(t, config.APIBodyEncodingAESGCMBase64)

		response := sendProtectedBody(t, app, http.MethodPost, "/api", raw, "")

		assert.Equal(t, http.StatusBadRequest, response.StatusCode, "plaintext JSON should be rejected")
		assert.Equal(t, string(codec.encoding), response.Header.Get(api.HeaderXBodyEncoding),
			"rejection should still use the configured response encoding")
		assert.NotContains(t, string(readProtectedResponse(t, response, codec)), string(raw),
			"decrypted rejection should be an error envelope, not the request")
	})

	t.Run("MismatchedEncoding", func(t *testing.T) {
		app, codec := newProtectedBodyApp(t, config.APIBodyEncodingAESGCMBase64)

		response := sendProtectedBody(t, app, http.MethodPost, "/api", base64Bytes(raw), bodyEncodingBase64)

		assert.Equal(t, http.StatusBadRequest, response.StatusCode, "a weaker encoding should not downgrade protected mode")
		assert.Equal(t, string(codec.encoding), response.Header.Get(api.HeaderXBodyEncoding),
			"rejection should be protected")
		_ = readProtectedResponse(t, response, codec)
	})

	t.Run("DownstreamError", func(t *testing.T) {
		app, codec := newProtectedBodyApp(t, config.APIBodyEncodingAESGCMBase64)

		response := sendProtectedBody(t, app, http.MethodGet, "/api/error", nil, "")

		assert.Equal(t, http.StatusBadRequest, response.StatusCode, "downstream transport error should retain its status")
		assert.Equal(t, string(codec.encoding), response.Header.Get(api.HeaderXBodyEncoding),
			"downstream error envelope should be protected")
		assert.Contains(t, string(readProtectedResponse(t, response, codec)), `"code":1400`,
			"decrypted body should contain the downstream error envelope")
	})

	t.Run("NonAPIPathUntouched", func(t *testing.T) {
		app, _ := newProtectedBodyApp(t, config.APIBodyEncodingAESGCMBase64)

		response := sendProtectedBody(t, app, http.MethodPost, "/other", raw, "")
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err, "reading response body should not fail")

		assert.Equal(t, raw, body, "non-API body should remain plaintext")
		assert.Empty(t, response.Header.Get(api.HeaderXBodyEncoding), "non-API response should have no marker")
	})
}
