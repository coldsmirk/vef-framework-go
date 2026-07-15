package httpx_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/version"
)

// newTestServer starts an httptest server closed on test cleanup.
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

// newClient builds a client whose construction must succeed.
func newClient(t *testing.T, opts ...httpx.Option) *httpx.Client {
	t.Helper()

	client, err := httpx.New(opts...)
	require.NoError(t, err, "New should succeed")

	return client
}

// StubTransport returns a canned response for every request without touching
// the network.
type StubTransport struct {
	status int
	body   string
}

func (s *StubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Status:     fmt.Sprintf("%d %s", s.status, http.StatusText(s.status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(s.body)),
	}, nil
}

// TestNew tests eager option validation.
func TestNew(t *testing.T) {
	t.Run("ZeroOptions", func(t *testing.T) {
		_, err := httpx.New()
		require.NoError(t, err, "The zero-option client should build")
	})

	t.Run("InvalidBaseURL", func(t *testing.T) {
		_, err := httpx.New(httpx.WithBaseURL("/relative"))
		require.ErrorIs(t, err, httpx.ErrInvalidOption, "A relative base URL should fail construction")
	})

	t.Run("InvalidProxyURL", func(t *testing.T) {
		_, err := httpx.New(httpx.WithProxy("/relative"))
		require.ErrorIs(t, err, httpx.ErrInvalidOption, "A relative proxy URL should fail construction")
	})

	t.Run("HTTPClientConflict", func(t *testing.T) {
		_, err := httpx.New(httpx.WithHTTPClient(new(http.Client)), httpx.WithProxy("http://proxy.local"))
		require.ErrorIs(t, err, httpx.ErrConflictingOptions, "WithHTTPClient combined with WithProxy should fail construction")
	})

	t.Run("TransportConflict", func(t *testing.T) {
		_, err := httpx.New(httpx.WithTransport(http.DefaultTransport), httpx.WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
		require.ErrorIs(t, err, httpx.ErrConflictingOptions, "WithTransport combined with WithTLSConfig should fail construction")
	})

	t.Run("NegativeMaxRedirects", func(t *testing.T) {
		_, err := httpx.New(httpx.WithMaxRedirects(-1))
		require.ErrorIs(t, err, httpx.ErrInvalidOption, "A negative redirect cap should fail construction")
	})
}

// TestClientDefaults tests client-level defaults and their request-level
// overrides.
func TestClientDefaults(t *testing.T) {
	t.Run("HeaderAndQuery", func(t *testing.T) {
		var header, query string

		server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			header = r.Header.Get("X-Api-Key")
			query = r.URL.Query().Get("apiKey")
		})

		client := newClient(t, httpx.WithHeader("X-Api-Key", "default-key"), httpx.WithQuery("apiKey", "default-query"))

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "default-key", header, "Client default header should reach the server")
		assert.Equal(t, "default-query", query, "Client default query parameter should reach the server")

		_, err = client.NewRequest().SetHeader("X-Api-Key", "override").SetQuery("apiKey", "override").Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "override", header, "Request header should replace the client default")
		assert.Equal(t, "override", query, "Request query parameter should replace the client default")
	})

	t.Run("UserAgent", func(t *testing.T) {
		var agent string

		server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			agent = r.Header.Get("User-Agent")
		})

		client := newClient(t)

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "vef/"+version.VEFVersion, agent, "The default User-Agent should carry the framework version")

		_, err = client.NewRequest().SetHeader("User-Agent", "custom-agent").Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "custom-agent", agent, "An explicit User-Agent should replace the default")
	})

	t.Run("AuthorizationPrecedence", func(t *testing.T) {
		var authorization string

		server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			authorization = r.Header.Get("Authorization")
		})

		client := newClient(t, httpx.WithBearerToken("client-token"))

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "Bearer client-token", authorization, "Client-level authentication should apply by default")

		_, err = client.NewRequest().SetBasicAuth("user", "pass").Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.True(t, strings.HasPrefix(authorization, "Basic "), "Request-level authentication should override the client default")
	})
}

// TestBaseURL tests base URL joining semantics.
func TestBaseURL(t *testing.T) {
	var path string

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
	})

	t.Run("JoinsRelative", func(t *testing.T) {
		client := newClient(t, httpx.WithBaseURL(server.URL+"/v1/"))

		_, err := client.NewRequest().Get(t.Context(), "/orders")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "/v1/orders", path, "A relative URL should join onto the base URL path")

		_, err = client.NewRequest().Get(t.Context(), "orders")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "/v1/orders", path, "Leading-slash and bare relative URLs should join identically")
	})

	t.Run("EmptyURLTargetsBase", func(t *testing.T) {
		client := newClient(t, httpx.WithBaseURL(server.URL+"/v1"))

		_, err := client.NewRequest().Get(t.Context(), "")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "/v1", path, "An empty URL should target the base URL itself")
	})

	t.Run("AbsoluteBypassesBase", func(t *testing.T) {
		client := newClient(t, httpx.WithBaseURL("http://base.invalid"))

		_, err := client.NewRequest().Get(t.Context(), server.URL+"/direct")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "/direct", path, "An absolute URL should bypass the base URL")
	})

	t.Run("RelativeWithoutBase", func(t *testing.T) {
		client := newClient(t)

		_, err := client.NewRequest().Get(t.Context(), "/orders")
		require.ErrorIs(t, err, httpx.ErrInvalidRequestURL, "A relative URL without a base URL should fail")
	})
}

