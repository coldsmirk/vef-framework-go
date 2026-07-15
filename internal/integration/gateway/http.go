package gateway

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/fiberx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/result"
)

// logger is the integration module's framework logger, following the
// repo-wide package-level convention.
var logger = logx.Named("integration")

// httpPathPrefix anchors the HTTP inbound gateway; the two path parameters
// identify the calling system and the invoked contract.
const httpPathPrefix = "/integration/inbound"

// HTTPGateway is the HTTP protocol adapter of the inbound flow: an
// app.Middleware (the framework's route-assembly hook — MCP precedent) that
// registers the vendor-facing endpoint, translates each request into the
// protocol-neutral envelope, and renders the adapter script's reply. It
// deliberately bypasses the /api dispatch model: vendor replies need raw
// control of status and body, never the standard result envelope.
type HTTPGateway struct {
	receiver *exec.Receiver
	limit    fiber.Handler
}

// NewHTTPGateway creates the HTTP inbound gateway. The rate limiter counts
// per (system, client IP), so one flooding vendor cannot starve the others.
func NewHTTPGateway(receiver *exec.Receiver, cfg *config.IntegrationConfig) app.Middleware {
	return &HTTPGateway{
		receiver: receiver,
		limit: limiter.New(limiter.Config{
			LimiterMiddleware: limiter.SlidingWindow{},
			Max:               cfg.Inbound.RateLimit.EffectiveMax(),
			Expiration:        cfg.Inbound.RateLimit.EffectivePeriod(),
			KeyGenerator: func(ctx fiber.Ctx) string {
				return ctx.Params("systemCode") + ":" + fiberx.GetIP(ctx)
			},
			LimitReached: func(fiber.Ctx) error {
				return result.ErrTooManyRequests
			},
		}),
	}
}

// Name returns the middleware name.
func (*HTTPGateway) Name() string {
	return "integration:inbound"
}

// Order places the gateway after the API engine and before the SPA fallback,
// alongside the MCP endpoint.
func (*HTTPGateway) Order() int {
	return 400
}

// Apply registers the inbound endpoint. It is a real route, not a scan-every-
// request interceptor: non-integration traffic pays nothing for it.
func (g *HTTPGateway) Apply(router fiber.Router) {
	router.Post(httpPathPrefix+"/:systemCode/:contractCode", g.limit, g.handle)
	logger.Infof("Integration inbound endpoint registered at %s/:systemCode/:contractCode", httpPathPrefix)
}

// handle translates the HTTP request into the protocol-neutral envelope,
// hands it to the receiver, and renders the reply. Pipeline errors are
// remapped to vendor-actionable statuses and rendered by the app error
// handler as the framework's standard envelope — adapters that must control
// the vendor-facing error format catch dispatch failures in the script
// instead.
func (g *HTTPGateway) handle(ctx fiber.Ctx) error {
	req := &integration.InboundRequest{
		SystemCode:   ctx.Params("systemCode"),
		ContractCode: ctx.Params("contractCode"),
		Protocol:     "http",
		Method:       ctx.Method(),
		Path:         ctx.Path(),
		Headers:      flattenHeaders(ctx.GetReqHeaders()),
		Query:        ctx.Queries(),
		Body:         ctx.Body(),
		ClientAddr:   fiberx.GetIP(ctx),
	}

	reply, err := g.receiver.Receive(ctx.Context(), req)
	if err != nil {
		return vendorStatus(err)
	}

	return renderReply(ctx, reply)
}

// vendorStatus maps a pipeline failure onto an HTTP status a vendor's retry
// logic can act on. On the API surface business errors deliberately ride
// HTTP 200 with envelope codes, but vendors do not read the envelope — a 200
// for "system not found" would count as a successful delivery. Errors already
// carrying a transport status (401 auth, 429 rate limit, 501 handler) pass
// through; missing or disabled definitions map uniformly to 404 so callers
// cannot probe which piece exists, invalid input maps to 400, and everything
// else is a plain 500.
func vendorStatus(err error) error {
	resultErr, ok := errors.AsType[result.Error](err)
	if !ok || resultErr.Status != fiber.StatusOK {
		return err
	}

	switch {
	case errors.Is(err, integration.ErrSystemNotFound),
		errors.Is(err, integration.ErrSystemDisabled),
		errors.Is(err, integration.ErrContractNotFound),
		errors.Is(err, integration.ErrContractDisabled),
		errors.Is(err, integration.ErrAdapterNotFound),
		errors.Is(err, integration.ErrAdapterDisabled):
		resultErr.Status = fiber.StatusNotFound
	case errors.Is(err, integration.ErrInputInvalid("")):
		resultErr.Status = fiber.StatusBadRequest
	default:
		resultErr.Status = fiber.StatusInternalServerError
	}

	return resultErr
}

// flattenHeaders lowercases the header names and joins multi-value headers,
// per the InboundRequest envelope contract.
func flattenHeaders(headers map[string][]string) map[string]string {
	flat := make(map[string]string, len(headers))
	for name, values := range headers {
		flat[strings.ToLower(name)] = strings.Join(values, ", ")
	}

	return flat
}

// renderReply writes the script's reply as the HTTP response. A map carrying
// any of the envelope keys (status, headers, body) is treated as the response
// envelope; every other value — including a plain map — is sent verbatim as a
// 200 JSON body.
func renderReply(ctx fiber.Ctx, reply any) error {
	envelope, ok := reply.(map[string]any)
	if !ok || !hasEnvelopeKey(envelope) {
		if reply == nil {
			return ctx.SendStatus(fiber.StatusOK)
		}

		return ctx.JSON(reply)
	}

	if headers, headersOk := envelope["headers"].(map[string]any); headersOk {
		for name, value := range headers {
			ctx.Set(name, cast.ToString(value))
		}
	}

	ctx.Status(replyStatus(envelope))

	return sendReplyBody(ctx, envelope["body"])
}

// hasEnvelopeKey reports whether the map uses the response envelope contract.
func hasEnvelopeKey(reply map[string]any) bool {
	for _, key := range []string{"status", "headers", "body"} {
		if _, ok := reply[key]; ok {
			return true
		}
	}

	return false
}

// replyStatus resolves the envelope status; absent or out-of-range values
// fall back to 200.
func replyStatus(envelope map[string]any) int {
	status := cast.ToInt(envelope["status"])
	if status < fiber.StatusContinue || status > 599 {
		return fiber.StatusOK
	}

	return status
}

// sendReplyBody writes the envelope body: strings pass through verbatim
// (text/plain unless the script set a content type), any other value is JSON.
func sendReplyBody(ctx fiber.Ctx, body any) error {
	setDefaultContentType := func(value string) {
		if ctx.GetRespHeader(fiber.HeaderContentType) == "" {
			ctx.Set(fiber.HeaderContentType, value)
		}
	}

	switch value := body.(type) {
	case nil:
		return ctx.Send(nil)
	case string:
		setDefaultContentType(fiber.MIMETextPlainCharsetUTF8)

		return ctx.SendString(value)
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return integration.ErrScriptFailed(err.Error())
		}

		setDefaultContentType(fiber.MIMEApplicationJSONCharsetUTF8)

		return ctx.Send(payload)
	}
}
