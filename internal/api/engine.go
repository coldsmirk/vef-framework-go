package api

import (
	"fmt"
	"time"

	"github.com/coldsmirk/go-collections"
	"github.com/coldsmirk/go-streams"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/result"
)

var logger = logx.Named("api")

type EngineOption func(*engine)

// engine implements api.Engine.
//
// Concurrency contract: operations is a ConcurrentMap and supports concurrent
// Register calls after Mount. routerOperations is a plain (non-concurrent) Map
// because it is written only during WithRouters (before any goroutine can call
// Register) and never mutated thereafter — reads are safe without a lock. Do
// not mutate routerOperations outside of init-time option application.
type engine struct {
	defaultVersion   string
	defaultTimeout   time.Duration
	defaultAuth      *api.AuthConfig
	defaultRateLimit *api.RateLimitConfig

	operations       collections.ConcurrentMap[api.Identifier, *api.Operation]
	routerOperations collections.Map[api.RouterStrategy, collections.ConcurrentSet[api.Identifier]]
	collectors       []api.OperationsCollector
	resolvers        []api.HandlerResolver
	adapters         []api.HandlerAdapter
	router           fiber.Router
}

func WithDefaultTimeout(timeout time.Duration) EngineOption {
	return func(e *engine) {
		e.defaultTimeout = timeout
	}
}

func WithDefaultVersion(version string) EngineOption {
	return func(e *engine) {
		e.defaultVersion = version
	}
}

func WithDefaultAuth(auth *api.AuthConfig) EngineOption {
	return func(e *engine) {
		e.defaultAuth = auth
	}
}

func WithDefaultRateLimit(rateLimit *api.RateLimitConfig) EngineOption {
	return func(e *engine) {
		e.defaultRateLimit = rateLimit
	}
}

func WithRouters(routers ...api.RouterStrategy) EngineOption {
	return func(e *engine) {
		e.routerOperations = streams.ToHashMapC(
			streams.FromSlice(routers),
			func(router api.RouterStrategy) (api.RouterStrategy, collections.ConcurrentSet[api.Identifier]) {
				return router, collections.NewConcurrentHashSet[api.Identifier]()
			},
		)
	}
}

func WithOperationCollectors(collectors ...api.OperationsCollector) EngineOption {
	return func(e *engine) {
		e.collectors = collectors
	}
}

func WithHandlerResolvers(resolvers ...api.HandlerResolver) EngineOption {
	return func(e *engine) {
		e.resolvers = resolvers
	}
}

func WithHandlerAdapters(adapters ...api.HandlerAdapter) EngineOption {
	return func(e *engine) {
		e.adapters = adapters
	}
}

// NewEngine creates a new API engine with the given options.
func NewEngine(opts ...EngineOption) (api.Engine, error) {
	eng := &engine{
		defaultTimeout: 30 * time.Second,
		defaultVersion: api.VersionV1,
		defaultAuth:    api.BearerAuth(),
		defaultRateLimit: &api.RateLimitConfig{
			Max:    100,
			Period: 5 * time.Minute,
		},
		operations: collections.NewConcurrentHashMap[api.Identifier, *api.Operation](),
	}

	for _, opt := range opts {
		opt(eng)
	}

	return eng, nil
}

// Register adds resources to the engine.
func (e *engine) Register(resources ...api.Resource) error {
	for _, res := range resources {
		if err := e.registerResource(res); err != nil {
			return err
		}
	}

	return nil
}

// Mount attaches the engine to a Fiber router.
func (e *engine) Mount(router fiber.Router) error {
	e.router = router

	for rs := range e.routerOperations.SeqKeys() {
		if err := rs.Setup(router); err != nil {
			return fmt.Errorf("failed to setup router %s: %w", rs.Name(), err)
		}
	}

	for rs, ops := range e.routerOperations.Seq() {
		for identifier := range ops.Seq() {
			if err := e.mountOperation(rs, identifier); err != nil {
				return err
			}
		}
	}

	return nil
}

// Lookup finds an operation by identifier.
func (e *engine) Lookup(identifier api.Identifier) *api.Operation {
	op, _ := e.operations.Get(identifier)

	return op
}

