package param_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cron"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/logx"
	"github.com/coldsmirk/vef-framework-go/mold"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/storage"
)

type ParamResolversTestSuite struct {
	suite.Suite

	app  *app.App
	stop func()
}

func (suite *ParamResolversTestSuite) SetupSuite() {
	suite.T().Log("Setting up ParamResolversTestSuite")

	opts := []fx.Option{
		vef.ProvideAPIResource(NewTestParamResolversResource),
		fx.Replace(&config.StorageConfig{
			Provider: config.StorageMemory,
		}),
	}

	suite.app, suite.stop = apptest.NewTestApp(suite.T(), opts...)

	suite.T().Log("ParamResolversTestSuite setup complete")
}

func (suite *ParamResolversTestSuite) TearDownSuite() {
	if suite.stop != nil {
		suite.stop()
	}
}

func (suite *ParamResolversTestSuite) TestCtxResolver() {
	suite.Run("InjectFiberCtx", func() {
		resp := suite.makeAPIRequest("verify_ctx", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestDBResolver() {
	suite.Run("InjectOrmDB", func() {
		resp := suite.makeAPIRequest("verify_db", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestLoggerResolver() {
	suite.Run("InjectLogger", func() {
		resp := suite.makeAPIRequest("verify_logger", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestPrincipalResolver() {
	suite.Run("InjectPrincipal", func() {
		resp := suite.makeAPIRequest("verify_principal", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestDBFactoryResolver() {
	suite.Run("InjectDBToFactory", func() {
		resp := suite.makeAPIRequest("verify_db_factory", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"Factory_injected":true`, "Response should contain Factory_injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestStorageResolver() {
	suite.Run("InjectStorageService", func() {
		resp := suite.makeAPIRequest("verify_storage", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestStorageFactoryResolver() {
	suite.Run("InjectStorageToFactory", func() {
		resp := suite.makeAPIRequest("verify_storage_factory", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"Factory_injected":true`, "Response should contain Factory_injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestMoldResolver() {
	suite.Run("InjectMoldTransformer", func() {
		resp := suite.makeAPIRequest("verify_mold", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestMoldFactoryResolver() {
	suite.Run("InjectMoldToFactory", func() {
		resp := suite.makeAPIRequest("verify_mold_factory", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"Factory_injected":true`, "Response should contain Factory_injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestEventResolver() {
	suite.Run("InjectEventPublisher", func() {
		resp := suite.makeAPIRequest("verify_event", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestEventFactoryResolver() {
	suite.Run("InjectEventToFactory", func() {
		resp := suite.makeAPIRequest("verify_event_factory", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"Factory_injected":true`, "Response should contain Factory_injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestCronResolver() {
	suite.Run("InjectCronScheduler", func() {
		resp := suite.makeAPIRequest("verify_cron", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"injected":true`, "Response should contain injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestCronFactoryResolver() {
	suite.Run("InjectCronToFactory", func() {
		resp := suite.makeAPIRequest("verify_cron_factory", "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		suite.Contains(suite.readBody(resp), `"Factory_injected":true`, "Response should contain Factory_injected:true")
	})
}

func (suite *ParamResolversTestSuite) TestParamsDecode() {
	suite.Run("DecodesFieldValues", func() {
		resp := suite.makeAPIRequestFull("verify_params_decode", `{"name":"alice","age":30}`, "{}")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		body := suite.readBody(resp)
		suite.Contains(body, `"name":"alice"`, "Decoded name field should match input")
		suite.Contains(body, `"age":30`, "Decoded age field should match input")
	})

	suite.Run("ValidationFailureReturns400Code", func() {
		// name is required; omitting it should trigger a validation error
		resp := suite.makeAPIRequestFull("verify_params_decode", `{"age":5}`, "{}")
		suite.Equal(400, resp.StatusCode, "A missing required field is a client error → HTTP 400")
		body := suite.readBody(resp)
		// ErrCodeBadRequest = 1400
		suite.Contains(body, `"code":1400`, "Validation failure should return bad-request code 1400")
	})

	suite.Run("TypeMismatchReturns400Code", func() {
		// age is int; supplying a string that cannot be decoded should surface as bad-request
		resp := suite.makeAPIRequestFull("verify_params_decode", `{"name":"bob","age":"not-a-number"}`, "{}")
		suite.Equal(400, resp.StatusCode, "A wrong-typed field is a client error → HTTP 400")
		body := suite.readBody(resp)
		suite.Contains(body, `"code":1400`, "Type-mismatch decode failure should return bad-request code 1400, not unknown 1900")
	})
}

func (suite *ParamResolversTestSuite) TestMetaDecode() {
	suite.Run("DecodesFieldValues", func() {
		resp := suite.makeAPIRequestFull("verify_meta_decode", "{}", `{"tag":"hello"}`)
		suite.Equal(200, resp.StatusCode, "Should return 200 OK")
		body := suite.readBody(resp)
		suite.Contains(body, `"tag":"hello"`, "Decoded tag field should match meta input")
	})

	suite.Run("TypeMismatchReturns400Code", func() {
		// tag is string; use a nested object to force a decode failure
		resp := suite.makeAPIRequestFull("verify_meta_decode", "{}", `{"tag":{"nested":true}}`)
		suite.Equal(400, resp.StatusCode, "A wrong-typed meta field is a client error → HTTP 400")
		body := suite.readBody(resp)
		suite.Contains(body, `"code":1400`, "Meta type-mismatch decode failure should return bad-request code 1400")
	})
}

func (suite *ParamResolversTestSuite) makeAPIRequest(action, body string) *http.Response {
	return suite.makeAPIRequestFull(action, body, "{}")
}

func (suite *ParamResolversTestSuite) makeAPIRequestFull(action, params, meta string) *http.Response {
	fullBody := `{"resource": "test/param_resolvers", "action": "` + action + `", "version": "v1", "params": ` + params + `, "meta": ` + meta + `}`

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader(fullBody))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.app.Test(req, 30*time.Second)
	suite.Require().NoError(err, "API request should complete without error")

	return resp
}

func (suite *ParamResolversTestSuite) readBody(resp *http.Response) string {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "reading response body should succeed")

	return string(body)
}

// TestParamResolversTestSuite tests param resolvers test suite scenarios.
func TestParamResolvers(t *testing.T) {
	suite.Run(t, new(ParamResolversTestSuite))
}

// Resource Definition

type TestParamResolversResource struct {
	api.Resource
}

func NewTestParamResolversResource() api.Resource {
	return &TestParamResolversResource{
		Resource: api.NewRPCResource(
			"test/param_resolvers",
			api.WithVersion(api.VersionV1),
			api.WithOperations(
				api.OperationSpec{Action: "verify_ctx", Public: true},
				api.OperationSpec{Action: "verify_db", Public: true},
				api.OperationSpec{Action: "verify_logger", Public: true},
				api.OperationSpec{Action: "verify_principal", Public: true},
				api.OperationSpec{Action: "verify_db_factory", Public: true},
				api.OperationSpec{Action: "verify_storage", Public: true},
				api.OperationSpec{Action: "verify_storage_factory", Public: true},
				api.OperationSpec{Action: "verify_mold", Public: true},
				api.OperationSpec{Action: "verify_mold_factory", Public: true},
				api.OperationSpec{Action: "verify_event", Public: true},
				api.OperationSpec{Action: "verify_event_factory", Public: true},
				api.OperationSpec{Action: "verify_cron", Public: true},
				api.OperationSpec{Action: "verify_cron_factory", Public: true},
				// param-decoding behavior tests
				api.OperationSpec{Action: "verify_params_decode", Public: true},
				api.OperationSpec{Action: "verify_meta_decode", Public: true},
			),
		),
	}
}

func (*TestParamResolversResource) VerifyCtx(ctx fiber.Ctx) error {
	injected := ctx != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyDB(ctx fiber.Ctx, db orm.DB) error {
	injected := db != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyLogger(ctx fiber.Ctx, logger logx.Logger) error {
	injected := logger != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyPrincipal(ctx fiber.Ctx, principal *security.Principal) error {
	injected := principal != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyDbFactory(db orm.DB) func(ctx fiber.Ctx) error {
	injected := db != nil

	return func(ctx fiber.Ctx) error {
		return result.Ok(map[string]any{"Factory_injected": injected}).Response(ctx)
	}
}

func (*TestParamResolversResource) VerifyStorage(ctx fiber.Ctx, service storage.Service) error {
	injected := service != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyStorageFactory(service storage.Service) func(ctx fiber.Ctx) error {
	injected := service != nil

	return func(ctx fiber.Ctx) error {
		return result.Ok(map[string]any{"Factory_injected": injected}).Response(ctx)
	}
}

func (*TestParamResolversResource) VerifyMold(ctx fiber.Ctx, transformer mold.Transformer) error {
	injected := transformer != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyMoldFactory(transformer mold.Transformer) func(ctx fiber.Ctx) error {
	injected := transformer != nil

	return func(ctx fiber.Ctx) error {
		return result.Ok(map[string]any{"Factory_injected": injected}).Response(ctx)
	}
}

func (*TestParamResolversResource) VerifyEvent(ctx fiber.Ctx, publisher event.Bus) error {
	injected := publisher != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyEventFactory(publisher event.Bus) func(ctx fiber.Ctx) error {
	injected := publisher != nil

	return func(ctx fiber.Ctx) error {
		return result.Ok(map[string]any{"Factory_injected": injected}).Response(ctx)
	}
}

func (*TestParamResolversResource) VerifyCron(ctx fiber.Ctx, scheduler cron.Scheduler) error {
	injected := scheduler != nil

	return result.Ok(map[string]any{"injected": injected}).Response(ctx)
}

func (*TestParamResolversResource) VerifyCronFactory(scheduler cron.Scheduler) func(ctx fiber.Ctx) error {
	injected := scheduler != nil

	return func(ctx fiber.Ctx) error {
		return result.Ok(map[string]any{"Factory_injected": injected}).Response(ctx)
	}
}

// TestParamsDecodeParams is the typed api.P params struct used for decode tests.
type TestParamsDecodeParams struct {
	api.P

	Name string `json:"name" validate:"required"`
	Age  int    `json:"age"`
}

// TestMetaDecodeParams is the typed api.M meta struct used for decode tests.
type TestMetaDecodeParams struct {
	api.M

	Tag string `json:"tag"`
}

func (*TestParamResolversResource) VerifyParamsDecode(ctx fiber.Ctx, p TestParamsDecodeParams) error {
	return result.Ok(map[string]any{"name": p.Name, "age": p.Age}).Response(ctx)
}

func (*TestParamResolversResource) VerifyMetaDecode(ctx fiber.Ctx, m TestMetaDecodeParams) error {
	return result.Ok(map[string]any{"tag": m.Tag}).Response(ctx)
}
