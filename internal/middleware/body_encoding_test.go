package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/result"
)

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	require.NoError(t, err, "gzip write should not fail")
	require.NoError(t, writer.Close(), "gzip close should not fail")

	return buf.Bytes()
}

func base64Bytes(data []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(data))
}

func newBodyEncodingApp(t *testing.T, bodyLimit string) *fiber.App {
	t.Helper()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			if e, ok := result.AsErr(err); ok {
				return c.SendStatus(e.Status)
			}

			return c.SendStatus(fiber.StatusInternalServerError)
		},
	})

	NewBodyEncodingMiddleware(&config.AppConfig{BodyLimit: bodyLimit}).Apply(app)

	echo := func(c fiber.Ctx) error {
		// Reflect the marker header so the test can prove it was stripped.
		c.Set("X-Seen-Encoding", c.Get(api.HeaderXBodyEncoding))

		return c.Send(bytes.Clone(c.Body()))
	}
	app.Post("/api", echo)
	app.Post("/other", echo)

	return app
}

func sendBodyEncoded(t *testing.T, app *fiber.App, path, encoding string, body []byte) *http.Response {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	if encoding != "" {
		req.Header.Set(api.HeaderXBodyEncoding, encoding)
	}

	resp, err := app.Test(req)
	require.NoError(t, err, "fiber test request should not fail")

	return resp
}

func TestDecodeBody(t *testing.T) {
	const limit = 1 << 20

	raw := []byte(`{"resource":"integration/adapter","action":"save"}`)

	tests := []struct {
		name     string
		input    []byte
		encoding string
		limit    int
		want     []byte
		wantErr  error
	}{
		{
			name:     "Base64",
			input:    base64Bytes(raw),
			encoding: bodyEncodingBase64,
			limit:    limit,
			want:     raw,
		},
		{
			name:     "Base64ToleratesSurroundingWhitespace",
			input:    []byte("  " + string(base64Bytes(raw)) + "\n"),
			encoding: bodyEncodingBase64,
			limit:    limit,
			want:     raw,
		},
		{
			name:     "GzipBase64",
			input:    base64Bytes(gzipBytes(t, raw)),
			encoding: bodyEncodingGzipBase64,
			limit:    limit,
			want:     raw,
		},
		{
			name:     "UnsupportedEncoding",
			input:    base64Bytes(raw),
			encoding: "zstd",
			limit:    limit,
			wantErr:  api.ErrUnsupportedBodyEncoding,
		},
		{
			name:     "MalformedBase64",
			input:    []byte("not@@base64!!"),
			encoding: bodyEncodingBase64,
			limit:    limit,
			wantErr:  api.ErrBodyDecodeFailed,
		},
		{
			name:     "GzipBase64WithNonGzipPayload",
			input:    base64Bytes([]byte("plain, not gzip")),
			encoding: bodyEncodingGzipBase64,
			limit:    limit,
			wantErr:  api.ErrBodyDecodeFailed,
		},
		{
			name:     "GzipBase64ExceedsLimit",
			input:    base64Bytes(gzipBytes(t, []byte(strings.Repeat("a", 4096)))),
			encoding: bodyEncodingGzipBase64,
			limit:    64,
			wantErr:  api.ErrBodyTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBody(tt.input, tt.encoding, tt.limit)

			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err, "should return the expected error sentinel")
				assert.Nil(t, got, "decoded bytes should be nil on error")

				return
			}

			require.NoError(t, err, "decode should succeed")
			assert.Equal(t, tt.want, got, "decoded bytes should match the original")
		})
	}
}

func TestBodyEncodingMiddleware(t *testing.T) {
	raw := []byte(`{"resource":"integration/adapter","action":"save","params":{"script":"return input"}}`)

	t.Run("PassthroughWithoutHeader", func(t *testing.T) {
		app := newBodyEncodingApp(t, "32mib")

		resp := sendBodyEncoded(t, app, "/api", "", raw)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "reading response body should not fail")

		assert.Equal(t, http.StatusOK, resp.StatusCode, "a request without the marker passes untouched")
		assert.Equal(t, raw, body, "body must be forwarded verbatim")
	})

	t.Run("Base64", func(t *testing.T) {
		app := newBodyEncodingApp(t, "32mib")

		resp := sendBodyEncoded(t, app, "/api", bodyEncodingBase64, base64Bytes(raw))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "reading response body should not fail")

		assert.Equal(t, http.StatusOK, resp.StatusCode, "base64 body decodes and dispatches")
		assert.Equal(t, raw, body, "handler must see the decoded JSON")
		assert.Empty(t, resp.Header.Get("X-Seen-Encoding"), "the marker header must be stripped before the handler")
	})

	t.Run("GzipBase64", func(t *testing.T) {
		app := newBodyEncodingApp(t, "32mib")

		resp := sendBodyEncoded(t, app, "/api", bodyEncodingGzipBase64, base64Bytes(gzipBytes(t, raw)))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "reading response body should not fail")

		assert.Equal(t, http.StatusOK, resp.StatusCode, "gzip+base64 body decodes and dispatches")
		assert.Equal(t, raw, body, "handler must see the decoded JSON")
	})

	t.Run("UnsupportedEncoding", func(t *testing.T) {
		app := newBodyEncodingApp(t, "32mib")

		resp := sendBodyEncoded(t, app, "/api", "gzip", base64Bytes(raw))

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "an unknown encoding is a client error")
	})

	t.Run("MalformedBase64", func(t *testing.T) {
		app := newBodyEncodingApp(t, "32mib")

		resp := sendBodyEncoded(t, app, "/api", bodyEncodingBase64, []byte("@@@ not base64 @@@"))

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a malformed body is a client error")
	})

	t.Run("DecodedBodyExceedsLimit", func(t *testing.T) {
		app := newBodyEncodingApp(t, "64b")

		resp := sendBodyEncoded(t, app, "/api", bodyEncodingGzipBase64, base64Bytes(gzipBytes(t, []byte(strings.Repeat("a", 4096)))))

		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "an over-limit decoded body is rejected")
	})

	t.Run("NonAPIPathUntouched", func(t *testing.T) {
		app := newBodyEncodingApp(t, "32mib")
		encoded := base64Bytes(raw)

		resp := sendBodyEncoded(t, app, "/other", bodyEncodingBase64, encoded)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "reading response body should not fail")

		assert.Equal(t, http.StatusOK, resp.StatusCode, "non-API surfaces own their body formats")
		assert.Equal(t, encoded, body, "the body must not be decoded off the API surface")
		assert.Equal(t, bodyEncodingBase64, resp.Header.Get("X-Seen-Encoding"), "the marker must survive off the API surface")
	})
}
