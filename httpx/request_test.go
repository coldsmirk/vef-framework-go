package httpx_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/httpx"
)

// XMLPayload is the XML body fixture; the element name derives from the type
// name.
type XMLPayload struct {
	Name string `xml:"name"`
}

// TestHeaders tests header building semantics.
func TestHeaders(t *testing.T) {
	var header http.Header

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		header = r.Header.Clone()
	})

	_, err := newClient(t).NewRequest().
		SetHeader("X-One", "1").
		AddHeader("X-Multi", "a").
		AddHeader("X-Multi", "b").
		SetHeaders(map[string]string{"X-Two": "2"}).
		Get(t.Context(), server.URL)
	require.NoError(t, err, "Request should succeed")
	assert.Equal(t, "1", header.Get("X-One"), "SetHeader should send the value")
	assert.Equal(t, []string{"a", "b"}, header.Values("X-Multi"), "AddHeader should keep every added value")
	assert.Equal(t, "2", header.Get("X-Two"), "SetHeaders should set each entry")
}

// TestQueryParams tests query merging across the URL string and the builder.
func TestQueryParams(t *testing.T) {
	var query map[string][]string

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
	})

	_, err := newClient(t).NewRequest().
		SetQuery("dup", "new").
		AddQuery("multi", "a").
		AddQuery("multi", "b").
		SetQueries(map[string]string{"extra": "3"}).
		Get(t.Context(), server.URL+"/path?carried=1&dup=old")
	require.NoError(t, err, "Request should succeed")
	assert.Equal(t, []string{"1"}, query["carried"], "URL-carried parameters should survive")
	assert.Equal(t, []string{"new"}, query["dup"], "SetQuery should replace a URL-carried value")
	assert.Equal(t, []string{"a", "b"}, query["multi"], "AddQuery should keep every added value")
	assert.Equal(t, []string{"3"}, query["extra"], "SetQueries should set each entry")
}

// TestPathParams tests ":name" template substitution.
func TestPathParams(t *testing.T) {
	var escapedPath string

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		escapedPath = r.URL.EscapedPath()
	})

	t.Run("Substitutes", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetPathParam("id", "42").
			SetPathParams(map[string]string{"name": "report"}).
			Get(t.Context(), server.URL+"/users/:id/files/:name")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "/users/42/files/report", escapedPath, "Every template segment should be substituted")
	})

	t.Run("EscapesValues", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetPathParam("name", "a b/c").
			Get(t.Context(), server.URL+"/files/:name")
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "/files/a%20b%2Fc", escapedPath, "Substituted values should be URL-escaped, slashes included")
	})

	t.Run("MissingValue", func(t *testing.T) {
		var hits atomic.Int32

		counting := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
		})

		_, err := newClient(t).NewRequest().Get(t.Context(), counting.URL+"/users/:id")
		require.ErrorIs(t, err, httpx.ErrMissingPathParam, "An unresolved template segment should fail the call")
		assert.Zero(t, hits.Load(), "The call should fail before reaching the wire")
	})

	t.Run("UnusedValueIgnored", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetPathParam("ghost", "x").
			Get(t.Context(), server.URL+"/plain")
		require.NoError(t, err, "A parameter matching no segment should be ignored")
	})
}

// TestCookies tests cookie building semantics.
func TestCookies(t *testing.T) {
	var cookies []*http.Cookie

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		cookies = r.Cookies()
	})

	_, err := newClient(t).NewRequest().
		SetCookie("session", "one").
		SetCookie("session", "two").
		SetCookie("other", "x").
		Get(t.Context(), server.URL)
	require.NoError(t, err, "Request should succeed")
	require.Len(t, cookies, 2, "A same-named cookie should be replaced, not duplicated")
	assert.Equal(t, "two", cookies[0].Value, "The last SetCookie value should win")
	assert.Equal(t, "other", cookies[1].Name, "Distinct cookies should all be sent")
}

