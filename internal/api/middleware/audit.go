package middleware

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils/v2"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Audit handles audit logging.
type Audit struct {
	bus event.Bus
}

// NewAudit creates a new audit middleware.
func NewAudit(bus event.Bus) api.Middleware {
	return &Audit{
		bus: bus,
	}
}

// Name returns the middleware name.
func (*Audit) Name() string {
	return "audit"
}

// Order returns the middleware order.
func (*Audit) Order() int {
	return -60
}

// Process handles the audit logging.
func (m *Audit) Process(ctx fiber.Ctx) error {
	op := shared.Operation(ctx)
	if op == nil {
		contextx.Logger(ctx).Warnf("Audit skipped: %v", ErrOperationNotFound)

		return ctx.Next()
	}

	return m.audit(ctx, op)
}

func (m *Audit) audit(ctx fiber.Ctx, op *api.Operation) error {
	if !op.EnableAudit || m.bus == nil {
		return ctx.Next()
	}

	var (
		start      = time.Now()
		handlerErr = ctx.Next()
		elapsed    = time.Since(start).Milliseconds()
	)

	evt, buildErr := buildAuditEvent(ctx, elapsed, handlerErr)
	if buildErr != nil {
		contextx.Logger(ctx).Errorf("%v: %v", ErrAuditEventBuildFailed, buildErr)

		return handlerErr
	}

	if err := m.bus.Publish(ctx.Context(), evt, event.WithAsync()); err != nil {
		contextx.Logger(ctx).Warnf("audit publish failed: %v", err)
	}

	return handlerErr
}

func buildAuditEvent(ctx fiber.Ctx, elapsed int64, err error) (*api.AuditEvent, error) {
	req := shared.Request(ctx)
	if req == nil {
		return nil, ErrRequestNotFound
	}

	principal := contextx.Principal(ctx)
	if principal == nil {
		principal = security.PrincipalAnonymous
	}

	var (
		resultCode int
		resultMsg  string
		resultData any
	)

	if err != nil {
		resultCode, resultMsg = extractErrorInfo(err)
	} else {
		// Attempt to decode the result envelope; for non-JSON responses (file
		// downloads, streamed content) fall back to the HTTP status code so
		// the audit record is always published.
		var res result.Result
		if decodeErr := json.Unmarshal(utils.CopyBytes(ctx.Response().Body()), &res); decodeErr == nil {
			resultCode = res.Code
			resultMsg = res.Message
			resultData = res.Data
		} else {
			resultCode = ctx.Response().StatusCode()
		}
	}

	return &api.AuditEvent{
		Resource:      req.Resource,
		Action:        req.Action,
		Version:       req.Version,
		UserID:        principal.ID,
		UserAgent:     utils.CopyString(ctx.Get(fiber.HeaderUserAgent)),
		RequestID:     contextx.RequestID(ctx),
		RequestIP:     httpx.GetIP(ctx),
		RequestParams: req.Params,
		RequestMeta:   req.Meta,
		ResultCode:    resultCode,
		ResultMessage: resultMsg,
		ResultData:    resultData,
		ElapsedTime:   elapsed,
	}, nil
}

func extractErrorInfo(err error) (code int, message string) {
	if resultErr, ok := result.AsErr(err); ok {
		return resultErr.Code, resultErr.Message
	}

	if fiberErr, ok := errors.AsType[*fiber.Error](err); ok {
		mappedCode, messageKey, _ := app.MapFiberError(fiberErr.Code)

		return mappedCode, i18n.T(messageKey)
	}

	return result.ErrCodeUnknown, err.Error()
}
