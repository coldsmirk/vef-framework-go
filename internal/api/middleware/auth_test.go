package middleware

import (
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
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// StubStrategy hands back a fixed principal, standing in for an
// application-supplied auth strategy.
type StubStrategy struct {
	principal *security.Principal
}

func (*StubStrategy) Name() string { return "stub" }

func (s *StubStrategy) Authenticate(fiber.Ctx, map[string]any) (*security.Principal, error) {
	return s.principal, nil
}

// StubRegistry resolves every lookup to its single strategy.
type StubRegistry struct {
	strategy api.AuthStrategy
}

func (*StubRegistry) Register(api.AuthStrategy) {}

func (r *StubRegistry) Get(string) (api.AuthStrategy, bool) { return r.strategy, true }

func (*StubRegistry) Names() []string { return []string{"stub"} }

// runAuthProcess drives Auth.Process for one request whose strategy returns the
// given principal, reporting whether the downstream handler was reached and the
// middleware's own error.
func runAuthProcess(t *testing.T, principal *security.Principal) (bool, error) {
	t.Helper()

	var (
		processErr error
		reached    bool
	)

	mw := NewAuth(&StubRegistry{strategy: &StubStrategy{principal: principal}}, nil)

	app := fiber.New()
	app.Post("/",
		func(c fiber.Ctx) error {
			// SetLogger is dual-mode: handing it the fiber.Ctx stores the logger in
			// Locals, which is where the middleware reads it back from.
			contextx.SetLogger(c, logx.Named("test"))
			shared.SetOperation(c, &api.Operation{Auth: &api.AuthConfig{Strategy: "stub"}})

			// Swallowed so the assertions can read the error directly instead of
			// through whatever error handler the app happens to install.
			processErr = mw.Process(c)

			return nil
		},
		func(c fiber.Ctx) error {
			reached = true

			return c.SendStatus(fiber.StatusOK)
		},
	)

	req := httptest.NewRequestWithContext(t.Context(), fiber.MethodPost, "/", nil)

	_, err := app.Test(req)
	require.NoError(t, err, "The test request should be served")

	return reached, processErr
}

func TestAuthProcess(t *testing.T) {
	// Pins the trust boundary: a strategy's principal is untrusted input and
	// must never carry a framework-reserved identity.
	t.Run("RejectsTheSystemIdentity", func(t *testing.T) {
		reached, err := runAuthProcess(t, security.PrincipalSystem)

		require.Error(t, err, "A strategy returning the system identity must be rejected")
		assert.False(t, reached, "A rejected request must not reach the handler")

		resErr, ok := result.AsErr(err)
		require.True(t, ok, "The rejection must be a result.Error")
		assert.Equal(t, security.ErrCodePrincipalInvalid, resErr.Code,
			"The rejection must carry the principal-invalid code")
	})

	t.Run("RejectsTheCronJobIdentity", func(t *testing.T) {
		reached, err := runAuthProcess(t, security.NewUser(orm.OperatorCronJob, "impostor"))

		require.Error(t, err, "A strategy returning the cron-job identity must be rejected")
		assert.False(t, reached, "A rejected request must not reach the handler")
	})

	t.Run("RejectsANilPrincipal", func(t *testing.T) {
		reached, err := runAuthProcess(t, nil)

		require.Error(t, err, "A strategy returning no principal must be rejected, not panic downstream")
		assert.False(t, reached, "A rejected request must not reach the handler")
	})

	t.Run("AllowsTheAnonymousIdentity", func(t *testing.T) {
		// The public strategy mints the anonymous identity on every request, so
		// treating it as reserved would close every public endpoint.
		reached, err := runAuthProcess(t, security.NewUser(orm.OperatorAnonymous, "anonymous"))

		require.NoError(t, err, "The anonymous identity must pass the reserved guard")
		assert.True(t, reached, "A public request must reach the handler")
	})

	t.Run("AllowsAnOrdinaryUser", func(t *testing.T) {
		reached, err := runAuthProcess(t, security.NewUser("u1", "Alice"))

		require.NoError(t, err, "An ordinary user must pass the reserved guard")
		assert.True(t, reached, "An authenticated request must reach the handler")
	})
}
