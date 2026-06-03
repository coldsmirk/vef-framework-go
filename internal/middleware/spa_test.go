package middleware

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/middleware"
)

func TestSpaEntryPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "Empty", path: "", want: "/"},
		{name: "Root", path: "/", want: "/"},
		{name: "Nested", path: "/app", want: "/app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spaEntryPath(&middleware.SPAConfig{Path: tt.path})
			assert.Equal(t, tt.want, got, "spaEntryPath should default an empty path to root and otherwise echo it")
		})
	}
}

func TestSpaStaticPrefix(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "EmptyDefaultsToRoot", path: "", want: "/static/"},
		{name: "Root", path: "/", want: "/static/"},
		{name: "Nested", path: "/app", want: "/app/static/"},
		{name: "NestedTrailingSlash", path: "/app/", want: "/app/static/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spaStaticPrefix(&middleware.SPAConfig{Path: tt.path})
			assert.Equal(t, tt.want, got, "spaStaticPrefix must normalize the join and never emit a double slash")
		})
	}
}

func TestHasAnyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		reqPath  string
		prefixes []string
		want     bool
	}{
		{name: "MatchesFirst", reqPath: "/api/users", prefixes: []string{"/api", "/ws"}, want: true},
		{name: "MatchesSecond", reqPath: "/ws/feed", prefixes: []string{"/api", "/ws"}, want: true},
		{name: "NoMatch", reqPath: "/dashboard", prefixes: []string{"/api", "/ws"}, want: false},
		{name: "NilPrefixes", reqPath: "/api/users", prefixes: nil, want: false},
		{name: "EmptyPrefixIgnored", reqPath: "/anything", prefixes: []string{""}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAnyPrefix(tt.reqPath, tt.prefixes)
			assert.Equal(t, tt.want, got, "hasAnyPrefix must ignore empty prefixes and match real ones")
		})
	}
}

func TestNewSPAMiddleware(t *testing.T) {
	t.Run("NilWhenNoConfigs", func(t *testing.T) {
		assert.Nil(t, NewSPAMiddleware(nil), "NewSPAMiddleware should return nil so the empty member is filtered from the middleware group")
	})

	t.Run("BuildsMiddlewareForConfigs", func(t *testing.T) {
		mw := NewSPAMiddleware([]*middleware.SPAConfig{{Path: "/"}})
		require.NotNil(t, mw, "NewSPAMiddleware should build a middleware when configs are supplied")
		assert.Equal(t, "spa", mw.Name(), "SPA middleware name should be stable")
		assert.Positive(t, mw.Order(), "SPA middleware must run after route handlers")
	})
}

func TestSPAMiddlewareApply(t *testing.T) {
	spaFS := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>spa</title>")},
	}

	newApp := func(t *testing.T, config *middleware.SPAConfig) *fiber.App {
		t.Helper()

		app := fiber.New()
		// A real API route so unmatched /api paths can 404 instead of being rewritten.
		app.Get("/api/health", func(c fiber.Ctx) error {
			return c.SendString("ok")
		})

		mw := NewSPAMiddleware([]*middleware.SPAConfig{config})
		require.NotNil(t, mw, "middleware should be built for the supplied config")
		mw.Apply(app)

		return app
	}

	get := func(t *testing.T, app *fiber.App, path string) (int, string) {
		t.Helper()

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, path, nil)
		resp, err := app.Test(req)
		require.NoError(t, err, "request should complete without error")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "reading the response body should not fail")

		return resp.StatusCode, string(body)
	}

	t.Run("RootMountServesIndex", func(t *testing.T) {
		app := newApp(t, &middleware.SPAConfig{Path: "/", Fs: spaFS})
		status, body := get(t, app, "/")
		assert.Equal(t, fiber.StatusOK, status, "root should serve the SPA index")
		assert.Contains(t, body, "<title>spa</title>", "root should return the index.html body")
	})

	t.Run("UnknownClientRouteFallsBackToIndex", func(t *testing.T) {
		app := newApp(t, &middleware.SPAConfig{Path: "/", Fs: spaFS})
		status, body := get(t, app, "/dashboard/settings")
		assert.Equal(t, fiber.StatusOK, status, "deep client routes should fall back to the SPA index")
		assert.Contains(t, body, "<title>spa</title>", "client-side routes should receive index.html")
	})

	t.Run("ExcludedAPIPathIsNotRewritten", func(t *testing.T) {
		app := newApp(t, &middleware.SPAConfig{Path: "/", Fs: spaFS, ExcludePaths: []string{"/api"}})
		status, body := get(t, app, "/api/missing")
		assert.Equal(t, fiber.StatusNotFound, status, "excluded /api paths must 404 instead of returning the SPA index")
		assert.NotContains(t, body, "<title>spa</title>", "excluded paths must not be rewritten to index.html")
	})

	t.Run("ExistingAPIRouteStillReachable", func(t *testing.T) {
		app := newApp(t, &middleware.SPAConfig{Path: "/", Fs: spaFS, ExcludePaths: []string{"/api"}})
		status, body := get(t, app, "/api/health")
		assert.Equal(t, fiber.StatusOK, status, "real API routes must remain reachable under the SPA catch-all")
		assert.Equal(t, "ok", body, "API handler response should pass through untouched")
	})
}
