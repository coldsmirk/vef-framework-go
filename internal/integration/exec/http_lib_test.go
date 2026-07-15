package exec

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/js"
)

// newScopedRuntime builds a bare runtime with the scoped http lib bound to
// baseURL, returning the runtime and a trace-carrying context.
func newScopedRuntime(t *testing.T, baseURL string) (*js.Runtime, context.Context, *traceCollector) {
	t.Helper()

	client, err := httpx.New(httpx.WithBaseURL(baseURL), httpx.WithTimeout(5*time.Second))
	require.NoError(t, err, "Client construction should succeed")

	engine, err := js.NewEngine(js.WithoutStdLibs())
	require.NoError(t, err, "Engine construction should succeed")

	rt, err := engine.NewRuntime()
	require.NoError(t, err, "Runtime construction should succeed")

	require.NoError(t, newHTTPLib(client, 5*time.Second).Install(rt), "http lib should install")
	require.NoError(t, newErrorsLib().Install(rt), "errors lib should install")

	collector := newTraceCollector(newCapturer(new(config.IntegrationLogConfig)))

	return rt, withTrace(t.Context(), collector), collector
}

func TestHTTPLib(t *testing.T) {
	var (
		gotMethod  string
		gotPath    string
		gotQuery   string
		gotBody    []byte
		gotContent string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")
		gotContent = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"value":42}`))
	}))
	defer srv.Close()

	rt, ctx, collector := newScopedRuntime(t, srv.URL)

	t.Run("GetAndResponseShape", func(t *testing.T) {
		value, err := rt.RunString(ctx, `
			(function(){
				const resp = http.get('/patients', { query: { q: 'zhang' } })
				return { status: resp.status, ok: resp.ok, parsed: resp.json().value, text: resp.text() }
			})()
		`)
		require.NoError(t, err, "Script should execute successfully")

		exported, ok := value.Export().(map[string]any)
		require.True(t, ok, "Script should return an object")
		assert.Equal(t, int64(200), exported["status"], "Status should be exposed")
		assert.Equal(t, true, exported["ok"], "ok should reflect 2xx")
		assert.Equal(t, int64(42), exported["parsed"], "json() should parse the body")
		assert.Contains(t, exported["text"], `"ok":true`, "text() should return the raw body")
		assert.Equal(t, http.MethodGet, gotMethod, "GET should reach the server")
		assert.Equal(t, "/patients", gotPath, "Path should join the base URL")
		assert.Equal(t, "zhang", gotQuery, "Query option should reach the server")
	})

	t.Run("PostObjectBodyImpliesJSON", func(t *testing.T) {
		_, err := rt.RunString(ctx, `http.post('/patients', { name: 'He' })`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "application/json", gotContent, "Object body should imply JSON content type")
		assert.JSONEq(t, `{"name":"He"}`, string(gotBody), "Object body should be JSON-encoded")
	})

	t.Run("StringBodyPassesVerbatim", func(t *testing.T) {
		_, err := rt.RunString(ctx, `http.post('/raw', 'payload', { headers: { 'Content-Type': 'text/plain' } })`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "payload", string(gotBody), "String body should pass through unchanged")
		assert.Equal(t, "text/plain", gotContent, "Explicit content type should win")
	})

	t.Run("FetchWithMethod", func(t *testing.T) {
		_, err := rt.RunString(ctx, `http.fetch('/x', { method: 'patch', body: 'b' })`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, http.MethodPatch, gotMethod, "fetch method should be honored")
	})

	t.Run("AbsoluteURLRejected", func(t *testing.T) {
		_, err := rt.RunString(ctx, `http.get('https://evil.example.com/steal')`)
		require.Error(t, err, "Absolute URL should be rejected")
		assert.ErrorIs(t, err, ErrAbsoluteURLNotAllowed, "Error should be the absolute-URL sentinel")
	})

	t.Run("SchemeRelativeURLRejected", func(t *testing.T) {
		_, err := rt.RunString(ctx, `http.get('//evil.example.com/steal')`)
		require.Error(t, err, "Scheme-relative URL should be rejected")
		assert.ErrorIs(t, err, ErrAbsoluteURLNotAllowed, "Error should be the absolute-URL sentinel")
	})

	t.Run("UnsupportedRedirectModeRejected", func(t *testing.T) {
		_, err := rt.RunString(ctx, `http.get('/x', { redirect: 'manual' })`)
		require.Error(t, err, "Unsupported redirect mode should be rejected")
		assert.ErrorIs(t, err, ErrRedirectModeUnsupported, "Error should be the redirect-mode sentinel")
	})

	t.Run("ExchangesAreTraced", func(t *testing.T) {
		before := len(collector.Exchanges())

		_, err := rt.RunString(ctx, `http.get('/traced')`)
		require.NoError(t, err, "Script should execute successfully")

		exchanges := collector.Exchanges()
		require.Len(t, exchanges, before+1, "Each call should add one trace exchange")
		last := exchanges[len(exchanges)-1]
		assert.Equal(t, http.MethodGet, last.Method, "Trace should record the method")
		assert.Contains(t, last.URL, "/traced", "Trace should record the resolved URL")
		assert.Equal(t, http.StatusOK, last.Status, "Trace should record the status")
	})
}

func TestHTTPLibTransportFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // closed on purpose: every call now fails at the transport

	rt, ctx, collector := newScopedRuntime(t, srv.URL)

	_, err := rt.RunString(ctx, `http.get('/down')`)
	require.Error(t, err, "Transport failure should throw")

	var transport *transportError
	require.ErrorAs(t, err, &transport, "Error should carry the transport marker")

	exchanges := collector.Exchanges()
	require.NotEmpty(t, exchanges, "Failed call should still be traced")
	assert.NotEmpty(t, exchanges[len(exchanges)-1].Error, "Trace should record the transport error")

	t.Run("CatchableInScript", func(t *testing.T) {
		value, err := rt.RunString(ctx, `
			(function(){
				try { http.get('/down') } catch (e) { return 'caught' }
				return 'missed'
			})()
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Equal(t, "caught", value.Export(), "Transport error should be catchable in script")
	})
}
