package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
)

// MockResource implements api.Resource for testing.
type MockResource struct {
	kind    api.Kind
	name    string
	version string
	auth    *api.AuthConfig
}

func (m *MockResource) Kind() api.Kind                { return m.kind }
func (m *MockResource) Name() string                  { return m.name }
func (m *MockResource) Version() string               { return m.version }
func (m *MockResource) Auth() *api.AuthConfig         { return m.auth }
func (*MockResource) Operations() []api.OperationSpec { return nil }

// MockRouterStrategy implements api.RouterStrategy for testing.
type MockRouterStrategy struct {
	name       string
	setupErr   error
	setupCalls int
	routeCalls int
}

func (m *MockRouterStrategy) Name() string {
	return m.name
}

func (m *MockRouterStrategy) CanHandle(kind api.Kind) bool {
	return kind.String() == m.name
}

func (m *MockRouterStrategy) Setup(fiber.Router) error {
	m.setupCalls++

	return m.setupErr
}

func (m *MockRouterStrategy) Route(fiber.Handler, *api.Operation) {
	m.routeCalls++
}

// MockOperationsCollector implements api.OperationsCollector for testing.
type MockOperationsCollector struct {
	specs []api.OperationSpec
}

func (m *MockOperationsCollector) Collect(api.Resource) []api.OperationSpec {
	return m.specs
}

// MockHandlerResolver implements api.HandlerResolver for testing.
type MockHandlerResolver struct {
	handler   any
	err       error
	returnNil bool
}

func (m *MockHandlerResolver) Resolve(api.Resource, api.OperationSpec) (any, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.returnNil {
		return nil, nil
	}

	return m.handler, nil
}

// MockHandlerAdapter implements api.HandlerAdapter for testing.
type MockHandlerAdapter struct {
	err       error
	returnNil bool
}

func (m *MockHandlerAdapter) Adapt(handler any, _ *api.Operation) (fiber.Handler, error) {
	if m.err != nil {
		return nil, m.err
	}

	if m.returnNil || handler == nil {
		return nil, nil
	}

	return func(fiber.Ctx) error { return nil }, nil
}

// DummyHandler is a simple handler for testing.
func DummyHandler(fiber.Ctx) error {
	return nil
}

// newTestEngine creates an engine with options and returns the concrete *engine.
func newTestEngine(t *testing.T, opts ...EngineOption) *engine {
	t.Helper()

	eng, err := NewEngine(opts...)
	require.NoError(t, err, "NewEngine should not return error")

	e, ok := eng.(*engine)
	require.True(t, ok, "Type assertion to *engine should succeed")

	return e
}

// newRegistrationEngine creates an engine pre-configured with a mock RPC router,
// collector, and resolver — the most common setup for registration tests.
func newRegistrationEngine(t *testing.T, specs []api.OperationSpec, extraOpts ...EngineOption) (*engine, *MockRouterStrategy) {
	t.Helper()

	router := &MockRouterStrategy{name: "rpc"}
	collector := &MockOperationsCollector{specs: specs}
	resolver := &MockHandlerResolver{handler: DummyHandler}

	opts := append([]EngineOption{
		WithRouters(router),
		WithOperationCollectors(collector),
		WithHandlerResolvers(resolver),
	}, extraOpts...)

	return newTestEngine(t, opts...), router
}

// TestNewEngine tests new engine scenarios.
func TestNewEngine(t *testing.T) {
	t.Run("ReturnsNonNilEngine", func(t *testing.T) {
		eng, err := NewEngine()
		assert.NoError(t, err, "NewEngine should not return error")
		assert.NotNil(t, eng, "NewEngine should return a non-nil engine")
	})

	t.Run("DefaultTimeout", func(t *testing.T) {
		e := newTestEngine(t)
		assert.Equal(t, 30*time.Second, e.defaultTimeout, "Default timeout should be 30 seconds")
	})

	t.Run("DefaultVersion", func(t *testing.T) {
		e := newTestEngine(t)
		assert.Equal(t, api.VersionV1, e.defaultVersion, "Default version should be v1")
	})

	t.Run("DefaultAuthIsBearer", func(t *testing.T) {
		e := newTestEngine(t)
		require.NotNil(t, e.defaultAuth, "Default auth should not be nil")
		assert.Equal(t, api.AuthStrategyBearer, e.defaultAuth.Strategy, "Default auth strategy should be bearer")
	})

	t.Run("DefaultRateLimit", func(t *testing.T) {
		e := newTestEngine(t)
		require.NotNil(t, e.defaultRateLimit, "Default rate limit should not be nil")
		assert.Equal(t, 100, e.defaultRateLimit.Max, "Default rate limit max should be 100")
		assert.Equal(t, 5*time.Minute, e.defaultRateLimit.Period, "Default rate limit period should be 5 minutes")
	})

	t.Run("OperationsMapInitialized", func(t *testing.T) {
		e := newTestEngine(t)
		assert.NotNil(t, e.operations, "Operations map should be initialized")
	})
}

