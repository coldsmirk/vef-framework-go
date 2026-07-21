package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/security"
)

// TestParseAction covers the action-string → (method, subPath) splitting logic.
func TestParseAction(t *testing.T) {
	r := &REST{}

	tests := []struct {
		name            string
		action          string
		expectedMethod  string
		expectedSubPath string
	}{
		{
			name:            "MethodOnly",
			action:          "GET",
			expectedMethod:  "GET",
			expectedSubPath: "",
		},
		{
			name:            "MethodOnlyLowercase",
			action:          "get",
			expectedMethod:  "GET",
			expectedSubPath: "",
		},
		{
			name:            "MethodWithLeadingSlashPath",
			action:          "POST /items",
			expectedMethod:  "POST",
			expectedSubPath: "/items",
		},
		{
			name:            "MethodWithoutLeadingSlash",
			action:          "DELETE :id",
			expectedMethod:  "DELETE",
			expectedSubPath: "/:id",
		},
		{
			name:            "MethodWithNestedPath",
			action:          "GET /users/:id/profile",
			expectedMethod:  "GET",
			expectedSubPath: "/users/:id/profile",
		},
		{
			name:            "LowercaseMethodWithPath",
			action:          "put /items/:id",
			expectedMethod:  "PUT",
			expectedSubPath: "/items/:id",
		},
		{
			name:            "ExtraSpacesInPath",
			action:          "PATCH  /items",
			expectedMethod:  "PATCH",
			expectedSubPath: "/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, subPath := r.parseAction(tt.action)
			assert.Equal(t, tt.expectedMethod, method, "HTTP method should be uppercased")
			assert.Equal(t, tt.expectedSubPath, subPath, "sub-path should have a leading slash when non-empty")
		})
	}
}

// TestBuildPath covers full-URL path construction from resource + subPath.
func TestBuildPath(t *testing.T) {
	r := &REST{}

	tests := []struct {
		name     string
		resource string
		subPath  string
		expected string
	}{
		{
			name:     "ResourceOnly",
			resource: "users",
			subPath:  "",
			expected: "/users",
		},
		{
			name:     "ResourceWithSubPath",
			resource: "users",
			subPath:  "/:id",
			expected: "/users/:id",
		},
		{
			name:     "ResourceWithNestedPath",
			resource: "orders",
			subPath:  "/:id/items",
			expected: "/orders/:id/items",
		},
		{
			name:     "SlashInResourceName",
			resource: "sys/users",
			subPath:  "",
			expected: "/sys/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := r.buildPath(tt.resource, tt.subPath)
			assert.Equal(t, tt.expected, path, "built path should combine resource and sub-path correctly")
		})
	}
}

// TestParseJSONBody covers the REST JSON body producer: numbers must arrive
// in params as json.Number so typed decoding stays digit-exact.
func TestParseJSONBody(t *testing.T) {
	var (
		captured *api.Request
		parseErr error
	)

	app := fiber.New()
	app.Post("/items", func(ctx fiber.Ctx) error {
		r := &REST{}
		req := &api.Request{Params: make(api.Params)}
		parseErr = r.parseBody(ctx, req)
		captured = req

		return ctx.SendStatus(fiber.StatusOK)
	})

	body := `{"count":9007199254740993,"ratio":0.5}`
	httpReq := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/items", strings.NewReader(body))
	httpReq.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := app.Test(httpReq)
	require.NoError(t, err, "fiber app.Test should not error")
	require.NotNil(t, resp, "response should not be nil")
	require.NoError(t, parseErr, "parseBody should succeed")

	assert.Equal(t, json.Number("9007199254740993"), captured.Params["count"], "REST params should hold json.Number")

	var out struct {
		Count int64   `json:"count"`
		Ratio float64 `json:"ratio"`
	}

	require.NoError(t, captured.Params.Decode(&out), "Decode should succeed")
	assert.Equal(t, int64(9007199254740993), out.Count, "int64 param must keep exact digits")
	assert.Equal(t, 0.5, out.Ratio, "plain numeric param should bind as before")
}

// TestExtractMeta covers header-prefix stripping and lowercase normalization.
func TestExtractMeta(t *testing.T) {
	tests := []struct {
		name         string
		headers      map[string]string
		expectedMeta map[string]string
	}{
		{
			name:         "SingleMetaHeader",
			headers:      map[string]string{"X-Meta-TenantId": "acme"},
			expectedMeta: map[string]string{"tenantid": "acme"},
		},
		{
			name:         "MetaKeyLowercased",
			headers:      map[string]string{"X-Meta-UserId": "42"},
			expectedMeta: map[string]string{"userid": "42"},
		},
		{
			name:         "MultipleMetaHeaders",
			headers:      map[string]string{"X-Meta-Lang": "en", "X-Meta-Region": "US"},
			expectedMeta: map[string]string{"lang": "en", "region": "US"},
		},
		{
			name:         "NonMetaHeaderIgnored",
			headers:      map[string]string{"X-App-ID": "app1"},
			expectedMeta: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedMeta api.Meta

			app := fiber.New()
			app.Get("/test", func(ctx fiber.Ctx) error {
				r := &REST{}
				req := &api.Request{Meta: make(api.Meta)}
				r.extractMeta(ctx, req)
				capturedMeta = req.Meta

				return ctx.SendStatus(fiber.StatusOK)
			})

			httpReq := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test", nil)
			for k, v := range tt.headers {
				httpReq.Header.Set(k, v)
			}

			resp, err := app.Test(httpReq)
			require.NoError(t, err, "fiber app.Test should not error")
			require.NotNil(t, resp, "response should not be nil")

			for k, v := range tt.expectedMeta {
				got, ok := capturedMeta[k]
				require.True(t, ok, "meta key %q should be present", k)
				assert.Equal(t, v, got, "meta value for key %q should match", k)
			}

			// No extra keys should be present beyond what's expected.
			assert.Len(t, capturedMeta, len(tt.expectedMeta), "meta map should contain exactly the expected keys")
		})
	}
}

func TestExtractQueryParams(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedParams map[string]any
	}{
		{
			name:           "BusinessQueryReachesParams",
			query:          "keyword=test&page=1",
			expectedParams: map[string]any{"keyword": "test", "page": "1"},
		},
		{
			name:           "ReservedKeyIsSkipped",
			query:          security.QueryKeyAccessToken + "=secret-token",
			expectedParams: map[string]any{},
		},
		{
			name:           "ReservedKeyIsSkippedBesideBusinessKeys",
			query:          "keyword=test&" + security.QueryKeyAccessToken + "=secret-token",
			expectedParams: map[string]any{"keyword": "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedParams api.Params

			app := fiber.New()
			app.Get("/test", func(ctx fiber.Ctx) error {
				r := &REST{}
				req := &api.Request{Params: make(api.Params)}
				r.extractQueryParams(ctx, req)
				capturedParams = req.Params

				return ctx.SendStatus(fiber.StatusOK)
			})

			httpReq := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/test?"+tt.query, nil)

			resp, err := app.Test(httpReq)
			require.NoError(t, err, "fiber app.Test should not error")
			require.NotNil(t, resp, "response should not be nil")

			assert.Equal(t, tt.expectedParams, map[string]any(capturedParams),
				"a framework-reserved query key must never reach the business parameter space")
		})
	}
}