// registerResource registers a single resource.
func (e *engine) registerResource(res api.Resource) error {
	if res == nil {
		return shared.ErrResourceNil
	}

	if res.Name() == "" {
		return shared.ErrResourceNameEmpty
	}

	for _, collector := range e.collectors {
		for _, spec := range collector.Collect(res) {
			if err := e.registerOperation(res, spec); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *engine) registerOperation(res api.Resource, spec api.OperationSpec) error {
	if spec.Action == "" {
		return fmt.Errorf("%w for resource %s", shared.ErrOperationActionEmpty, res.Name())
	}

	rs := e.findRouterStrategy(res.Kind())
	if rs == nil {
		return fmt.Errorf("%w: %s", shared.ErrNoRouterForKind, res.Kind())
	}

	h, err := e.resolveHandler(spec, res)
	if err != nil {
		return fmt.Errorf("failed to resolve handler for %s:%s: %w", res.Name(), spec.Action, err)
	}

	op := e.buildOperation(res, spec, h)

	if existing, inserted := e.operations.PutIfAbsent(op.Identifier, op); !inserted {
		return &shared.DuplicateError{
			BaseError: shared.BaseError{Identifier: &op.Identifier},
			Existing:  existing,
		}
	}

	operations, ok := e.routerOperations.Get(rs)
	if !ok {
		return fmt.Errorf("%w: %s", shared.ErrNoRouterFound, rs.Name())
	}

	operations.AddIfAbsent(op.Identifier)

	if e.router != nil {
		if err := e.mountOperation(rs, op.Identifier); err != nil {
			return err
		}
	}

	logger.Infof("Registered %s operation: resource=%s, action=%s, version=%s, auth=%s, audit=%v",
		rs.Name(),
		op.Resource, op.Action, op.Version,
		op.Auth.Strategy, op.EnableAudit)

	return nil
}

// buildOperation constructs an api.Operation from a resource and spec.
func (e *engine) buildOperation(res api.Resource, spec api.OperationSpec, handler any) *api.Operation {
	ac := e.resolveAuthConfig(res, spec)
	if spec.RequiredPermission != "" {
		if ac.Options == nil {
			ac.Options = make(map[string]any)
		}

		ac.Options[shared.AuthOptionRequiredPermission] = spec.RequiredPermission
	}

	return &api.Operation{
		Identifier: api.Identifier{
			Resource: res.Name(),
			Action:   spec.Action,
			Version:  lo.CoalesceOrEmpty(res.Version(), e.defaultVersion, api.VersionV1),
		},
		Auth:        ac,
		Timeout:     e.resolveTimeout(spec.Timeout),
		RateLimit:   e.resolveRateLimit(spec.RateLimit),
		EnableAudit: spec.EnableAudit,
		Meta: map[string]any{
			shared.MetaKeyResource: res,
		},
		Handler: handler,
	}
}

// mountOperation adapts and routes a single operation.
func (e *engine) mountOperation(rs api.RouterStrategy, identifier api.Identifier) error {
	op, ok := e.operations.Get(identifier)
	if !ok {
		return fmt.Errorf("%w: %s", shared.ErrOperationNotFound, identifier)
	}

	handler, err := e.adaptHandler(op)
	if err != nil {
		return fmt.Errorf("failed to adapt handler for %s: %w", identifier, err)
	}

	rs.Route(e.wrapHandlerIfNecessary(handler, op), op)

	return nil
}

func (e *engine) resolveAuthConfig(res api.Resource, spec api.OperationSpec) *api.AuthConfig {
	if spec.Public {
		return api.Public()
	}

	if res.Auth() != nil {
		return res.Auth().Clone()
	}

	return e.defaultAuth.Clone()
}

// findRouterStrategy finds a router that can handle the given resource kind.
func (e *engine) findRouterStrategy(kind api.Kind) api.RouterStrategy {
	for router := range e.routerOperations.SeqKeys() {
		if router.CanHandle(kind) {
			return router
		}
	}

	return nil
}

// resolveHandler resolves the handler from spec or resource.
func (e *engine) resolveHandler(spec api.OperationSpec, res api.Resource) (any, error) {
	for _, resolver := range e.resolvers {
		handler, err := resolver.Resolve(res, spec)
		if err != nil {
			return nil, err
		}

		if handler != nil {
			return handler, nil
		}
	}

	return nil, fmt.Errorf("%w for %s:%s", shared.ErrNoHandlerResolverFound, spec.Action, res.Name())
}

// resolveTimeout returns operation timeout or default.
func (e *engine) resolveTimeout(t time.Duration) time.Duration {
	if t > 0 {
		return t
	}

	return e.defaultTimeout
}

// resolveRateLimit returns operation rate limit or default.
func (e *engine) resolveRateLimit(limit *api.RateLimitConfig) *api.RateLimitConfig {
	if limit != nil {
		return limit
	}

	return e.defaultRateLimit
}

// adaptHandler uses the adapter chain to convert the handler.
func (e *engine) adaptHandler(op *api.Operation) (fiber.Handler, error) {
	for _, adapter := range e.adapters {
		handler, err := adapter.Adapt(op.Handler, op)
		if err != nil {
			return nil, err
		}

		if handler != nil {
			return handler, nil
		}
	}

	return nil, fmt.Errorf("%w: %T", shared.ErrNoHandlerAdapterFound, op.Handler)
}

func (*engine) wrapHandlerIfNecessary(handler fiber.Handler, op *api.Operation) fiber.Handler {
	if op.Timeout <= 0 {
		return handler
	}

	return timeout.New(handler, timeout.Config{
		Timeout: op.Timeout,
		// The timeout middleware does not route OnTimeout's returned error
		// through the app error handler, so render the response here. All
		// fields derive from the ErrRequestTimeout sentinel (single source
		// of truth) rather than being hardcoded inline.
		OnTimeout: func(c fiber.Ctx) error {
			return result.Result{
				Code:    result.ErrRequestTimeout.Code,
				Message: result.ErrRequestTimeout.Message,
			}.Response(c, result.ErrRequestTimeout.Status)
		},
	})
}