// TestAuth tests request-level authentication setters.
func TestAuth(t *testing.T) {
	var authorization string

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
	})

	t.Run("BasicAuth", func(t *testing.T) {
		var username, password string

		basic := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			username, password, _ = r.BasicAuth()
		})

		_, err := newClient(t).NewRequest().SetBasicAuth("user", "secret").Get(t.Context(), basic.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "user", username, "The basic auth username should round-trip")
		assert.Equal(t, "secret", password, "The basic auth password should round-trip")
	})

	t.Run("LastSetterWins", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetBasicAuth("user", "secret").
			SetBearerToken("token").
			Get(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "Bearer token", authorization, "The last authentication setter should win")
	})
}

// TestBodies tests every body representation and their interplay.
func TestBodies(t *testing.T) {
	var (
		contentType string
		received    []byte
	)

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		received, _ = io.ReadAll(r.Body)
	})

	t.Run("JSON", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetJSON(map[string]string{"name": "vef"}).
			Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "application/json", contentType, "SetJSON should set the content type")
		assert.JSONEq(t, `{"name":"vef"}`, string(received), "SetJSON should marshal the value")
	})

	t.Run("JSONMarshalError", func(t *testing.T) {
		var hits atomic.Int32

		counting := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
		})

		_, err := newClient(t).NewRequest().SetJSON(make(chan int)).Post(t.Context(), counting.URL)
		require.ErrorContains(t, err, "marshal JSON body", "A marshal failure should surface at the terminal method")
		assert.Zero(t, hits.Load(), "A builder error should abort before the wire")
	})

	t.Run("XML", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetXML(XMLPayload{Name: "vef"}).
			Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "application/xml", contentType, "SetXML should set the content type")
		assert.Contains(t, string(received), "<name>vef</name>", "SetXML should marshal the value")
	})

	t.Run("FormURLEncoded", func(t *testing.T) {
		var form map[string][]string

		formServer := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			contentType = r.Header.Get("Content-Type")
			require.NoError(t, r.ParseForm(), "The server should parse the form")
			form = r.PostForm
		})

		_, err := newClient(t).NewRequest().
			SetForm(map[string]string{"a": "1"}).
			AddFormField("tag", "x").
			AddFormField("tag", "y").
			Post(t.Context(), formServer.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "application/x-www-form-urlencoded", contentType, "Fields alone should encode as urlencoded")
		assert.Equal(t, []string{"1"}, form["a"], "SetForm fields should round-trip")
		assert.Equal(t, []string{"x", "y"}, form["tag"], "AddFormField should keep repeated values")
	})

	t.Run("MultipartWithFiles", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "doc.txt")
		require.NoError(t, os.WriteFile(path, []byte("file-content"), 0o600), "The fixture file should be written")

		var (
			fieldValue    string
			fileNames     map[string]string
			fileContents  map[string]string
			multipartType string
		)

		multipartServer := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
			multipartType = r.Header.Get("Content-Type")
			require.NoError(t, r.ParseMultipartForm(1<<20), "The server should parse the multipart body")

			fieldValue = r.FormValue("field")
			fileNames = make(map[string]string)
			fileContents = make(map[string]string)

			for name, headers := range r.MultipartForm.File {
				fileNames[name] = headers[0].Filename

				opened, err := headers[0].Open()
				require.NoError(t, err, "The uploaded part should open")

				data, err := io.ReadAll(opened)
				require.NoError(t, err, "The uploaded part should be readable")
				require.NoError(t, opened.Close(), "The uploaded part should close")

				fileContents[name] = string(data)
			}
		})

		_, err := newClient(t).NewRequest().
			SetForm(map[string]string{"field": "value"}).
			AddFile("doc", path).
			AddFileReader("blob", "b.bin", strings.NewReader("reader-content")).
			Post(t.Context(), multipartServer.URL)
		require.NoError(t, err, "Request should succeed")
		assert.True(t, strings.HasPrefix(multipartType, "multipart/form-data"), "Attaching files should upgrade the body to multipart")
		assert.Equal(t, "value", fieldValue, "Form fields should ride along as multipart parts")
		assert.Equal(t, "doc.txt", fileNames["doc"], "AddFile should use the path's base name")
		assert.Equal(t, "file-content", fileContents["doc"], "AddFile should stream the file content")
		assert.Equal(t, "b.bin", fileNames["blob"], "AddFileReader should use the given file name")
		assert.Equal(t, "reader-content", fileContents["blob"], "AddFileReader should stream the reader content")
	})

	t.Run("RawBody", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetBody([]byte("raw"), "text/plain").
			Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "text/plain", contentType, "SetBody should use the explicit content type")
		assert.Equal(t, "raw", string(received), "SetBody should send the bytes verbatim")
	})

	t.Run("StreamBody", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetBodyReader(strings.NewReader("streamed"), "application/octet-stream").
			Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "streamed", string(received), "SetBodyReader should stream the reader")
	})

	t.Run("LastBodySetterWins", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetJSON(map[string]string{"a": "1"}).
			SetForm(map[string]string{"b": "2"}).
			Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "application/x-www-form-urlencoded", contentType, "The last body setter should win")
	})

	t.Run("ExplicitContentTypeWins", func(t *testing.T) {
		_, err := newClient(t).NewRequest().
			SetJSON(map[string]string{"a": "1"}).
			SetHeader("Content-Type", "application/vnd.custom+json").
			Post(t.Context(), server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "application/vnd.custom+json", contentType, "An explicit Content-Type header should beat the body default")
	})
}

