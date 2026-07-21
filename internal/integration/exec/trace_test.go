package exec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
)

func TestCapturer(t *testing.T) {
	capturer := newCapturer(&config.IntegrationLogConfig{MaskFields: []string{"idCardNo"}})

	t.Run("MasksConfiguredAndDefaultJSONFields", func(t *testing.T) {
		body := capturer.captureBody(`{"name":"He","idCardNo":"110101199001010011","password":"p","nested":{"token":"t"}}`)

		assert.NotContains(t, body, "110101199001010011", "Configured field should be masked")
		assert.NotContains(t, body, `"p"`, "Default-masked password should be hidden")
		assert.NotContains(t, body, `"t"`, "Nested default-masked token should be hidden")
		assert.Contains(t, body, "He", "Unmasked fields should stay visible")
	})

	t.Run("NonJSONBodyPassesThrough", func(t *testing.T) {
		assert.Equal(t, "<xml/>", capturer.captureBody("<xml/>"), "Non-JSON bodies should pass through unmasked")
	})

	t.Run("TruncatesOversizedBody", func(t *testing.T) {
		long := strings.Repeat("x", 5000)

		captured := capturer.captureBody(long)
		assert.Less(t, len(captured), 5000, "Oversized body should be truncated")
		assert.Contains(t, captured, "truncated", "Truncation should be visible")
	})

	t.Run("MasksCredentialHeaders", func(t *testing.T) {
		masked := capturer.maskHeaderMap(map[string]string{
			"Authorization": "Bearer secret",
			"Content-Type":  "application/json",
		})

		assert.Equal(t, integration.MaskedSecret, masked["authorization"], "Authorization should always be masked")
		assert.Equal(t, "application/json", masked["content-type"], "Regular headers should stay visible")
	})

	t.Run("MasksQueryParams", func(t *testing.T) {
		masked := capturer.maskURL("https://api.example.com/q?token=abc&page=2")

		assert.NotContains(t, masked, "abc", "Masked query parameter value should be hidden")
		assert.Contains(t, masked, "page=2", "Other query parameters should stay visible")
	})

	t.Run("CaptureValueMasksAndSerializes", func(t *testing.T) {
		captured := capturer.captureValue(map[string]any{"password": "p", "kept": "v"})

		assert.NotContains(t, string(captured), `"p"`, "Sensitive value should be masked")
		assert.Contains(t, string(captured), `"v"`, "Regular value should be captured")
	})

	t.Run("CaptureValueNilYieldsNil", func(t *testing.T) {
		assert.Nil(t, capturer.captureValue(nil), "Nil value should capture as nil")
	})
}

func TestTraceCollector(t *testing.T) {
	capturer := newCapturer(new(config.IntegrationLogConfig))
	collector := newTraceCollector(capturer, nil)

	collector.record(integration.HTTPExchange{
		Method:         "POST",
		URL:            "/api",
		RequestHeaders: map[string]string{"Authorization": "Bearer x"},
		RequestBody:    `{"password":"p"}`,
		Status:         200,
	})

	exchanges := collector.Exchanges()
	require.Len(t, exchanges, 1, "Recorded exchange should be returned")
	assert.Equal(t, integration.MaskedSecret, exchanges[0].RequestHeaders["authorization"],
		"Recorded headers should arrive masked")
	assert.NotContains(t, exchanges[0].RequestBody, `"p"`, "Recorded body should arrive masked")
}