// TestEngineOptions tests engine options scenarios.
func TestEngineOptions(t *testing.T) {
	t.Run("WithDefaultTimeout", func(t *testing.T) {
		customTimeout := 60 * time.Second
		e := newTestEngine(t, WithDefaultTimeout(customTimeout))
		assert.Equal(t, customTimeout, e.defaultTimeout, "Custom timeout should be applied")
	})

	t.Run("WithDefaultVersion", func(t *testing.T) {
		e := newTestEngine(t, WithDefaultVersion("v2"))
		assert.Equal(t, "v2", e.defaultVersion, "Custom version should be applied")
	})

	t.Run("WithDefaultAuth", func(t *testing.T) {
		e := newTestEngine(t, WithDefaultAuth(api.SignatureAuth()))
		assert.Equal(t, api.AuthStrategySignature, e.defaultAuth.Strategy, "Custom auth strategy should be applied")
	})

	t.Run("WithDefaultRateLimit", func(t *testing.T) {
		customRateLimit := &api.RateLimitConfig{Max: 50, Period: 1 * time.Minute}
		e := newTestEngine(t, WithDefaultRateLimit(customRateLimit))
		assert.Equal(t, 50, e.defaultRateLimit.Max, "Custom rate limit max should be applied")
		assert.Equal(t, 1*time.Minute, e.defaultRateLimit.Period, "Custom rate limit period should be applied")
	})

	t.Run("WithRouters", func(t *testing.T) {
		router1 := &MockRouterStrategy{name: "rpc"}
		router2 := &MockRouterStrategy{name: "rest"}
		e := newTestEngine(t, WithRouters(router1, router2))
		require.NotNil(t, e.routerOperations, "Router operations map should be initialized")
		assert.Equal(t, 2, e.routerOperations.Size(), "Should have 2 routers")
	})

	t.Run("WithOperationCollectors", func(t *testing.T) {
		collector := &MockOperationsCollector{}
		e := newTestEngine(t, WithOperationCollectors(collector))
		assert.Len(t, e.collectors, 1, "Should have 1 collector")
	})

	t.Run("WithHandlerResolvers", func(t *testing.T) {
		resolver := &MockHandlerResolver{handler: DummyHandler}
		e := newTestEngine(t, WithHandlerResolvers(resolver))
		assert.Len(t, e.resolvers, 1, "Should have 1 resolver")
	})

	t.Run("WithHandlerAdapters", func(t *testing.T) {
		adapter := &MockHandlerAdapter{}
		e := newTestEngine(t, WithHandlerAdapters(adapter))
		assert.Len(t, e.adapters, 1, "Should have 1 adapter")
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		e := newTestEngine(t,
			WithDefaultTimeout(45*time.Second),
			WithDefaultVersion("v3"),
			WithDefaultAuth(api.Public()),
		)
		assert.Equal(t, 45*time.Second, e.defaultTimeout, "Custom timeout should be applied")
		assert.Equal(t, "v3", e.defaultVersion, "Custom version should be applied")
		assert.Equal(t, api.AuthStrategyNone, e.defaultAuth.Strategy, "Custom auth should be applied")
	})
}