// TestTerminalMethods tests the verb methods and single-use enforcement.
func TestTerminalMethods(t *testing.T) {
	var method string

	server := newTestServer(t, func(_ http.ResponseWriter, r *http.Request) {
		method = r.Method
	})

	verbs := []struct {
		want string
		call func(*httpx.Request, context.Context, string) (*httpx.Response, error)
	}{
		{http.MethodGet, (*httpx.Request).Get},
		{http.MethodPost, (*httpx.Request).Post},
		{http.MethodPut, (*httpx.Request).Put},
		{http.MethodPatch, (*httpx.Request).Patch},
		{http.MethodDelete, (*httpx.Request).Delete},
		{http.MethodHead, (*httpx.Request).Head},
		{http.MethodOptions, (*httpx.Request).Options},
	}

	client := newClient(t)

	for _, verb := range verbs {
		t.Run(verb.want, func(t *testing.T) {
			_, err := verb.call(client.NewRequest(), t.Context(), server.URL)
			require.NoError(t, err, "Request should succeed")
			assert.Equal(t, verb.want, method, "The verb method should send its HTTP method")
		})
	}

	t.Run("CustomMethod", func(t *testing.T) {
		_, err := client.NewRequest().Do(t.Context(), "PURGE", server.URL)
		require.NoError(t, err, "Request should succeed")
		assert.Equal(t, "PURGE", method, "Do should send an arbitrary method")
	})

	t.Run("SecondExecutionFails", func(t *testing.T) {
		request := client.NewRequest()

		_, err := request.Get(t.Context(), server.URL)
		require.NoError(t, err, "The first execution should succeed")

		_, err = request.Get(t.Context(), server.URL)
		require.ErrorIs(t, err, httpx.ErrRequestReused, "A Request must be single-use")
	})
}

// TestRequestGetters tests the finalized-state getters used by hooks.
func TestRequestGetters(t *testing.T) {
	server := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {})

	var (
		hookMethod   string
		hookURL      string
		hookBody     []byte
		hookDeadline bool
	)

	client := newClient(t, httpx.WithRequestHook(func(req *httpx.Request) error {
		hookMethod = req.Method()
		hookURL = req.URL()
		hookBody = req.Body()
		_, hookDeadline = req.Context().Deadline()

		return nil
	}))

	request := client.NewRequest().SetJSON(map[string]string{"k": "v"})

	resp, err := request.Post(t.Context(), server.URL+"/echo?q=1")
	require.NoError(t, err, "Request should succeed")

	assert.Equal(t, http.MethodPost, hookMethod, "The hook should see the final method")
	assert.Equal(t, server.URL+"/echo?q=1", hookURL, "The hook should see the fully resolved URL")
	assert.JSONEq(t, `{"k":"v"}`, string(hookBody), "The hook should see the marshaled body")
	assert.True(t, hookDeadline, "The hook context should carry the call deadline")

	assert.Equal(t, http.MethodPost, request.Method(), "Method should stay readable after execution")
	assert.Same(t, request, resp.Request(), "The response should reference its request")
}
