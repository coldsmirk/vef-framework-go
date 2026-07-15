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
	collector := newTraceCollector(capturer)

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