// TestEngineRegister tests engine register scenarios.
func TestEngineRegister(t *testing.T) {
	t.Run("NilResource", func(t *testing.T) {
		eng, err := NewEngine()
		require.NoError(t, err, "NewEngine should not return error")

		err = eng.Register(nil)
		assert.Error(t, err, "Register should return error for nil resource")
		assert.Contains(t, err.Error(), "nil", "Error message should mention nil")
	})

	t.Run("EmptyResourceName", func(t *testing.T) {
		eng, err := NewEngine(
			WithOperationCollectors(&MockOperationsCollector{
				specs: []api.OperationSpec{{Action: "create"}},
			}),
		)
		require.NoError(t, err, "NewEngine should not return error")

		res := &MockResource{kind: api.KindRPC, name: ""}
		err = eng.Register(res)
		assert.Error(t, err, "Register should return error for empty resource name")
		assert.Contains(t, err.Error(), "empty", "Error message should mention empty")
	})

	t.Run("EmptyActionName", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, nil)
		e.collectors = []api.OperationsCollector{&MockOperationsCollector{
			specs: []api.OperationSpec{{Action: ""}},
		}}
		e.resolvers = nil

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.Register(res)
		assert.Error(t, err, "Register should return error for empty action name")
		assert.Contains(t, err.Error(), "empty", "Error message should mention empty")
	})

	t.Run("NoRouterForKind", func(t *testing.T) {
		e := newTestEngine(t,
			WithRouters(&MockRouterStrategy{name: "rest"}),
			WithOperationCollectors(&MockOperationsCollector{
				specs: []api.OperationSpec{{Action: "create", Handler: DummyHandler}},
			}),
		)

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.Register(res)
		assert.Error(t, err, "Register should return error when no router handles kind")
		assert.Contains(t, err.Error(), "no router", "Error message should mention no router")
	})

	t.Run("HandlerResolverError", func(t *testing.T) {
		e := newTestEngine(t,
			WithRouters(&MockRouterStrategy{name: "rpc"}),
			WithOperationCollectors(&MockOperationsCollector{
				specs: []api.OperationSpec{{Action: "create"}},
			}),
			WithHandlerResolvers(&MockHandlerResolver{err: errors.New("resolve failed")}),
		)

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.Register(res)
		assert.Error(t, err, "Register should return error when handler resolver fails")
		assert.Contains(t, err.Error(), "resolve", "Error message should mention resolve")
	})

	t.Run("NoHandlerResolver", func(t *testing.T) {
		e := newTestEngine(t,
			WithRouters(&MockRouterStrategy{name: "rpc"}),
			WithOperationCollectors(&MockOperationsCollector{
				specs: []api.OperationSpec{{Action: "create"}},
			}),
		)

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.Register(res)
		assert.Error(t, err, "Register should return error when no handler resolver found")
	})

	t.Run("SuccessfulRegistration", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.Register(res)
		assert.NoError(t, err, "Register should succeed")
	})

	t.Run("DuplicateOperation", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}

		err := e.Register(res)
		require.NoError(t, err, "First registration should succeed")

		// Second registration of the same resource should fail
		// because Identifier value is used as map key (not pointer)
		err = e.Register(res)
		assert.Error(t, err, "Duplicate registration should fail")
		assert.Contains(t, err.Error(), "duplicate", "Error should indicate duplicate")
	})

	t.Run("MultipleResources", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res1 := &MockResource{kind: api.KindRPC, name: "test/resource1"}
		res2 := &MockResource{kind: api.KindRPC, name: "test/resource2"}

		err := e.Register(res1, res2)
		assert.NoError(t, err, "Registering multiple resources should succeed")
	})

	t.Run("ResourceWithCustomVersion", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource", version: "v2"}

		err := e.Register(res)
		require.NoError(t, err, "Registration with custom version should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "create", Version: "v2"})
		assert.NotNil(t, op, "Operation with custom version should be found")
	})

	t.Run("ResourceWithCustomAuth", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource", auth: api.SignatureAuth()}

		err := e.Register(res)
		require.NoError(t, err, "Registration with custom auth should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "create", Version: api.VersionV1})
		require.NotNil(t, op, "Operation should be found")
		assert.Equal(t, api.AuthStrategySignature, op.Auth.Strategy, "Operation should use resource's custom auth")
	})

	t.Run("OperationWithPermToken", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "delete", PermToken: "sys:user:delete"}})
		res := &MockResource{kind: api.KindRPC, name: "sys/user"}

		err := e.Register(res)
		require.NoError(t, err, "Registration with perm token should succeed")

		op := e.Lookup(api.Identifier{Resource: "sys/user", Action: "delete", Version: api.VersionV1})
		require.NotNil(t, op, "Operation should be found")
		require.NotNil(t, op.Auth.Options, "Auth options should not be nil")
		assert.Equal(t, "sys:user:delete", op.Auth.Options[shared.AuthOptionPermToken], "PermToken should be stored correctly")
	})

	t.Run("OperationWithCustomTimeout", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "export", Timeout: 2 * time.Minute}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}

		err := e.Register(res)
		require.NoError(t, err, "Registration with custom timeout should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "export", Version: api.VersionV1})
		require.NotNil(t, op, "Operation should be found")
		assert.Equal(t, 2*time.Minute, op.Timeout, "Operation should have custom timeout")
	})

	t.Run("OperationWithCustomRateLimit", func(t *testing.T) {
		customRateLimit := &api.RateLimitConfig{Max: 10, Period: 1 * time.Minute}
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "login", RateLimit: customRateLimit}})
		res := &MockResource{kind: api.KindRPC, name: "auth"}

		err := e.Register(res)
		require.NoError(t, err, "Registration with custom rate limit should succeed")

		op := e.Lookup(api.Identifier{Resource: "auth", Action: "login", Version: api.VersionV1})
		require.NotNil(t, op, "Operation should be found")
		assert.Equal(t, 10, op.RateLimit.Max, "Operation should have custom rate limit max")
	})

	t.Run("OperationWithEnableAudit", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "update", EnableAudit: true}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}

		err := e.Register(res)
		require.NoError(t, err, "Registration with audit enabled should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "update", Version: api.VersionV1})
		require.NotNil(t, op, "Operation should be found")
		assert.True(t, op.EnableAudit, "Operation should have audit enabled")
	})
}

