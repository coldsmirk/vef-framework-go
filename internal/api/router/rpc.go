package router

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/coldsmirk/go-collections"
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/fiberx"
	"github.com/coldsmirk/vef-framework-go/internal/api/middleware"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
)

const (
	DefaultRPCEndpoint = "/api"
	FormKeyParams      = "params"
	FormKeyMeta        = "meta"
)

// RPC implements api.RouterStrategy for RPC-style single endpoint routing.
type RPC struct {
	path       string
	chain      *middleware.Chain
	operations collections.ConcurrentMap[api.Identifier, *routeEntry]
}

type routeEntry struct {
	op      *api.Operation
	handler fiber.Handler
}

// NewRPC creates a new RPC-style router.
// If path is empty, defaults to "/api".
func NewRPC(path string, chain *middleware.Chain) api.RouterStrategy {
	if path == "" {
		path = DefaultRPCEndpoint
	}

	return &RPC{
		path:       path,
		chain:      chain,
		operations: collections.NewConcurrentHashMap[api.Identifier, *routeEntry](),
	}
}

func (*RPC) Name() string {
	return api.KindRPC.String()
}

func (*RPC) CanHandle(kind api.Kind) bool {
	return kind == api.KindRPC
}

func (r *RPC) Setup(router fiber.Router) error {
	handlers := slices.Concat(r.chain.Handlers(), []any{r.dispatch})

	group := router.Group(r.path)
	group.Post("", r.resolve, handlers...)

	return nil
}

// resolve is a middleware that parses the request and sets the operation in context.
func (r *RPC) resolve(ctx fiber.Ctx) error {
	req, err := r.parseRequest(ctx)
	if err != nil {
		return err
	}

	entry, ok := r.operations.Get(req.Identifier)
	if !ok {
		nfe := &shared.NotFoundError{
			BaseError: shared.BaseError{
				Identifier: &req.Identifier,
				Err:        fiber.ErrNotFound,
			},
			Suggestion: r.findClosestAPI(req.Identifier),
		}
		// Log the full message (including "did you mean" suggestion) since the
		// global error handler only surfaces the generic not-found response.
		contextx.Logger(ctx).Warnf("RPC resolve: %s", nfe.Error())

		return nfe
	}

	shared.SetRequest(ctx, req)
	shared.SetOperation(ctx, entry.op)
	shared.SetHandler(ctx, entry.handler)

	return ctx.Next()
}

func (r *RPC) Route(handler fiber.Handler, op *api.Operation) {
	r.operations.Put(op.Identifier, &routeEntry{
		op:      op,
		handler: handler,
	})
}

func (*RPC) dispatch(ctx fiber.Ctx) error {
	return shared.Handler(ctx)(ctx)
}

func (*RPC) parseRequest(ctx fiber.Ctx) (*api.Request, error) {
	req := &api.Request{
		Params: api.Params{},
		Meta:   api.Meta{},
	}

	if fiberx.IsJSON(ctx) {
		if err := ctx.Bind().Body(req); err != nil {
			return nil, err
		}
	} else {
		if err := parseFormRequest(ctx, req); err != nil {
			return nil, err
		}
	}

	return req, nil
}

func (r *RPC) findClosestAPI(requested api.Identifier) *api.Identifier {
	requestedStr := identifierToString(requested)

	match, distance, ok := shared.Closest(requestedStr, r.operations.SeqKeys(), identifierToString)

	// Only suggest if the match is unambiguous and the distance is less than
	// half the requested string length.
	if ok && distance < len(requestedStr)/2 {
		return &match
	}

	return nil
}

func parseFormRequest(ctx fiber.Ctx, request *api.Request) error {
	if err := ctx.Bind().Form(request); err != nil {
		return err
	}

	if params := ctx.FormValue(FormKeyParams); params != "" {
		if err := json.Unmarshal([]byte(params), &request.Params); err != nil {
			contextx.Logger(ctx).Warnf("Failed to parse params json: %v", err)

			return api.ErrInvalidRequestParams
		}
	}

	if meta := ctx.FormValue(FormKeyMeta); meta != "" {
		if err := json.Unmarshal([]byte(meta), &request.Meta); err != nil {
			contextx.Logger(ctx).Warnf("Failed to parse meta json: %v", err)

			return api.ErrInvalidRequestMeta
		}
	}

	if fiberx.IsMultipart(ctx) {
		if form, err := ctx.MultipartForm(); err == nil && form != nil {
			for key, files := range form.File {
				if len(files) > 0 {
					request.Params[key] = files
				}
			}
		}
	}

	return nil
}

func identifierToString(identifier api.Identifier) string {
	return fmt.Sprintf("%s/%s@%s", identifier.Resource, identifier.Action, identifier.Version)
}