// TestRedirects tests the redirect policy built from WithMaxRedirects.
func TestRedirects(t *testing.T) {
	t.Run("FollowsByDefault", func(t *testing.T) {
		mux := http.NewServeMux()
		server := newTestServer(t, mux.ServeHTTP)
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/target", http.StatusFound)
		})
		mux.HandleFunc("/target", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("arrived"))
		})

		resp, err := newClient(t).NewRequest().Get(t.Context(), server.URL+"/start")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "arrived", resp.String(), "The redirect should be followed by default")
	})

	t.Run("ZeroCapReturnsRedirect", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		})

		resp, err := newClient(t, httpx.WithMaxRedirects(0)).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, http.StatusFound, resp.StatusCode(), "A zero cap should return the 3xx response as-is")
		assert.NotEmpty(t, resp.Header("Location"), "The redirect response should keep its Location header")
	})

	t.Run("CapExceeded", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/loop", http.StatusFound)
		})

		_, err := newClient(t, httpx.WithMaxRedirects(2)).NewRequest().Get(t.Context(), server.URL)
		require.ErrorIs(t, err, httpx.ErrTooManyRedirects, "Exceeding the redirect cap should fail the call")
	})
}

// TestTimeout tests the whole-call deadline and its per-request override.
func TestTimeout(t *testing.T) {
	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(60 * time.Millisecond):
		case <-r.Context().Done():
		}
	})

	t.Run("ClientTimeoutExpires", func(t *testing.T) {
		client := newClient(t, httpx.WithTimeout(10*time.Millisecond))

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.ErrorIs(t, err, context.DeadlineExceeded, "The client timeout should cancel the call")
	})

	t.Run("RequestOverrideExtends", func(t *testing.T) {
		client := newClient(t, httpx.WithTimeout(10*time.Millisecond))

		_, err := client.NewRequest().SetTimeout(2*time.Second).Get(t.Context(), server.URL)
		require.NoError(t, err, "A longer per-request timeout should replace the client default")
	})

	t.Run("RequestOverrideRemoves", func(t *testing.T) {
		client := newClient(t, httpx.WithTimeout(10*time.Millisecond))

		_, err := client.NewRequest().SetTimeout(0).Get(t.Context(), server.URL)
		require.NoError(t, err, "A zero per-request timeout should remove the bound")
	})
}

// TestHooks tests request and response hook semantics.
func TestHooks(t *testing.T) {
	t.Run("RequestHookAdjustsHeaders", func(t *testing.T) {
		var signature string

		server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			signature = r.Header.Get("X-Signature")
		})

		sign := func(req *httpx.Request) error {
			req.SetHeader("X-Signature", fmt.Sprintf("%s:%d", req.Method(), len(req.Body())))

			return nil
		}

		client := newClient(t, httpx.WithRequestHook(sign))

		_, err := client.NewRequest().SetBody([]byte("12345"), "text/plain").Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "POST:5", signature, "The hook should see the final method and body and mutate headers")
	})

	t.Run("RequestHookErrorAborts", func(t *testing.T) {
		var hits atomic.Int32

		server := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
		})

		client := newClient(t, httpx.WithRequestHook(func(*httpx.Request) error {
			return errors.New("rejected")
		}))

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.ErrorContains(t, err, "request hook", "The hook error should surface with context")
		assert.Zero(t, hits.Load(), "A failing request hook should abort before the wire")
	})

	t.Run("ResponseHookObserves", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})

		var status int

		client := newClient(t, httpx.WithResponseHook(func(resp *httpx.Response) error {
			status = resp.StatusCode()

			return nil
		}))

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, http.StatusCreated, status, "The response hook should observe the buffered response")
	})

	t.Run("ResponseHookErrorFails", func(t *testing.T) {
		server := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {})

		client := newClient(t, httpx.WithResponseHook(func(*httpx.Response) error {
			return errors.New("rejected")
		}))

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.ErrorContains(t, err, "response hook", "The hook error should surface with context")
	})

	t.Run("RunInRegistrationOrder", func(t *testing.T) {
		server := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {})

		var order []string

		appendStep := func(step string) func(*httpx.Request) error {
			return func(*httpx.Request) error {
				order = append(order, step)

				return nil
			}
		}

		client := newClient(t,
			httpx.WithRequestHook(appendStep("first"), appendStep("second")),
			httpx.WithResponseHook(func(*httpx.Response) error {
				order = append(order, "response")

				return nil
			}),
		)

		_, err := client.NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, []string{"first", "second", "response"}, order, "Hooks should run in registration order")
	})
}

// TestTransportSeam tests that WithTransport replaces the wire layer.
func TestTransportSeam(t *testing.T) {
	client := newClient(t, httpx.WithTransport(&StubTransport{status: http.StatusTeapot, body: "stubbed"}))

	resp, err := client.NewRequest().Get(t.Context(), "http://stub.invalid/anything")
	require.NoError(t, err, "Request should succeed")
	assert.Equal(t, http.StatusTeapot, resp.StatusCode(), "The stub transport should provide the status")
	assert.Equal(t, "stubbed", resp.String(), "The stub transport should provide the body")
}