// TestEngineLookup tests engine lookup scenarios.
func TestEngineLookup(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		e := newTestEngine(t)
		op := e.Lookup(api.Identifier{Resource: "nonexistent", Action: "create", Version: api.VersionV1})
		assert.Nil(t, op, "Lookup should return nil for non-existent operation")
	})

	t.Run("Found", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		require.NoError(t, e.Register(res), "Register should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "create", Version: api.VersionV1})
		require.NotNil(t, op, "Lookup should return operation")
		assert.Equal(t, "test/resource", op.Resource, "Operation resource should match")
		assert.Equal(t, "create", op.Action, "Operation action should match")
		assert.Equal(t, api.VersionV1, op.Version, "Operation version should match")
	})

	t.Run("VersionMismatch", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		require.NoError(t, e.Register(res), "Register should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "create", Version: "v2"})
		assert.Nil(t, op, "Lookup should return nil for version mismatch")
	})

	t.Run("ActionMismatch", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{{Action: "create"}})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		require.NoError(t, e.Register(res), "Register should succeed")

		op := e.Lookup(api.Identifier{Resource: "test/resource", Action: "update", Version: api.VersionV1})
		assert.Nil(t, op, "Lookup should return nil for action mismatch")
	})

	t.Run("MultipleOperations", func(t *testing.T) {
		e, _ := newRegistrationEngine(t, []api.OperationSpec{
			{Action: "create"},
			{Action: "update"},
			{Action: "delete"},
		})
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		require.NoError(t, e.Register(res), "Register should succeed")

		for _, action := range []string{"create", "update", "delete"} {
			op := e.Lookup(api.Identifier{Resource: "test/resource", Action: action, Version: api.VersionV1})
			assert.NotNil(t, op, "Lookup should return operation for action %s", action)
		}
	})
}

