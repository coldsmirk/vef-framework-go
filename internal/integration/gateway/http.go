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

var logger = logx.Named("integration")

// httpPathPrefix anchors the HTTP inbound gateway; the two path parameters
// identify the calling system and the invoked contract.
const httpPathPrefix = "/integration/inbound"

// HTTPGateway is the HTTP protocol adapter of the inbound flow: an
// app.Middleware (the framework's route-assembly hook — MCP precedent) that
// registers the external-facing endpoint, translates each request into the
// protocol-neutral envelope, and renders the adapter script's reply. It
// deliberately bypasses the /api dispatch model: external replies need raw
// control of status and body, never the standard result envelope.
type HTTPGateway struct {
	receiver *exec.Receiver
	limit    fiber.Handler
}

// NewHTTPGateway creates the HTTP inbound gateway. The rate limiter counts
// per (system, client IP), so one flooding system cannot starve the others.
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
// remapped to caller-actionable statuses and rendered by the app error
// handler as the framework's standard envelope — adapters that must control
// the external-facing error format catch dispatch failures in the script
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
		return callerStatus(err)
	}

	// A render fault (a script that returned a malformed $response envelope)
	// is a business error riding HTTP 200; route it through callerStatus too,
	// so an external caller sees a 500, not a 200 carrying an error body it
	// would read as a successful delivery.
	if err := renderReply(ctx, reply); err != nil {
		return callerStatus(err)
	}

	return nil
}

// callerStatus maps a pipeline failure onto an HTTP status an external
// caller's retry logic can act on. On the API surface business errors
// deliberately ride HTTP 200 with envelope codes, but external callers do not
// read the envelope — a 200 for "contract not found" would count as a
// successful delivery. Errors already carrying a transport status (401 auth —
// which also covers unknown systems, so system codes cannot be enumerated —
// 429 rate limit, 501 handler) pass through; missing or disabled definitions
// behind a verified caller map uniformly to 404 so it cannot probe which
// piece exists, invalid input maps to 400, and everything else is a plain
// 500.
func callerStatus(err error) error {
	resultErr, ok := errors.AsType[result.Error](err)
	if !ok || resultErr.Status != fiber.StatusOK {
		return err
	}

	switch {
	case errors.Is(err, integration.ErrContractNotFound),
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

// responseEnvelopeKey marks a script reply as an explicit response envelope:
// the script returns { $response: { status, headers, body } } to take raw
// control of the HTTP reply. The "$"-prefixed marker cannot collide with a
// plausible business payload, so an ordinary reply — even one that happens to
// carry a "status" or "body" field — is never misread as an envelope.
const responseEnvelopeKey = "$response"

// renderReply writes the script's reply as the HTTP response: a map carrying
// the $response marker is rendered as that envelope, every other non-nil
// value is sent verbatim as a 200 JSON body.
func renderReply(ctx fiber.Ctx, reply any) error {
	if reply == nil {
		return ctx.SendStatus(fiber.StatusOK)
	}

	if wrapper, ok := reply.(map[string]any); ok {
		if raw, ok := wrapper[responseEnvelopeKey]; ok {
			envelope, ok := raw.(map[string]any)
			if !ok {
				return integration.ErrScriptFailed(responseEnvelopeKey + " must be an object")
			}

			return renderEnvelope(ctx, envelope)
		}
	}

	return ctx.JSON(reply)
}

// renderEnvelope writes an explicit response envelope: status (default 200),
// headers, and body. Whether the script set its own content type is tracked
// here — the response header itself cannot answer that, because fasthttp
// reports a text/plain default even before anything was set.
func renderEnvelope(ctx fiber.Ctx, envelope map[string]any) error {
	contentTypeSet := false

	if headers, ok := envelope["headers"].(map[string]any); ok {
		for name, value := range headers {
			ctx.Set(name, cast.ToString(value))

			if strings.EqualFold(name, fiber.HeaderContentType) {
				contentTypeSet = true
			}
		}
	}

	ctx.Status(replyStatus(envelope))

	return sendReplyBody(ctx, envelope["body"], contentTypeSet)
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
func sendReplyBody(ctx fiber.Ctx, body any, contentTypeSet bool) error {
	setDefaultContentType := func(value string) {
		if !contentTypeSet {
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
