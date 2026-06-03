package middleware

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/middleware"
)

func TestSimplifyUserAgent(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{name: "Empty", ua: "", want: "Unknown"},
		{name: "ClientAndOS", ua: "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0", want: "Chrome/Windows"},
		{name: "ClientOnly", ua: "PostmanRuntime/7.36.0", want: "Postman"},
		{name: "OSOnly", ua: "Mozilla/5.0 (Linux x86_64)", want: "Linux"},
		{name: "UnknownBoth", ua: "some-random-agent", want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, simplifyUserAgent(tt.ua), "simplifyUserAgent should produce the concise Client/OS form")
		})
	}
}

func TestDetectOS(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{name: "Android", ua: "mozilla/5.0 (linux; android 13)", want: "Android"},
		{name: "IPhone", ua: "mozilla/5.0 (iphone; cpu iphone os 17_0)", want: "iOS"},
		{name: "IPad", ua: "mozilla/5.0 (ipad; cpu os 17_0)", want: "iOS"},
		{name: "Mac", ua: "mozilla/5.0 (macintosh; intel mac os x 10_15)", want: "Mac"},
		{name: "Windows", ua: "mozilla/5.0 (windows nt 10.0)", want: "Windows"},
		{name: "Linux", ua: "mozilla/5.0 (x11; linux x86_64)", want: "Linux"},
		{name: "AndroidWinsOverLinux", ua: "mozilla/5.0 (linux; android 13)", want: "Android"},
		{name: "Unknown", ua: "custom-agent", want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectOS(tt.ua), "detectOS should map the UA token to the expected OS")
		})
	}
}

func TestDetectClient(t *testing.T) {
	tests := []struct {
		name string
		ua   string
		want string
	}{
		{name: "WeChat", ua: "micromessenger/8.0", want: "WeChat"},
		{name: "DingTalk", ua: "dingtalk/6.5", want: "DingTalk"},
		{name: "Alipay", ua: "alipayclient/10.3", want: "Alipay"},
		{name: "EdgeShort", ua: "chrome/120.0 edg/120.0", want: "Edge"},
		{name: "EdgeLegacy", ua: "edge/18.0", want: "Edge"},
		{name: "ChromeNotEdge", ua: "chrome/120.0 safari/537.36", want: "Chrome"},
		{name: "SafariNotChrome", ua: "version/17.0 safari/605.1", want: "Safari"},
		{name: "Firefox", ua: "firefox/121.0", want: "Firefox"},
		{name: "Postman", ua: "postmanruntime/7.36", want: "Postman"},
		{name: "Curl", ua: "curl/8.4.0", want: "cURL"},
		{name: "OkHttp", ua: "okhttp/4.12.0", want: "OkHttp"},
		{name: "Unknown", ua: "mozilla/5.0", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectClient(tt.ua), "detectClient must apply the Chrome/Edge and Safari/Chrome exclusions correctly")
		})
	}
}

func TestFormatLatency(t *testing.T) {
	// Color codes depend on TTY detection, so assert the latency text is always
	// present regardless of the active termenv color profile and threshold tier.
	for _, ms := range []int64{10, 200, 500, 1000} {
		t.Run(strconv.FormatInt(ms, 10)+"ms", func(t *testing.T) {
			const marker = "marker-latency"
			assert.Contains(t, formatLatency(ms, marker), marker, "formatLatency must preserve the latency text across all thresholds")
		})
	}
}

func TestFormatStatus(t *testing.T) {
	for _, status := range []int{199, 200, 201, 300, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			assert.Contains(t, formatStatus(status), strconv.Itoa(status), "formatStatus must render the numeric status code")
		})
	}
}

func TestIsSPAStaticRequest(t *testing.T) {
	tests := []struct {
		name    string
		configs []*middleware.SPAConfig
		method  string
		path    string
		want    bool
	}{
		{
			name:    "RootEntry",
			configs: []*middleware.SPAConfig{{Path: "/"}},
			method:  fiber.MethodGet,
			path:    "/",
			want:    true,
		},
		{
			name:    "RootStaticAsset",
			configs: []*middleware.SPAConfig{{Path: "/"}},
			method:  fiber.MethodGet,
			path:    "/static/app.js",
			want:    true,
		},
		{
			name:    "EmptyPathDefaultsToRootStatic",
			configs: []*middleware.SPAConfig{{Path: ""}},
			method:  fiber.MethodGet,
			path:    "/static/app.js",
			want:    true,
		},
		{
			name:    "NonStaticIsLogged",
			configs: []*middleware.SPAConfig{{Path: "/"}},
			method:  fiber.MethodGet,
			path:    "/api/users",
			want:    false,
		},
		{
			name:    "StaticPrefixCollisionNotMatched",
			configs: []*middleware.SPAConfig{{Path: "/"}},
			method:  fiber.MethodGet,
			path:    "/staticfoo",
			want:    false,
		},
		{
			name:    "NestedMountStatic",
			configs: []*middleware.SPAConfig{{Path: "/app"}},
			method:  fiber.MethodGet,
			path:    "/app/static/app.js",
			want:    true,
		},
		{
			name:    "PostIsNeverStatic",
			configs: []*middleware.SPAConfig{{Path: "/"}},
			method:  fiber.MethodPost,
			path:    "/static/app.js",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()

			var got bool
			app.Add([]string{tt.method}, "/*", func(c fiber.Ctx) error {
				got = isSPAStaticRequest(c, tt.configs)

				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err, "request should complete without error")
			require.Equal(t, fiber.StatusOK, resp.StatusCode, "harness route should return OK")
			assert.Equal(t, tt.want, got, "isSPAStaticRequest must skip only SPA entry and static-asset GETs")
		})
	}
}

func TestFormatRequestDetails(t *testing.T) {
	app := fiber.New()

	var details string
	app.Get("/widgets", func(c fiber.Ctx) error {
		data := &logger.Data{
			Start: time.Unix(0, 0),
			Stop:  time.Unix(0, 0).Add(250 * time.Millisecond),
		}
		details = formatRequestDetails(c, data)

		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodGet, "/widgets", nil)
	req.Header.Set(fiber.HeaderUserAgent, "curl/8.4.0")
	resp, err := app.Test(req)
	require.NoError(t, err, "request should complete without error")
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "harness route should return OK")

	assert.Contains(t, details, fiber.MethodGet, "details should include the request method")
	assert.Contains(t, details, "/widgets", "details should include the request path")
	assert.Contains(t, details, "250ms", "details should render the computed latency")
	assert.Contains(t, details, "cURL", "details should include the simplified user agent")
	assert.Contains(t, details, strconv.Itoa(fiber.StatusOK), "details should include the response status code")
}
