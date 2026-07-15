package gateway

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/result"
)

// renderToResponse runs renderReply inside a fiber handler and captures the
// rendered response plus the error the render returned.
func renderToResponse(t *testing.T, reply any) (*http.Response, error) {
	t.Helper()

	var renderErr error

	app := fiber.New()
	app.Get("/reply", func(ctx fiber.Ctx) error {
		renderErr = renderReply(ctx, reply)

		return nil
	})

	resp, err := app.Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/reply", nil))
	require.NoError(t, err, "Test request should execute")

	return resp, renderErr
}

// TestRenderReply tests the script-reply rendering contract.
func TestRenderReply(t *testing.T) {
	t.Run("NilRendersEmptySuccess", func(t *testing.T) {
		resp, renderErr := renderToResponse(t, nil)
		require.NoError(t, renderErr, "A nil reply should render")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "A nil reply should render an empty 200")
	})

	t.Run("PlainMapWithEnvelopeLikeKeysStaysVerbatim", func(t *testing.T) {
		resp, renderErr := renderToResponse(t, map[string]any{"status": "ok", "body": "ack"})
		require.NoError(t, renderErr, "A plain reply should render")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Response body should read")

		assert.Equal(t, http.StatusOK, resp.StatusCode, "A plain reply should ride HTTP 200")
		assert.JSONEq(t, `{"status":"ok","body":"ack"}`, string(body),
			"A business payload carrying status/body fields must never be misread as an envelope")
	})

	t.Run("EnvelopeControlsStatusHeadersAndBody", func(t *testing.T) {
		resp, renderErr := renderToResponse(t, map[string]any{
			"$response": map[string]any{
				"status":  float64(201),
				"headers": map[string]any{"X-Ack": "1"},
				"body":    "<Ack>0</Ack>",
			},
		})
		require.NoError(t, renderErr, "An envelope reply should render")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Response body should read")

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "The envelope should control the status")
		assert.Equal(t, "1", resp.Header.Get("X-Ack"), "The envelope should control the headers")
		assert.Equal(t, "<Ack>0</Ack>", string(body), "A string body should pass through verbatim")
		assert.Contains(t, resp.Header.Get(fiber.HeaderContentType), "text/plain",
			"A string body should default to text/plain")
	})

	t.Run("EnvelopeObjectBodyRendersAsJSON", func(t *testing.T) {
		resp, renderErr := renderToResponse(t, map[string]any{
			"$response": map[string]any{"body": map[string]any{"received": true}},
		})
		require.NoError(t, renderErr, "An envelope reply should render")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Response body should read")

		assert.JSONEq(t, `{"received":true}`, string(body), "An object body should encode as JSON")
		assert.Contains(t, resp.Header.Get(fiber.HeaderContentType), "application/json",
			"An object body should default to application/json")
	})

	t.Run("EnvelopeStatusOutOfRangeFallsBack", func(t *testing.T) {
		resp, renderErr := renderToResponse(t, map[string]any{
			"$response": map[string]any{"status": "ok", "body": "x"},
		})
		require.NoError(t, renderErr, "An envelope reply should render")
		assert.Equal(t, http.StatusOK, resp.StatusCode, "An unusable status should fall back to 200")
	})

	t.Run("EnvelopeMustBeObject", func(t *testing.T) {
		_, renderErr := renderToResponse(t, map[string]any{"$response": "nope"})
		require.ErrorIs(t, renderErr, integration.ErrScriptFailed(""),
			"A non-object envelope should fail as a script fault")
	})
}

// TestCallerStatus tests the pipeline-error to HTTP-status mapping.
func TestCallerStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "ContractNotFound", err: integration.ErrContractNotFound, want: http.StatusNotFound},
		{name: "ContractDisabled", err: integration.ErrContractDisabled, want: http.StatusNotFound},
		{name: "AdapterNotFound", err: integration.ErrAdapterNotFound, want: http.StatusNotFound},
		{name: "AdapterDisabled", err: integration.ErrAdapterDisabled, want: http.StatusNotFound},
		{name: "InputInvalid", err: integration.ErrInputInvalid("bad"), want: http.StatusBadRequest},
		{name: "ScriptFailure", err: integration.ErrScriptFailed("boom"), want: http.StatusInternalServerError},
		{name: "AuthKeepsItsOwnStatus", err: integration.ErrInboundAuthFailed, want: http.StatusUnauthorized},
		{name: "HandlerMissingKeepsItsOwnStatus", err: integration.ErrInboundHandlerMissing, want: http.StatusNotImplemented},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped, ok := errors.AsType[result.Error](callerStatus(tt.err))
			require.True(t, ok, "The mapped error should stay a result error")
			assert.Equal(t, tt.want, mapped.Status, "The status should be caller-actionable")
		})
	}

	t.Run("NonResultErrorPassesThrough", func(t *testing.T) {
		plain := errors.New("boom")
		assert.Equal(t, plain, callerStatus(plain), "A non-result error should pass through unchanged")
	})
}

// TestFlattenHeaders tests the header normalization of the HTTP envelope.
func TestFlattenHeaders(t *testing.T) {
	flat := flattenHeaders(map[string][]string{
		"X-API-Key": {"k1"},
		"Accept":    {"application/json", "text/xml"},
	})

	assert.Equal(t, "k1", flat["x-api-key"], "Header names should be lowercased")
	assert.Equal(t, "application/json, text/xml", flat["accept"], "Multi-value headers should join with a comma")
}
