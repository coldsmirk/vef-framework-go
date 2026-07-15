package exec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
)

// alwaysMaskedHeaders are credential-bearing headers masked in every capture
// regardless of configuration.
var alwaysMaskedHeaders = collections.NewHashSetFrom(
	"authorization",
	"proxy-authorization",
	"cookie",
	"set-cookie",
)

// defaultMaskedFields are JSON field and query parameter names masked in
// captures on top of vef.integration.log.mask_fields.
var defaultMaskedFields = []string{"password", "token", "secret"}

// traceKey carries the per-invocation trace collector through the request
// context to the scoped client's hooks.
type traceKey struct{}

// withTrace attaches the collector to ctx.
func withTrace(ctx context.Context, tc *traceCollector) context.Context {
	return context.WithValue(ctx, traceKey{}, tc)
}

// traceFrom returns the collector attached to ctx, or nil.
func traceFrom(ctx context.Context) *traceCollector {
	tc, _ := ctx.Value(traceKey{}).(*traceCollector)

	return tc
}

// traceCollector accumulates the wire exchanges of one invocation, masking
// and truncating them as they arrive. It serves both the invocation log and
// the dry-run trace.
type traceCollector struct {
	capturer *capturer

	mu        sync.Mutex
	exchanges []integration.HTTPExchange
}

func newTraceCollector(capturer *capturer) *traceCollector {
	return &traceCollector{capturer: capturer}
}

// record captures one exchange; safe for concurrent use.
func (t *traceCollector) record(exchange integration.HTTPExchange) {
	exchange.URL = t.capturer.maskURL(exchange.URL)
	exchange.RequestHeaders = t.capturer.maskHeaderMap(exchange.RequestHeaders)
	exchange.ResponseHeaders = t.capturer.maskHeaderMap(exchange.ResponseHeaders)
	exchange.RequestBody = t.capturer.captureBody(exchange.RequestBody)
	exchange.ResponseBody = t.capturer.captureBody(exchange.ResponseBody)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.exchanges = append(t.exchanges, exchange)
}

// Exchanges returns the captured exchanges in arrival order.
func (t *traceCollector) Exchanges() []integration.HTTPExchange {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.exchanges
}

// capturer applies the vef.integration.log masking and truncation rules to
// captured payloads. It is immutable and shared across invocations.
type capturer struct {
	limit      int
	maskFields collections.Set[string]
}

func newCapturer(cfg *config.IntegrationLogConfig) *capturer {
	fields := make([]string, 0, len(defaultMaskedFields)+len(cfg.MaskFields))
	fields = append(fields, defaultMaskedFields...)

	for _, field := range cfg.MaskFields {
		fields = append(fields, strings.ToLower(field))
	}

	return &capturer{
		limit:      cfg.EffectiveCaptureLimit(),
		maskFields: collections.NewHashSetFrom(fields...),
	}
}

// captureValue masks and truncates an arbitrary JSON-shaped value into its
// stored capture form.
func (c *capturer) captureValue(value any) json.RawMessage {
	if value == nil {
		return nil
	}

	data, err := json.Marshal(c.maskValue(value))
	if err != nil {
		return nil
	}

	if len(data) > c.limit {
		truncated, _ := json.Marshal(string(data[:c.limit]) + "…(truncated)")

		return truncated
	}

	return data
}

// captureBody masks and truncates a raw body string: JSON bodies get
// field-level masking, everything else is truncated verbatim.
func (c *capturer) captureBody(body string) string {
	if body == "" {
		return ""
	}

	var value any
	if err := json.Unmarshal([]byte(body), &value); err == nil {
		if data, err := json.Marshal(c.maskValue(value)); err == nil {
			body = string(data)
		}
	}

	if len(body) > c.limit {
		return body[:c.limit] + "…(truncated)"
	}

	return body
}

// maskValue walks a JSON-shaped value, replacing the values of masked field
// names (case-insensitive).
func (c *capturer) maskValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		masked := make(map[string]any, len(v))
		for key, item := range v {
			if c.maskFields.Contains(strings.ToLower(key)) {
				masked[key] = integration.MaskedSecret
			} else {
				masked[key] = c.maskValue(item)
			}
		}

		return masked

	case []any:
		masked := make([]any, len(v))
		for i, item := range v {
			masked[i] = c.maskValue(item)
		}

		return masked

	default:
		return value
	}
}

// maskHeaderMap lower-cases header names and masks credential values.
func (c *capturer) maskHeaderMap(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	masked := make(map[string]string, len(headers))

	for name, value := range headers {
		lower := strings.ToLower(name)
		if alwaysMaskedHeaders.Contains(lower) || c.maskFields.Contains(lower) {
			value = integration.MaskedSecret
		}

		masked[lower] = value
	}

	return masked
}

// maskURL masks the values of masked query parameters, keeping the rest of
// the URL intact.
func (c *capturer) maskURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
		return rawURL
	}

	query := parsed.Query()
	changed := false

	for name := range query {
		if c.maskFields.Contains(strings.ToLower(name)) {
			query.Set(name, integration.MaskedSecret)

			changed = true
		}
	}

	if !changed {
		return rawURL
	}

	parsed.RawQuery = query.Encode()

	return parsed.String()
}

// flattenHeader collapses an http.Header into a single-valued map for
// capture.
func flattenHeader(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}

	flat := make(map[string]string, len(header))
	for name, values := range header {
		flat[name] = strings.Join(values, ", ")
	}

	return flat
}
