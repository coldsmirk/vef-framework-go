package jshttp_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/js/jshttp"
)

// newHTTPRuntime builds a bare runtime with the http library enabled and the
// given target URL bound as the global "target".
func newHTTPRuntime(t *testing.T, target string, opts ...jshttp.Option) *js.Runtime {
	t.Helper()

	engine, err := js.NewEngine(js.WithoutStdLibs(), js.WithLibs(jshttp.New(opts...)))
	require.NoError(t, err, "NewEngine should succeed")

	rt, err := engine.NewRuntime(js.EnableLibs(jshttp.Name))
	require.NoError(t, err, "NewRuntime should succeed")

	require.NoError(t, rt.Set("target", target), "Set should bind the target URL")

	return rt
}

// TestGet tests plain GET requests and the fetch-aligned response shape.
func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "vef")
		w.Header().Add("X-Multi", "a")
		w.Header().Add("X-Multi", "b")
		_, _ = w.Write([]byte(`{"message":"hi"}`))
	}))
	defer srv.Close()

	rt := newHTTPRuntime(t, srv.URL)

	result, err := rt.RunString(t.Context(), `
		const resp = http.get(target);
		({
			status: resp.status,
			statusText: resp.statusText,
			ok: resp.ok,
			url: resp.url,
			redirected: resp.redirected,
			body: resp.body,
			text: resp.text(),
			message: resp.json().message,
			custom: resp.headers['x-custom'],
			multi: resp.headers['x-multi'],
			bytes: resp.arrayBuffer().byteLength
		})
	`)
	require.NoError(t, err, "Script should execute successfully")

	obj := result.ToObject(rt.VM())
	assert.Equal(t, int64(http.StatusOK), obj.Get("status").ToInteger(), "Status should be 200")
	assert.Equal(t, "OK", obj.Get("statusText").String(), "Status text should match the status code")
	assert.True(t, obj.Get("ok").ToBoolean(), "Ok should be true for 2xx")
	assert.Equal(t, srv.URL, obj.Get("url").String(), "Url should be the final request URL")
	assert.False(t, obj.Get("redirected").ToBoolean(), "Redirected should be false without redirects")
	assert.JSONEq(t, `{"message":"hi"}`, obj.Get("body").String(), "Body should carry the raw payload")
	assert.JSONEq(t, `{"message":"hi"}`, obj.Get("text").String(), "text() should match the body")
	assert.Equal(t, "hi", obj.Get("message").String(), "json() should parse the payload")
	assert.Equal(t, "vef", obj.Get("custom").String(), "Header names should be lower-cased")
	assert.Equal(t, "a, b", obj.Get("multi").String(), "Multi-value headers should be comma-joined")
	assert.Equal(t, int64(16), obj.Get("bytes").ToInteger(), "arrayBuffer() should expose the raw bytes")
}

// TestPost tests request body encoding.
func TestPost(t *testing.T) {
	var (
		gotBody        []byte
		gotContentType string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotContentType = r.Header.Get("Content-Type")

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	rt := newHTTPRuntime(t, srv.URL)

	t.Run("ObjectBodyEncodedAsJSON", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `http.post(target, { name: 'vef', count: 2 }).status`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(http.StatusCreated), result.ToInteger(), "Status should be 201")
		assert.Equal(t, "application/json", gotContentType, "Object body should imply a JSON content type")
		assert.JSONEq(t, `{"name":"vef","count":2}`, string(gotBody), "Object body should be JSON-encoded")
	})

	t.Run("StringBodyPassedVerbatim", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `http.post(target, 'raw-payload', { headers: { 'Content-Type': 'text/plain' } })`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, "raw-payload", string(gotBody), "String body should pass through unchanged")
		assert.Equal(t, "text/plain", gotContentType, "Explicit content type should be respected")
	})
}

