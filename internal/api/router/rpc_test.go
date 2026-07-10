package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
)

// parseRPCRequest runs (*RPC).parseRequest inside a Fiber handler against the
// given request body and content type, returning the parsed api.Request.
func parseRPCRequest(t *testing.T, contentType, body string) *api.Request {
	t.Helper()

	var (
		captured *api.Request
		parseErr error
	)

	app := fiber.New()
	app.Post("/api", func(ctx fiber.Ctx) error {
		r := &RPC{}
		captured, parseErr = r.parseRequest(ctx)

		return ctx.SendStatus(fiber.StatusOK)
	})

	httpReq := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader(body))
	httpReq.Header.Set(fiber.HeaderContentType, contentType)

	resp, err := app.Test(httpReq)
	require.NoError(t, err, "fiber app.Test should not error")
	require.NotNil(t, resp, "response should not be nil")
	require.NoError(t, parseErr, "parseRequest should succeed")
	require.NotNil(t, captured, "parsed request should not be nil")

	return captured
}

// TestParseRequestNumericFidelity proves the RPC entry hop keeps request
// numbers digit-exact: integers beyond 2^53 arrive as json.Number, survive
// into json.RawMessage captures and int64 fields, while plain numeric params
// still bind as before.
func TestParseRequestNumericFidelity(t *testing.T) {
	type echoParams struct {
		Payload json.RawMessage `json:"payload"`
		Count   int64           `json:"count"`
		Ratio   float64         `json:"ratio"`
	}

	t.Run("JSONBody", func(t *testing.T) {
		body := `{"resource":"demo","action":"echo","version":"v1",` +
			`"params":{"payload":{"n":9007199254740993},"count":9007199254740993,"ratio":0.5},` +
			`"meta":{"page":2}}`

		req := parseRPCRequest(t, fiber.MIMEApplicationJSON, body)

		assert.Equal(t, "demo", req.Resource, "resource should bind")
		assert.Equal(t, json.Number("9007199254740993"), req.Params["count"], "raw params should hold json.Number")
		assert.Equal(t, json.Number("2"), req.Meta["page"], "raw meta should hold json.Number")

		var out echoParams

		require.NoError(t, req.Params.Decode(&out), "Decode should succeed")
		assert.Equal(t, `{"n":9007199254740993}`, string(out.Payload), "RawMessage param must keep the exact digits")
		assert.Equal(t, int64(9007199254740993), out.Count, "int64 param must keep exact digits")
		assert.Equal(t, 0.5, out.Ratio, "plain numeric param should bind as before")
	})

	t.Run("FormBody", func(t *testing.T) {
		form := url.Values{}
		form.Set("resource", "demo")
		form.Set("action", "echo")
		form.Set("version", "v1")
		form.Set("params", `{"payload":{"n":9007199254740993},"count":9007199254740993,"ratio":0.5}`)

		req := parseRPCRequest(t, fiber.MIMEApplicationForm, form.Encode())

		assert.Equal(t, json.Number("9007199254740993"), req.Params["count"], "form params should hold json.Number")

		var out echoParams

		require.NoError(t, req.Params.Decode(&out), "Decode should succeed")
		assert.Equal(t, `{"n":9007199254740993}`, string(out.Payload), "RawMessage param must keep the exact digits")
		assert.Equal(t, int64(9007199254740993), out.Count, "int64 param must keep exact digits")
		assert.Equal(t, 0.5, out.Ratio, "plain numeric param should bind as before")
	})
}
