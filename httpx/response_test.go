package httpx_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/httpx"
)

// TestResponse tests the buffered response accessors.
func TestResponse(t *testing.T) {
	t.Run("SuccessFields", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Res", "yes")
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "1"})
			_, _ = w.Write([]byte("hello"))
		})

		resp, err := newClient(t).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")

		assert.Equal(t, http.StatusOK, resp.StatusCode(), "StatusCode should report the code")
		assert.Equal(t, "200 OK", resp.Status(), "Status should report the full status line")
		assert.True(t, resp.IsSuccess(), "A 200 response should be a success")
		assert.Equal(t, "yes", resp.Header("X-Res"), "Header should return the named value")
		assert.Equal(t, "yes", resp.Headers().Get("X-Res"), "Headers should expose the full set")
		require.Len(t, resp.Cookies(), 1, "The set cookie should be exposed")
		assert.Equal(t, "sid", resp.Cookies()[0].Name, "The cookie name should round-trip")
		assert.Equal(t, []byte("hello"), resp.Body(), "Body should return the buffered bytes")
		assert.Equal(t, "hello", resp.String(), "String should return the body as text")
		assert.Positive(t, resp.Duration(), "Duration should be measured")
		assert.Equal(t, 1, resp.Attempts(), "A plain call should record one attempt")
	})

	t.Run("NotSuccess", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		resp, err := newClient(t).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "A non-2xx response is not an error")
		assert.False(t, resp.IsSuccess(), "A 404 response should not be a success")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), "StatusCode should report the code")
	})

	t.Run("JSON", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"name":"vef"}`))
		})

		resp, err := newClient(t).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")

		var out struct {
			Name string `json:"name"`
		}

		require.NoError(t, resp.JSON(&out), "JSON should unmarshal the body")
		assert.Equal(t, "vef", out.Name, "The decoded value should round-trip")
	})

	t.Run("XML", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`<XMLPayload><name>vef</name></XMLPayload>`))
		})

		resp, err := newClient(t).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")

		var out XMLPayload

		require.NoError(t, resp.XML(&out), "XML should unmarshal the body")
		assert.Equal(t, "vef", out.Name, "The decoded value should round-trip")
	})
}

// TestMaxResponseBody tests the buffered-body size cap.
func TestMaxResponseBody(t *testing.T) {
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("12345678"))
	})

	t.Run("AtCap", func(t *testing.T) {
		resp, err := newClient(t, httpx.WithMaxResponseBody(8)).NewRequest().Get(t.Context(), server.URL)
		require.NoError(t, err, "A body exactly at the cap should pass")
		assert.Equal(t, "12345678", resp.String(), "The body should be fully buffered")
	})

	t.Run("OverCap", func(t *testing.T) {
		_, err := newClient(t, httpx.WithMaxResponseBody(7)).NewRequest().Get(t.Context(), server.URL)
		require.ErrorIs(t, err, httpx.ErrResponseTooLarge, "A body over the cap should fail the call")
	})
}