// TestFetch tests the fetch entry point: init handling, query, headers, and
// invalid input.
func TestFetch(t *testing.T) {
	var (
		gotMethod string
		gotQuery  string
		gotToken  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.Query().Get("q")
		gotToken = r.Header.Get("X-Token")

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt := newHTTPRuntime(t, srv.URL)

	t.Run("DefaultsToGet", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `http.fetch(target).status`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, int64(http.StatusOK), result.ToInteger(), "Fetch without init should default to GET")
		assert.Equal(t, http.MethodGet, gotMethod, "Method should default to GET")
	})

	t.Run("MethodQueryAndHeaders", func(t *testing.T) {
		_, err := rt.RunString(t.Context(), `
			http.fetch(target, { method: 'patch', query: { q: 'find' }, headers: { 'X-Token': 'abc' }, body: 'x' })
		`)
		require.NoError(t, err, "Script should execute successfully")
		assert.Equal(t, http.MethodPatch, gotMethod, "Method should be upper-cased and honored")
		assert.Equal(t, "find", gotQuery, "Query init should reach the server")
		assert.Equal(t, "abc", gotToken, "Header init should reach the server")
	})

	t.Run("InvalidRedirectMode", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `
			try {
				http.fetch(target, { redirect: 'bounce' });
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Contains(t, result.String(), "invalid request", "Unknown redirect mode should throw a catchable error")
	})

	t.Run("MissingURL", func(t *testing.T) {
		result, err := rt.RunString(t.Context(), `
			try {
				http.fetch();
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Contains(t, result.String(), "scheme not allowed", "Fetch without a URL should throw a catchable error")
	})
}