// TestEngineMount tests engine mount scenarios.
func TestEngineMount(t *testing.T) {
	t.Run("SetupError", func(t *testing.T) {
		e := newTestEngine(t, WithRouters(&MockRouterStrategy{name: "rpc", setupErr: errors.New("setup failed")}))

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		assert.Error(t, err, "Mount should return error when setup fails")
		assert.Contains(t, err.Error(), "setup", "Error message should mention setup")
	})

	t.Run("SuccessfulMount", func(t *testing.T) {
		e, router := newRegistrationEngine(t,
			[]api.OperationSpec{{Action: "create"}},
			WithHandlerAdapters(&MockHandlerAdapter{}),
		)
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		require.NoError(t, e.Register(res), "Register should succeed")

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		assert.NoError(t, err, "Mount should succeed")
		assert.Equal(t, 1, router.setupCalls, "Setup should be called once")
		assert.Equal(t, 1, router.routeCalls, "Route should be called once")
	})

	t.Run("MultipleRouters", func(t *testing.T) {
		rpcRouter := &MockRouterStrategy{name: "rpc"}
		restRouter := &MockRouterStrategy{name: "rest"}
		e := newTestEngine(t,
			WithRouters(rpcRouter, restRouter),
			WithOperationCollectors(&MockOperationsCollector{
				specs: []api.OperationSpec{{Action: "create"}},
			}),
			WithHandlerResolvers(&MockHandlerResolver{handler: DummyHandler}),
			WithHandlerAdapters(&MockHandlerAdapter{}),
		)
		require.NoError(t, e.Register(&MockResource{kind: api.KindRPC, name: "test/rpc"}), "Register RPC should succeed")
		require.NoError(t, e.Register(&MockResource{kind: api.KindREST, name: "test/rest"}), "Register REST should succeed")

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		assert.NoError(t, err, "Mount should succeed with multiple routers")
		assert.Equal(t, 1, rpcRouter.setupCalls, "RPC router setup should be called once")
		assert.Equal(t, 1, restRouter.setupCalls, "REST router setup should be called once")
	})

	t.Run("AdapterError", func(t *testing.T) {
		e, _ := newRegistrationEngine(t,
			[]api.OperationSpec{{Action: "create"}},
			WithHandlerAdapters(&MockHandlerAdapter{err: errors.New("adapt failed")}),
		)
		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		require.NoError(t, e.Register(res), "Register should succeed")

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		assert.Error(t, err, "Mount should return error when adapter fails")
		assert.Contains(t, err.Error(), "adapt", "Error message should mention adapt")
	})

	t.Run("OperationNotFoundInOperationsMap", func(t *testing.T) {
		router := &MockRouterStrategy{name: "rpc"}
		e := newTestEngine(t, WithRouters(router))

		// Manually add an identifier to routerOperations without adding to operations
		identifier := api.Identifier{Resource: "ghost/resource", Action: "create", Version: api.VersionV1}
		if ops, ok := e.routerOperations.Get(router); ok {
			ops.Add(identifier)
		}

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		assert.Error(t, err, "Mount should return error when operation not found")
		assert.Contains(t, err.Error(), "not found", "Error message should mention not found")
	})
}

// TestResolveTimeout tests resolve timeout scenarios.
func TestResolveTimeout(t *testing.T) {
	e := newTestEngine(t, WithDefaultTimeout(30*time.Second))

	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{"ZeroUsesDefault", 0, 30 * time.Second},
		{"CustomTimeoutUsed", 60 * time.Second, 60 * time.Second},
		{"NegativeUsesDefault", -1 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, e.resolveTimeout(tt.input), "Should return expected timeout")
		})
	}
}

// TestResolveRateLimit tests resolve rate limit scenarios.
func TestResolveRateLimit(t *testing.T) {
	defaultLimit := &api.RateLimitConfig{Max: 100, Period: 5 * time.Minute}
	customLimit := &api.RateLimitConfig{Max: 10, Period: 1 * time.Minute}
	e := newTestEngine(t, WithDefaultRateLimit(defaultLimit))

	tests := []struct {
		name     string
		input    *api.RateLimitConfig
		expected *api.RateLimitConfig
	}{
		{"NilUsesDefault", nil, defaultLimit},
		{"CustomRateLimitUsed", customLimit, customLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, e.resolveRateLimit(tt.input), "Should return expected rate limit")
		})
	}
}

