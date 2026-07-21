package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// StubDataScope is an inert scope; the middleware only stores it, never runs it.
type StubDataScope struct{}

func (*StubDataScope) Key() string { return "stub" }

func (*StubDataScope) Priority() int { return 0 }

func (*StubDataScope) Supports(*security.Principal, *orm.Table) bool { return true }

func (*StubDataScope) Apply(*security.Principal, orm.SelectQuery) error { return nil }

// RecordingResolver captures the arguments the middleware resolves with and
// hands back a fixed outcome.
type RecordingResolver struct {
	scope security.DataScope
	err   error

	calls      int
	principal  *security.Principal
	permission string
}

func (r *RecordingResolver) ResolveDataScope(_ context.Context, principal *security.Principal, permission string) (security.DataScope, error) {
	r.calls++
	r.principal = principal
	r.permission = permission

	return r.scope, r.err
}

// dataPermissionOutcome is what one Process run observed.
type dataPermissionOutcome struct {
	err     error
	reached bool
	applier security.DataPermissionApplier
}

// runDataPermissionProcess drives DataPermission.Process for one request.
// A nil op leaves the operation out of the context, exercising the missing
// operation path; principal is stored only when non-nil.
func runDataPermissionProcess(
	t *testing.T,
	resolver security.DataPermissionResolver,
	op *api.Operation,
	principal *security.Principal,
) dataPermissionOutcome {
	t.Helper()

	var outcome dataPermissionOutcome

	mw := NewDataPermission(resolver)

	app := fiber.New()
	app.Post("/",
		func(c fiber.Ctx) error {
			contextx.SetLogger(c, logx.Named("test"))

			if op != nil {
				shared.SetOperation(c, op)
			}

			if principal != nil {
				contextx.SetPrincipal(c, principal)
			}

			// Swallowed so the assertions read the middleware's own error rather
			// than whatever the app error handler renders.
			outcome.err = mw.Process(c)
			outcome.applier = contextx.DataPermApplier(c.Context())

			return nil
		},
		func(c fiber.Ctx) error {
			outcome.reached = true

			return c.SendStatus(fiber.StatusOK)
		},
	)

	req := httptest.NewRequestWithContext(t.Context(), fiber.MethodPost, "/", nil)

	_, err := app.Test(req)
	require.NoError(t, err, "The test request should be served")

	return outcome
}

// operationRequiring builds an operation declaring the given permission; an
// empty token means the operation requires none.
func operationRequiring(permission string) *api.Operation {
	options := map[string]any{}
	if permission != "" {
		options[shared.AuthOptionRequiredPermission] = permission
	}

	return &api.Operation{Auth: &api.AuthConfig{Options: options}}
}

func TestDataPermissionProcess(t *testing.T) {
	t.Run("ResolvesTheScopeForTheDeclaredPermission", func(t *testing.T) {
		resolver := &RecordingResolver{scope: new(StubDataScope)}
		principal := security.NewUser("u1", "Alice", "admin")

		outcome := runDataPermissionProcess(t, resolver, operationRequiring("user.read"), principal)

		require.NoError(t, outcome.err, "A resolvable data scope must not fail the request")
		assert.True(t, outcome.reached, "A resolved request must reach the handler")
		assert.Equal(t, 1, resolver.calls, "The resolver should be consulted exactly once")
		assert.Same(t, principal, resolver.principal, "The resolver should see the request principal")
		assert.Equal(t, "user.read", resolver.permission, "The resolver should see the operation's required permission")
		assert.NotNil(t, outcome.applier, "The resolved scope must be injected as an applier for downstream queries")
	})

	t.Run("InjectsAnApplierEvenWithoutAMatchingScope", func(t *testing.T) {
		// A nil scope means "no restriction for this principal", not "skip data
		// permissions": downstream queries still go through an applier.
		resolver := &RecordingResolver{}

		outcome := runDataPermissionProcess(t, resolver, operationRequiring("user.read"), security.NewUser("u1", "Alice"))

		require.NoError(t, outcome.err, "An unrestricted principal must not fail the request")
		assert.True(t, outcome.reached, "An unrestricted request must reach the handler")
		assert.NotNil(t, outcome.applier, "A nil scope must still yield an applier")
	})

	t.Run("FallsBackToTheAnonymousPrincipal", func(t *testing.T) {
		resolver := &RecordingResolver{}

		outcome := runDataPermissionProcess(t, resolver, operationRequiring("user.read"), nil)

		require.NoError(t, outcome.err, "A missing principal must not fail resolution")
		require.NotNil(t, resolver.principal, "The resolver must never receive a nil principal")
		assert.Equal(t, security.PrincipalAnonymous.ID, resolver.principal.ID,
			"An unauthenticated request should resolve as the anonymous identity")
	})

	t.Run("SkipsResolutionWithoutARequiredPermission", func(t *testing.T) {
		resolver := &RecordingResolver{scope: new(StubDataScope)}

		outcome := runDataPermissionProcess(t, resolver, operationRequiring(""), security.NewUser("u1", "Alice"))

		require.NoError(t, outcome.err, "An operation declaring no permission must pass through")
		assert.True(t, outcome.reached, "An unscoped request must reach the handler")
		assert.Zero(t, resolver.calls, "No permission means nothing to resolve")
		assert.Nil(t, outcome.applier, "No permission means no applier is injected")
	})

	t.Run("DeniesWhenTheOperationIsMissing", func(t *testing.T) {
		resolver := &RecordingResolver{scope: new(StubDataScope)}

		outcome := runDataPermissionProcess(t, resolver, nil, security.NewUser("u1", "Alice"))

		require.Error(t, outcome.err, "Without an operation the middleware cannot know the scope, so it must deny")
		assert.ErrorIs(t, outcome.err, fiber.ErrUnauthorized, "A missing operation should deny with 401")
		assert.False(t, outcome.reached, "A denied request must not reach the handler")
		assert.Zero(t, resolver.calls, "A denied request must not consult the resolver")
	})

	t.Run("DeniesWhenNoResolverIsRegistered", func(t *testing.T) {
		// Fail closed: an operation declaring a data permission must never run
		// unscoped just because the application registered no resolver.
		outcome := runDataPermissionProcess(t, nil, operationRequiring("user.read"), security.NewUser("u1", "Alice"))

		require.Error(t, outcome.err, "A declared data permission with no resolver must deny")
		assert.ErrorIs(t, outcome.err, fiber.ErrForbidden, "A missing resolver should deny with 403")
		assert.ErrorIs(t, outcome.err, ErrDataPermissionResolverNotProvided, "The denial should name the missing resolver")
		assert.False(t, outcome.reached, "A denied request must not reach the handler")
	})

	t.Run("DeniesWhenResolutionFails", func(t *testing.T) {
		loaderErr := errors.New("loader timeout")
		resolver := &RecordingResolver{err: loaderErr}

		outcome := runDataPermissionProcess(t, resolver, operationRequiring("user.read"), security.NewUser("u1", "Alice"))

		require.Error(t, outcome.err, "A resolver failure must deny rather than run the query unscoped")
		assert.ErrorIs(t, outcome.err, fiber.ErrForbidden, "A resolver failure should deny with 403")
		assert.ErrorIs(t, outcome.err, loaderErr, "The underlying resolver error should stay inspectable")
		assert.False(t, outcome.reached, "A denied request must not reach the handler")
		assert.Nil(t, outcome.applier, "A failed resolution must not inject an applier")
	})
}