// TestRedirectModes tests the fetch redirect member.
func TestRedirectModes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("landed"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("FollowByDefault", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL)

		result, err := rt.RunString(t.Context(), `
			const resp = http.fetch(target + '/start');
			({ status: resp.status, url: resp.url, redirected: resp.redirected, body: resp.body })
		`)
		require.NoError(t, err, "Script should execute successfully")

		obj := result.ToObject(rt.VM())
		assert.Equal(t, int64(http.StatusOK), obj.Get("status").ToInteger(), "Redirect should be followed")
		assert.Equal(t, srv.URL+"/final", obj.Get("url").String(), "Url should be the redirect target")
		assert.True(t, obj.Get("redirected").ToBoolean(), "Redirected should be true after following")
		assert.Equal(t, "landed", obj.Get("body").String(), "Body should come from the final response")
	})

	t.Run("Manual", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL)

		result, err := rt.RunString(t.Context(), `
			const resp = http.fetch(target + '/start', { redirect: 'manual' });
			({ status: resp.status, location: resp.headers['location'], redirected: resp.redirected })
		`)
		require.NoError(t, err, "Script should execute successfully")

		obj := result.ToObject(rt.VM())
		assert.Equal(t, int64(http.StatusFound), obj.Get("status").ToInteger(), "Manual mode should surface the 3xx response")
		assert.Equal(t, "/final", obj.Get("location").String(), "Location header should be exposed")
		assert.False(t, obj.Get("redirected").ToBoolean(), "Redirected should be false in manual mode")
	})

	t.Run("Error", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL)

		result, err := rt.RunString(t.Context(), `
			try {
				http.fetch(target + '/start', { redirect: 'error' });
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Script should handle the thrown error")
		assert.Contains(t, result.String(), "redirect blocked", "Error mode should reject the redirect")
	})
}

// TestGuard tests that restrictions are off by default and enforced once
// opted in.
func TestGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Run("LoopbackAllowedByDefault", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL)

		result, err := rt.RunString(t.Context(), `http.get(target).status`)
		require.NoError(t, err, "Loopback target should be reachable by default")
		assert.Equal(t, int64(http.StatusOK), result.ToInteger(), "Request should succeed")
	})

	t.Run("PublicNetworkOnlyBlocksLoopback", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL, jshttp.WithPublicNetworkOnly())

		_, err := rt.RunString(t.Context(), `http.get(target)`)
		require.Error(t, err, "Loopback target should be rejected under WithPublicNetworkOnly")
		assert.Contains(t, err.Error(), "address not allowed", "Error should carry the guard reason")
	})

	t.Run("HostOutsideAllowlist", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL, jshttp.WithAllowedHosts("api.example.com"))

		_, err := rt.RunString(t.Context(), `http.get(target)`)
		require.Error(t, err, "Host outside the allowlist should be rejected")
		assert.Contains(t, err.Error(), "host not allowed", "Error should carry the allowlist reason")
	})

	t.Run("AllowlistedHostPasses", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL, jshttp.WithAllowedHosts("127.0.0.1"))

		result, err := rt.RunString(t.Context(), `http.get(target).status`)
		require.NoError(t, err, "Allowlisted host should pass")
		assert.Equal(t, int64(http.StatusOK), result.ToInteger(), "Request should succeed")
	})

	t.Run("RedirectOutsideAllowlistBlocked", func(t *testing.T) {
		redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://localhost:9/next", http.StatusFound)
		}))
		defer redirecting.Close()

		rt := newHTTPRuntime(t, redirecting.URL, jshttp.WithAllowedHosts("127.0.0.1"))

		_, err := rt.RunString(t.Context(), `http.get(target)`)
		require.Error(t, err, "Redirect to a host outside the allowlist should be rejected")
		assert.Contains(t, err.Error(), "host not allowed", "Error should carry the allowlist reason")
	})

	t.Run("ScriptCanCatchGuardError", func(t *testing.T) {
		rt := newHTTPRuntime(t, srv.URL, jshttp.WithPublicNetworkOnly())

		result, err := rt.RunString(t.Context(), `
			try {
				http.get(target);
				'no error'
			} catch (e) {
				String(e)
			}
		`)
		require.NoError(t, err, "Guard errors should be catchable in scripts")
		assert.Contains(t, result.String(), "address not allowed", "Caught error should carry the guard reason")
	})
}

// TestResourceLimits tests that limits are off by default and enforced once
// opted in.
func TestResourceLimits(t *testing.T) {
	t.Run("BodyUnboundedByDefault", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, 64<<10))
		}))
		defer srv.Close()

		rt := newHTTPRuntime(t, srv.URL)

		result, err := rt.RunString(t.Context(), `http.get(target).body.length`)
		require.NoError(t, err, "Large responses should pass without a configured cap")
		assert.Equal(t, int64(64<<10), result.ToInteger(), "Full body should be delivered")
	})

	t.Run("BodyTooLarge", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(make([]byte, 100))
		}))
		defer srv.Close()

		rt := newHTTPRuntime(t, srv.URL, jshttp.WithMaxBodySize(8))

		_, err := rt.RunString(t.Context(), `http.get(target)`)
		require.Error(t, err, "Oversized response should fail instead of being truncated")
		assert.Contains(t, err.Error(), "body too large", "Error should carry the size limit reason")
	})

	t.Run("TimeoutCapsSlowServer", func(t *testing.T) {
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			<-release
			w.WriteHeader(http.StatusOK)
		}))

		defer func() {
			close(release)
			srv.Close()
		}()

		rt := newHTTPRuntime(t, srv.URL, jshttp.WithTimeout(100*time.Millisecond))

		start := time.Now()
		_, err := rt.RunString(t.Context(), `http.get(target)`)
		require.Error(t, err, "A slow server should trip the configured timeout")
		assert.Less(t, time.Since(start), 5*time.Second, "Request should abort promptly")
	})
}

// TestJSONHelperError tests json() on a non-JSON payload.
func TestJSONHelperError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("plain text"))
	}))
	defer srv.Close()

	rt := newHTTPRuntime(t, srv.URL)

	result, err := rt.RunString(t.Context(), `
		const resp = http.get(target);
		try {
			resp.json();
			'no error'
		} catch (e) {
			'parse failed'
		}
	`)
	require.NoError(t, err, "Script should handle the parse error")
	assert.Equal(t, "parse failed", result.String(), "json() on non-JSON should throw a catchable error")
}

// TestNestedJSONRoundTrip tests that parsed JSON payloads stay structured.
func TestNestedJSONRoundTrip(t *testing.T) {
	payload := map[string]any{
		"items": []any{map[string]any{"id": 1, "tags": []any{"a", "b"}}},
		"total": 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	rt := newHTTPRuntime(t, srv.URL)

	result, err := rt.RunString(t.Context(), `
		const data = http.get(target).json();
		data.items[0].tags.join(',') + ':' + data.total
	`)
	require.NoError(t, err, "Script should execute successfully")
	assert.Equal(t, "a,b:1", result.String(), "Nested JSON should be navigable in the script")
}