// TestFindRouterStrategy tests find router strategy scenarios.
func TestFindRouterStrategy(t *testing.T) {
	rpcRouter := &MockRouterStrategy{name: "rpc"}
	restRouter := &MockRouterStrategy{name: "rest"}
	e := newTestEngine(t, WithRouters(rpcRouter, restRouter))

	t.Run("FindRPCRouter", func(t *testing.T) {
		result := e.findRouterStrategy(api.KindRPC)
		require.NotNil(t, result, "Should find RPC router")
		assert.Equal(t, "rpc", result.Name(), "Router name should be rpc")
	})

	t.Run("FindRESTRouter", func(t *testing.T) {
		result := e.findRouterStrategy(api.KindREST)
		require.NotNil(t, result, "Should find REST router")
		assert.Equal(t, "rest", result.Name(), "Router name should be rest")
	})

	t.Run("NotFound", func(t *testing.T) {
		onlyRPC := newTestEngine(t, WithRouters(&MockRouterStrategy{name: "rpc"}))
		result := onlyRPC.findRouterStrategy(api.KindREST)
		assert.Nil(t, result, "Should return nil for unknown kind")
	})
}

// TestAdaptHandler tests adapt handler scenarios.
func TestAdaptHandler(t *testing.T) {
	t.Run("FirstAdapterSucceeds", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerAdapters(&MockHandlerAdapter{}, &MockHandlerAdapter{}))

		op := &api.Operation{Handler: DummyHandler}
		result, err := e.adaptHandler(op)
		assert.NoError(t, err, "adaptHandler should not return error")
		assert.NotNil(t, result, "adaptHandler should return handler")
	})

	t.Run("FirstAdapterReturnsNilSecondSucceeds", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerAdapters(
			&MockHandlerAdapter{returnNil: true},
			&MockHandlerAdapter{},
		))

		op := &api.Operation{Handler: DummyHandler}
		result, err := e.adaptHandler(op)
		assert.NoError(t, err, "adaptHandler should not return error")
		assert.NotNil(t, result, "adaptHandler should return handler from second adapter")
	})

	t.Run("NoAdaptersConfigured", func(t *testing.T) {
		e := newTestEngine(t)

		op := &api.Operation{Handler: DummyHandler}
		result, err := e.adaptHandler(op)
		assert.Error(t, err, "adaptHandler should return error when no adapter handles")
		assert.Nil(t, result, "adaptHandler should return nil handler")
		assert.ErrorIs(t, err, shared.ErrNoHandlerAdapterFound, "Error should be ErrNoHandlerAdapterFound")
	})

	t.Run("AdapterError", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerAdapters(&MockHandlerAdapter{err: errors.New("adapt failed")}))

		op := &api.Operation{Handler: DummyHandler}
		result, err := e.adaptHandler(op)
		assert.Error(t, err, "adaptHandler should return error")
		assert.Nil(t, result, "adaptHandler should return nil handler")
	})

	t.Run("SingleAdapterReturnsNil", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerAdapters(&MockHandlerAdapter{returnNil: true}))

		op := &api.Operation{Handler: DummyHandler}
		result, err := e.adaptHandler(op)
		assert.Error(t, err, "adaptHandler should return error when adapter returns nil")
		assert.Nil(t, result, "adaptHandler should return nil handler")
		assert.ErrorIs(t, err, shared.ErrNoHandlerAdapterFound, "Error should be ErrNoHandlerAdapterFound")
	})

	t.Run("MultipleAdaptersAllReturnNil", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerAdapters(
			&MockHandlerAdapter{returnNil: true},
			&MockHandlerAdapter{returnNil: true},
		))

		op := &api.Operation{Handler: DummyHandler}
		result, err := e.adaptHandler(op)
		assert.Error(t, err, "adaptHandler should return error when all adapters return nil")
		assert.Nil(t, result, "adaptHandler should return nil handler")
		assert.ErrorIs(t, err, shared.ErrNoHandlerAdapterFound, "Error should be ErrNoHandlerAdapterFound")
	})
}

