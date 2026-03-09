package middleware

import (
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/muesli/termenv"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/middleware"
	"github.com/coldsmirk/vef-framework-go/result"
)

var (
	output              = termenv.DefaultOutput()
	labelValueSeparator = " | "
	ipLabel             = output.String("ip: ").Foreground(termenv.ANSIBrightBlack).String()
	uaLabel             = output.String("ua: ").Foreground(termenv.ANSIBrightBlack).String()
	latencyLabel        = output.String("latency: ").Foreground(termenv.ANSIBrightBlack).String()
	statusLabel         = output.String("status: ").Foreground(termenv.ANSIBrightBlack).String()
)

// simplifyUserAgent reduces verbose UA strings to concise "Client/OS" format for log readability.
func simplifyUserAgent(ua string) string {
	if ua == "" {
		return "Unknown"
	}

	ua = strings.ToLower(ua)

	os := detectOS(ua)
	client := detectClient(ua)

	if client != "" && os != "Unknown" {
		return client + "/" + os
	}

	if client != "" {
		return client
	}

	return os
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iOS"
	case strings.Contains(ua, "mac os x") || strings.Contains(ua, "macintosh"):
		return "Mac"
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

func detectClient(ua string) string {
	switch {
	case strings.Contains(ua, "micromessenger"):
		return "WeChat"
	case strings.Contains(ua, "dingtalk"):
		return "DingTalk"
	case strings.Contains(ua, "alipay"):
		return "Alipay"
	case strings.Contains(ua, "edg/") || strings.Contains(ua, "edge/"):
		return "Edge"
	case strings.Contains(ua, "chrome/") && !strings.Contains(ua, "edg"):
		return "Chrome"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome"):
		return "Safari"
	case strings.Contains(ua, "firefox/"):
		return "Firefox"
	case strings.Contains(ua, "postman"):
		return "Postman"
	case strings.Contains(ua, "curl"):
		return "cURL"
	case strings.Contains(ua, "okhttp"):
		return "OkHttp"
	default:
		return ""
	}
}

func isSPAStaticRequest(ctx fiber.Ctx, spaConfigs []*middleware.SPAConfig) bool {
	if ctx.Method() != fiber.MethodGet {
		return false
	}

	reqPath := ctx.Path()
	for _, config := range spaConfigs {
		spaPath := config.Path
		if spaPath == "" {
			spaPath = "/"
		}

		staticPath := path.Join(spaPath, "static/")
		if reqPath == spaPath || strings.HasPrefix(reqPath, staticPath) {
			return true
		}
	}

	return false
}

func formatLatency(ms int64, latencyStr string) string {
	switch {
	case ms >= 1000:
		return output.String(latencyStr).Foreground(termenv.ANSIBrightRed).Bold().String()
	case ms >= 500:
		return output.String(latencyStr).Foreground(termenv.ANSIBrightYellow).Bold().String()
	case ms >= 200:
		return output.String(latencyStr).Foreground(termenv.ANSIBrightBlue).String()
	default:
		return output.String(latencyStr).Foreground(termenv.ANSIBrightGreen).String()
	}
}

func formatStatus(status int) string {
	color := termenv.ANSIBrightRed
	if status >= 200 && status < 300 {
		color = termenv.ANSIBrightGreen
	}

	return output.String(cast.ToString(status)).Foreground(color).String()
}

func formatRequestDetails(ctx fiber.Ctx, data *logger.Data) string {
	method, reqPath := ctx.Method(), ctx.Path()
	ip, latency, status := httpx.GetIP(ctx), data.Stop.Sub(data.Start), ctx.Response().StatusCode()
	ua := simplifyUserAgent(ctx.Get(fiber.HeaderUserAgent))
	ms := latency.Milliseconds()

	var latencyStr string
	if ms > 0 {
		latencyStr = cast.ToString(ms) + "ms"
	} else {
		latencyStr = cast.ToString(latency.Microseconds()) + "μs"
	}

	var sb strings.Builder
	sb.WriteString(labelValueSeparator)
	sb.WriteString(output.String(method).Foreground(termenv.ANSIBrightCyan).String())
	sb.WriteString(" ")
	sb.WriteString(output.String(reqPath).Foreground(termenv.ANSIBrightCyan).String())

	sb.WriteString(labelValueSeparator)
	sb.WriteString(ipLabel)
	sb.WriteString(output.String(ip).Foreground(termenv.ANSIBrightCyan).String())

	sb.WriteString(labelValueSeparator)
	sb.WriteString(uaLabel)
	sb.WriteString(output.String(ua).Foreground(termenv.ANSIBrightCyan).String())

	sb.WriteString(labelValueSeparator)
	sb.WriteString(latencyLabel)
	sb.WriteString(formatLatency(ms, latencyStr))

	sb.WriteString(labelValueSeparator)
	sb.WriteString(statusLabel)
	sb.WriteString(formatStatus(status))

	return sb.String()
}

func logRequest(ctx fiber.Ctx, data *logger.Data) {
	details := formatRequestDetails(ctx, data)
	logger := contextx.Logger(ctx)

	if data.ChainErr == nil {
		logger.Infof(
			"%s%s",
			output.String("Request completed").Foreground(termenv.ANSIBrightGreen).String(),
			details,
		)

		return
	}

	if err, ok := result.AsErr(data.ChainErr); ok {
		msg := "Request completed with error: " + data.ChainErr.Error() +
			"(" + cast.ToString(err.Code) + ")"
		logger.Warnf(
			"%s%s",
			output.String(msg).Foreground(termenv.ANSIBrightYellow).String(),
			details,
		)

		return
	}

	msg := "Request failed with error: " + data.ChainErr.Error()
	logger.Errorf(
		"%s%s",
		output.String(msg).Foreground(termenv.ANSIBrightRed).String(),
		details,
	)
}

// NewRequestRecordMiddleware skips SPA static assets to reduce log noise while capturing API traffic.
func NewRequestRecordMiddleware(spaConfigs []*middleware.SPAConfig) app.Middleware {
	handler := logger.New(logger.Config{
		Next: func(ctx fiber.Ctx) bool {
			return isSPAStaticRequest(ctx, spaConfigs)
		},
		LoggerFunc: func(ctx fiber.Ctx, data *logger.Data, _ *logger.Config) error {
			logRequest(ctx, data)

			return nil
		},
	})

	return &SimpleMiddleware{
		handler: handler,
		name:    "request_record",
		order:   -100,
	}
}
