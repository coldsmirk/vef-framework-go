package app_test

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/result"
)

// AppTestSuite tests the app lifecycle and API functionality.
type AppTestSuite struct {
	suite.Suite

	app             *app.App
	stop            func()
	originalI18nEnv string
}

// SetupSuite runs once before all tests in the suite.
func (suite *AppTestSuite) SetupSuite() {
	suite.T().Log("Setting up AppTestSuite - starting test app")

	// Save and clear VEF_I18N_LANGUAGE so i18n initializes with the default
	// language (zh-CN). The original value is restored in TearDownSuite so it
	// outlives every test in the suite.
	suite.originalI18nEnv = os.Getenv("VEF_I18N_LANGUAGE")
	_ = os.Unsetenv("VEF_I18N_LANGUAGE")

	suite.app, suite.stop = apptest.NewTestApp(
		suite.T(),
		fx.Invoke(func() {
			// Re-initialize i18n with default language after clearing env var
			_ = i18n.SetLanguage("")
		}),
		vef.ProvideAPIResource(NewTestResource),
	)

	suite.Require().NotNil(suite.app, "App should be initialized")

	suite.T().Log("AppTestSuite setup complete - test app ready")
}

// TearDownSuite runs once after all tests in the suite.
func (suite *AppTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down AppTestSuite")

	if suite.stop != nil {
		suite.stop()
	}

	// Restore VEF_I18N_LANGUAGE to the value it held before SetupSuite ran,
	// ensuring the env is clean for tests that run after this suite.
	if suite.originalI18nEnv != "" {
		_ = os.Setenv("VEF_I18N_LANGUAGE", suite.originalI18nEnv)
	} else {
		_ = os.Unsetenv("VEF_I18N_LANGUAGE")
	}

	suite.T().Log("AppTestSuite teardown complete")
}

// TestResource is a simple test resource for API testing.
type TestResource struct {
	api.Resource
}

func NewTestResource() api.Resource {
	return &TestResource{
		Resource: api.NewRPCResource(
			"test",
			api.WithOperations(
				api.OperationSpec{
					Action: "ping",
					Public: true,
				},
			),
		),
	}
}

func (*TestResource) Ping(ctx fiber.Ctx) error {
	return result.Ok("pong").Response(ctx)
}

// TestAppLifecycle tests basic app lifecycle.
func (suite *AppTestSuite) TestAppLifecycle() {
	suite.T().Log("Testing app lifecycle (start and stop)")

	suite.Run("StartStop", func() {
		errChan := suite.app.Start()
		err := <-errChan
		suite.NoError(err, "App should start successfully")

		time.Sleep(100 * time.Millisecond)

		err = suite.app.Stop()
		suite.NoError(err, "App should stop successfully")
	})
}

// TestCustomResource tests app with custom API resource.
func (suite *AppTestSuite) TestCustomResource() {
	suite.T().Log("Testing custom API resource")

	suite.Run("PingEndpoint", func() {
		req := httptest.NewRequestWithContext(
			context.Background(),
			fiber.MethodPost,
			"/api",
			strings.NewReader(`{"resource": "test", "action": "ping", "version": "v1"}`),
		)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

		resp, err := suite.app.Test(req, 2*time.Second)
		suite.Require().NoError(err, "API request should not fail")
		suite.Require().NotNil(resp, "Response should not be nil")
		suite.Equal(200, resp.StatusCode, "Ping endpoint should return 200 OK")

		body, err := io.ReadAll(resp.Body)
		suite.Require().NoError(err, "Ping endpoint response body should be readable")
		suite.Equal(`{"code":0,"message":"成功","data":"pong"}`, string(body), "Response body should match expected")
	})
}

// TestAppTestSuite runs the test suite.
func TestApp(t *testing.T) {
	suite.Run(t, new(AppTestSuite))
}