// TestWrapHandlerIfNecessary tests wrap handler if necessary scenarios.
func TestWrapHandlerIfNecessary(t *testing.T) {
	e := newTestEngine(t)
	handler := func(fiber.Ctx) error { return nil }

	t.Run("NoWrapWhenZeroTimeout", func(t *testing.T) {
		op := &api.Operation{Timeout: 0}
		result := e.wrapHandlerIfNecessary(handler, op)
		// Cannot directly compare functions, but we can verify it doesn't panic
		assert.NotNil(t, result, "Handler should not be nil")
	})

	t.Run("WrapsWhenPositiveTimeout", func(t *testing.T) {
		op := &api.Operation{Timeout: 30 * time.Second}
		result := e.wrapHandlerIfNecessary(handler, op)
		assert.NotNil(t, result, "Handler should not be nil")
	})

	t.Run("NoWrapWhenNegativeTimeout", func(t *testing.T) {
		op := &api.Operation{Timeout: -1 * time.Second}
		result := e.wrapHandlerIfNecessary(handler, op)
		assert.NotNil(t, result, "Handler should not be nil")
	})

	t.Run("TimeoutReturnsErrRequestTimeout", func(t *testing.T) {
		op := &api.Operation{Timeout: 10 * time.Millisecond}
		slowHandler := func(fiber.Ctx) error {
			time.Sleep(100 * time.Millisecond)

			return nil
		}
		wrappedHandler := e.wrapHandlerIfNecessary(slowHandler, op)

		app := fiber.New()
		defer app.Shutdown()

		app.Get("/test", wrappedHandler)

		req := httptest.NewRequestWithContext(context.Background(), "GET", "/test", nil)
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
		require.NoError(t, err, "Test request should not error")
		// Fiber timeout middleware returns 500 by default when OnTimeout returns an error
		// The actual status code depends on error handler configuration
		assert.True(t, resp.StatusCode >= 400, "Should return error status code on timeout")
	})
}

// TestResolveAuthConfig tests resolve auth config scenarios.
func TestResolveAuthConfig(t *testing.T) {
	e := newTestEngine(t, WithDefaultAuth(api.BearerAuth()))
	res := &MockResource{kind: api.KindRPC, name: "test/resource"}

	t.Run("PublicSpecReturnsPublicAuth", func(t *testing.T) {
		spec := api.OperationSpec{Action: "ping", Public: true}

		result := e.resolveAuthConfig(res, spec)
		require.NotNil(t, result, "Auth config should not be nil")
		assert.Equal(t, api.AuthStrategyNone, result.Strategy, "Public spec should return public auth (none strategy)")
	})

	t.Run("ResourceAuthOverridesDefault", func(t *testing.T) {
		resWithAuth := &MockResource{kind: api.KindRPC, name: "test/resource", auth: api.SignatureAuth()}
		spec := api.OperationSpec{Action: "create"}

		result := e.resolveAuthConfig(resWithAuth, spec)
		require.NotNil(t, result, "Auth config should not be nil")
		assert.Equal(t, api.AuthStrategySignature, result.Strategy, "Resource auth should override default")
	})

	t.Run("DefaultAuthUsedWhenNoResourceAuth", func(t *testing.T) {
		spec := api.OperationSpec{Action: "create"}

		result := e.resolveAuthConfig(res, spec)
		require.NotNil(t, result, "Auth config should not be nil")
		assert.Equal(t, api.AuthStrategyBearer, result.Strategy, "Default auth should be used")
	})

	t.Run("AuthConfigIsCloned", func(t *testing.T) {
		defaultAuth := api.BearerAuth()
		cloneEngine := newTestEngine(t, WithDefaultAuth(defaultAuth))

		spec := api.OperationSpec{Action: "create"}
		result := cloneEngine.resolveAuthConfig(res, spec)
		result.Strategy = "modified"

		assert.Equal(t, api.AuthStrategyBearer, defaultAuth.Strategy, "Original auth should not be modified")
	})

	t.Run("ResourceAuthConfigIsCloned", func(t *testing.T) {
		resourceAuth := api.SignatureAuth()
		resWithAuth := &MockResource{kind: api.KindRPC, name: "test/resource", auth: resourceAuth}
		spec := api.OperationSpec{Action: "create"}

		result := e.resolveAuthConfig(resWithAuth, spec)
		result.Strategy = "modified"

		assert.Equal(t, api.AuthStrategySignature, resourceAuth.Strategy, "Original resource auth should not be modified")
	})
}

