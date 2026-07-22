package app_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net/http"
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

// TestBodyEncoding proves the transport body-encoding paths compose with the
// real middleware stack (CORS, the content-type guard, the dispatcher) and
// reach the handler with the decoded request. It guards the middleware ordering
// against future reordering.
func (suite *AppTestSuite) TestBodyEncoding() {
	const (
		pingReq  = `{"resource": "test", "action": "ping", "version": "v1"}`
		wantBody = `{"code":0,"message":"成功","data":"pong"}`
	)

	gzipJSON := func(payload string) []byte {
		var buf bytes.Buffer

		writer := gzip.NewWriter(&buf)
		_, err := writer.Write([]byte(payload))
		suite.Require().NoError(err, "gzip write should not fail")
		suite.Require().NoError(writer.Close(), "gzip close should not fail")

		return buf.Bytes()
	}

	send := func(header, value string, body []byte) *http.Response {
		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", bytes.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

		if header != "" {
			req.Header.Set(header, value)
		}

		resp, err := suite.app.Test(req, 2*time.Second)
		suite.Require().NoError(err, "API request should not fail")

		return resp
	}

	assertPong := func(resp *http.Response) {
		suite.Require().Equal(200, resp.StatusCode, "the decoded request must dispatch")

		body, err := io.ReadAll(resp.Body)
		suite.Require().NoError(err, "response body should be readable")
		suite.Equal(wantBody, string(body), "the handler must run on the decoded body")
	}

	suite.Run("NativeContentEncodingGzip", func() {
		// Tier 0: Fiber decompresses Content-Encoding itself, no custom middleware.
		assertPong(send(fiber.HeaderContentEncoding, "gzip", gzipJSON(pingReq)))
	})

	suite.Run("Base64", func() {
		assertPong(send(api.HeaderXBodyEncoding, "base64", []byte(base64.StdEncoding.EncodeToString([]byte(pingReq)))))
	})

	suite.Run("GzipBase64", func() {
		assertPong(send(api.HeaderXBodyEncoding, "gzip+base64", []byte(base64.StdEncoding.EncodeToString(gzipJSON(pingReq)))))
	})

	suite.Run("UnsupportedEncoding", func() {
		resp := send(api.HeaderXBodyEncoding, "rot13", []byte(pingReq))
		suite.Equal(400, resp.StatusCode, "an unknown encoding is rejected before dispatch")
	})
}

// TestAppTestSuite runs the test suite.
func TestApp(t *testing.T) {
	suite.Run(t, new(AppTestSuite))
}