func TestTraceRedaction(t *testing.T) {
	capturer := newCapturer(new(config.IntegrationLogConfig))
	// A blank redaction value must be skipped — replacing "" would corrupt the
	// whole string; the real credential is scrubbed by value under any name.
	collector := newTraceCollector(capturer, []string{"s3cr3t-value", ""})

	collector.record(integration.HTTPExchange{
		Method:         "POST",
		URL:            "https://vendor/api?appkey=s3cr3t-value&page=1",
		RequestHeaders: map[string]string{"X-App-Secret": "s3cr3t-value", "X-Trace": "keep-me"},
		RequestBody:    `{"credential":"s3cr3t-value"}`,
		// net/http reports a transport failure as a *url.Error whose message
		// embeds the whole URL, query string included.
		Error: `Post "https://vendor/api?appkey=s3cr3t-value&page=1": dial tcp 10.0.0.1:443: connect: connection refused`,
	})

	exchange := collector.Exchanges()[0]
	assert.NotContains(t, exchange.URL, "s3cr3t-value", "The credential value must be scrubbed from the query, whatever the param name")
	assert.Equal(t, integration.MaskedSecret, exchange.RequestHeaders["x-app-secret"], "A credential header the name mask cannot know is scrubbed by value")
	assert.Equal(t, "keep-me", exchange.RequestHeaders["x-trace"], "A non-credential header value is left intact")
	assert.NotContains(t, exchange.RequestBody, "s3cr3t-value", "The credential value must be scrubbed from the body")
	assert.NotContains(t, exchange.Error, "s3cr3t-value", "The credential value must be scrubbed from the transport error message")
	assert.Contains(t, exchange.Error, "connection refused", "The diagnostic part of the transport error must survive scrubbing")
}

// TestTraceRedactionPrecedesCapture pins the ordering inside record: the
// credential scrub runs on the raw capture, ahead of every transform that
// rewrites or drops part of it.
func TestTraceRedactionPrecedesCapture(t *testing.T) {
	t.Run("SurvivesTheTruncationBoundary", func(t *testing.T) {
		const (
			secret = "s3cr3t-value"
			limit  = 24
			filler = 20
		)

		collector := newTraceCollector(newCapturer(&config.IntegrationLogConfig{CaptureLimit: limit}), []string{secret})

		// The credential straddles the capture limit, so truncating first
		// would leave its head behind with nothing for the value scrub to
		// match.
		straddling := strings.Repeat("x", filler) + secret + strings.Repeat("y", filler)
		collector.record(integration.HTTPExchange{RequestBody: straddling, Error: straddling})

		head := secret[:limit-filler]

		exchange := collector.Exchanges()[0]
		assert.NotContains(t, exchange.RequestBody, head, "No head of the credential may survive truncation in the body")
		assert.NotContains(t, exchange.Error, head, "No head of the credential may survive truncation in the error")
	})

	t.Run("SurvivesTheBodyJSONRoundTrip", func(t *testing.T) {
		// Go's JSON encoder escapes & < > as \uXXXX, so a credential carrying
		// one no longer matches its own literal once the body capture has
		// re-marshaled it.
		const secret = "a&b<c"

		collector := newTraceCollector(newCapturer(new(config.IntegrationLogConfig)), []string{secret})
		collector.record(integration.HTTPExchange{RequestBody: `{"credential":"` + secret + `"}`})

		captured := collector.Exchanges()[0].RequestBody
		assert.NotContains(t, captured, secret, "The credential must not survive the body capture")
		assert.NotContains(t, captured, `a\u0026b\u003cc`,
			"The credential must not survive as the escaped form the round-trip produces")
		assert.Contains(t, captured, integration.MaskedSecret, "The credential's position should carry the mask")
	})
}

func TestTraceErrorCapture(t *testing.T) {
	capture := func(message string) string {
		collector := newTraceCollector(newCapturer(&config.IntegrationLogConfig{CaptureLimit: 32}), nil)
		collector.record(integration.HTTPExchange{Error: message})

		return collector.Exchanges()[0].Error
	}

	t.Run("BoundsOversizedError", func(t *testing.T) {
		captured := capture(strings.Repeat("e", 200))

		assert.Less(t, len(captured), 200, "An oversized error message should be bounded by the capture limit")
		assert.Contains(t, captured, "truncated", "Truncation should be visible")
	})

	t.Run("KeepsErrorTextVerbatim", func(t *testing.T) {
		// A message that happens to parse as JSON must not be run through the
		// body capture, which would re-marshal it as a payload.
		assert.Equal(t, `{"b": 1, "a": 2}`, capture(`{"b": 1, "a": 2}`),
			"An error message is diagnostic prose and must be captured verbatim")
	})
}