// TestResolveHandler tests resolve handler scenarios.
func TestResolveHandler(t *testing.T) {
	res := &MockResource{kind: api.KindRPC, name: "test/resource"}
	spec := api.OperationSpec{Action: "create"}

	t.Run("FirstResolverSucceeds", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerResolvers(
			&MockHandlerResolver{handler: DummyHandler},
			&MockHandlerResolver{handler: DummyHandler},
		))

		result, err := e.resolveHandler(spec, res)
		assert.NoError(t, err, "resolveHandler should not return error")
		assert.NotNil(t, result, "resolveHandler should return handler")
	})

	t.Run("FirstResolverReturnsNilSecondSucceeds", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerResolvers(
			&MockHandlerResolver{returnNil: true},
			&MockHandlerResolver{handler: DummyHandler},
		))

		result, err := e.resolveHandler(spec, res)
		assert.NoError(t, err, "resolveHandler should not return error")
		assert.NotNil(t, result, "resolveHandler should return handler from second resolver")
	})

	t.Run("AllResolversReturnNil", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerResolvers(
			&MockHandlerResolver{returnNil: true},
			&MockHandlerResolver{returnNil: true},
		))

		result, err := e.resolveHandler(spec, res)
		assert.Error(t, err, "resolveHandler should return error when all resolvers return nil")
		assert.Nil(t, result, "resolveHandler should return nil handler")
		assert.Contains(t, err.Error(), "no handler resolver found", "Error should mention no resolver found")
	})

	t.Run("ResolverError", func(t *testing.T) {
		e := newTestEngine(t, WithHandlerResolvers(&MockHandlerResolver{err: errors.New("resolve failed")}))

		result, err := e.resolveHandler(spec, res)
		assert.Error(t, err, "resolveHandler should return error")
		assert.Nil(t, result, "resolveHandler should return nil handler")
	})
}

// TestRegisterResource tests register resource scenarios.
func TestRegisterResource(t *testing.T) {
	t.Run("NoCollectorsReturnsSuccess", func(t *testing.T) {
		e := newTestEngine(t, WithRouters(&MockRouterStrategy{name: "rpc"}))

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.registerResource(res)
		assert.NoError(t, err, "registerResource with no collectors should succeed")
	})

	t.Run("CollectorReturnsEmptySpecs", func(t *testing.T) {
		e := newTestEngine(t,
			WithRouters(&MockRouterStrategy{name: "rpc"}),
			WithOperationCollectors(&MockOperationsCollector{specs: []api.OperationSpec{}}),
		)

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err := e.registerResource(res)
		assert.NoError(t, err, "registerResource with empty specs should succeed")
	})
}

// TestRegisterAfterMount tests register after mount scenarios.
func TestRegisterAfterMount(t *testing.T) {
	t.Run("RegisterAfterMountRoutesImmediately", func(t *testing.T) {
		e, router := newRegistrationEngine(t,
			[]api.OperationSpec{{Action: "create"}},
			WithHandlerAdapters(&MockHandlerAdapter{}),
		)

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		require.NoError(t, err, "Mount should succeed")
		assert.Equal(t, 0, router.routeCalls, "No routes before registration")

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err = e.Register(res)
		assert.NoError(t, err, "Register after Mount should succeed")
		assert.Equal(t, 1, router.routeCalls, "Route should be called immediately after registration")
	})

	t.Run("RegisterAfterMountAdapterError", func(t *testing.T) {
		e, _ := newRegistrationEngine(t,
			[]api.OperationSpec{{Action: "create"}},
			WithHandlerAdapters(&MockHandlerAdapter{err: errors.New("adapt failed")}),
		)

		app := fiber.New()
		defer app.Shutdown()

		err := e.Mount(app)
		require.NoError(t, err, "Mount should succeed with no operations")

		res := &MockResource{kind: api.KindRPC, name: "test/resource"}
		err = e.Register(res)
		assert.Error(t, err, "Register after Mount should fail when adapter fails")
		assert.Contains(t, err.Error(), "adapt", "Error should mention adapt")
	})
}
