package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// Contextual injects DB and Logger into the request context.
// It sets up a contextual database with the operator ID and a scoped logger
// with request identification information.
type Contextual struct {
	db orm.DB
}

// NewContextual creates a new context middleware.
func NewContextual(db orm.DB) api.Middleware {
	return &Contextual{
		db: db,
	}
}

// Name returns the middleware name.
func (*Contextual) Name() string {
	return "contextual"
}

// Order returns the middleware order.
// Runs after authentication (-100) but before authorization (-90).
func (*Contextual) Order() int {
	return -90
}

// Process sets up the request context with DB and Logger.
func (m *Contextual) Process(ctx fiber.Ctx) error {
	principal := contextx.Principal(ctx)
	if principal == nil {
		principal = security.PrincipalAnonymous
	}

	db := m.db.WithNamedArg(orm.PlaceholderKeyOperator, principal.ID)
	contextx.SetDB(ctx, db)
	ctx.SetContext(contextx.SetDB(ctx.Context(), db))

	req := shared.Request(ctx)

	lgr := contextx.Logger(ctx)
	if req != nil && lgr != nil {
		scopedLogger := lgr.
			Named(buildLoggerName(req.Resource, req.Action, req.Version)).
			Named(buildLoggerName(string(principal.Type), principal.ID, principal.Name))
		contextx.SetLogger(ctx, scopedLogger)
		ctx.SetContext(contextx.SetLogger(ctx.Context(), scopedLogger))
	}

	return ctx.Next()
}

// buildLoggerName joins three parts into "a:b@c" format.
func buildLoggerName(a, b, c string) string {
	return a + ":" + b + "@" + c
}
