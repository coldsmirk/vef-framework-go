package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/integration/auth"
	"github.com/coldsmirk/vef-framework-go/internal/integration/exec"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
	"github.com/coldsmirk/vef-framework-go/internal/integration/worker"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// PatientInfo is the standard model used by the typed-call assertions.
type PatientInfo struct {
	Name   string `json:"name"`
	Gender string `json:"gender"`
}

// ModuleTestSuite boots the full framework with the integration module
// against an in-memory SQLite primary and a live httptest upstream.
type ModuleTestSuite struct {
	suite.Suite

	cleanup  func()
	db       orm.DB
	invoker  integration.Invoker
	concrete *exec.Invoker
	codec    *service.SecretCodec
	registry *auth.Registry

	upstream     *httptest.Server
	seenAuth     string
	seenBody     []byte
	countedCalls int
}

func TestModuleSuite(t *testing.T) {
	suite.Run(t, new(ModuleTestSuite))
}

func (s *ModuleTestSuite) SetupSuite() {
	mux := http.NewServeMux()

	mux.HandleFunc("/patients/query", func(w http.ResponseWriter, r *http.Request) {
		s.seenAuth = r.Header.Get("Authorization")
		s.seenBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"brxm":"张三","xb":"1"}`))
	})

	mux.HandleFunc("/whoami", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	mux.HandleFunc("/fail", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"msg":"boom"}`))
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)

		_, _ = w.Write([]byte(`{}`))
	})

	mux.HandleFunc("/counted", func(w http.ResponseWriter, _ *http.Request) {
		s.countedCalls++

		_, _ = w.Write([]byte(`{"n":1}`))
	})

	s.upstream = httptest.NewServer(mux)

	_, s.cleanup = apptest.NewTestApp(s.T(),
		fx.Replace(&config.IntegrationConfig{
			AutoMigrate: true,
			SecretKey:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)),
			Log:         config.IntegrationLogConfig{Mode: config.IntegrationLogAll},
		}),
		fx.Provide(func() context.Context { return context.Background() }),
		Module,
		fx.Populate(&s.db, &s.invoker, &s.concrete, &s.codec, &s.registry),
	)
}

func (s *ModuleTestSuite) TearDownSuite() {
	if s.cleanup != nil {
		s.cleanup()
	}

	s.upstream.Close()
}

// --- Seed helpers ---

func (s *ModuleTestSuite) createContract(code string, input, output json.RawMessage) *integration.Contract {
	contract := &integration.Contract{
		Code:         code,
		Name:         code,
		InputSchema:  input,
		OutputSchema: output,
		IsEnabled:    true,
	}

	_, err := s.db.NewInsert().Model(contract).Exec(s.T().Context())
	s.Require().NoError(err, "Contract seed should insert")

	return contract
}

func (s *ModuleTestSuite) createSystem(code string, authCfg *integration.AuthConfig) *integration.System {
	if authCfg != nil {
		scheme, ok := s.registry.Resolve(authCfg)
		s.Require().True(ok, "Seed auth scheme should resolve")
		s.Require().NoError(s.codec.EncryptAuth(scheme, authCfg, nil), "Seed auth should encrypt")
	}

	system := &integration.System{
		Code:      code,
		Name:      code,
		BaseURL:   s.upstream.URL,
		Auth:      authCfg,
		Params:    map[string]string{"branch": "east-01"},
		IsEnabled: true,
	}

	_, err := s.db.NewInsert().Model(system).Exec(s.T().Context())
	s.Require().NoError(err, "System seed should insert")

	return system
}

func (s *ModuleTestSuite) createAdapter(system *integration.System, contract *integration.Contract, script string) *integration.Adapter {
	adapter := &integration.Adapter{
		SystemID:   system.ID,
		ContractID: contract.ID,
		Script:     script,
		IsEnabled:  true,
	}

	_, err := s.db.NewInsert().Model(adapter).Exec(s.T().Context())
	s.Require().NoError(err, "Adapter seed should insert")

	return adapter
}

func (s *ModuleTestSuite) createRoute(key, contractID, systemID string) {
	route := &integration.Route{
		RouteKey:   key,
		ContractID: contractID,
		SystemID:   systemID,
		IsEnabled:  true,
	}

	_, err := s.db.NewInsert().Model(route).Exec(s.T().Context())
	s.Require().NoError(err, "Route seed should insert")
}

func (s *ModuleTestSuite) findLogs(contractCode string) []integration.InvocationLog {
	var logs []integration.InvocationLog

	err := s.db.NewSelect().
		Model(&logs).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("contract_code", contractCode)
		}).
		Scan(s.T().Context())
	s.Require().NoError(err, "Log query should succeed")

	return logs
}

var (
	patientInputSchema  = json.RawMessage(`{"type":"object","properties":{"idCardNo":{"type":"string"}},"required":["idCardNo"]}`)
	patientOutputSchema = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"gender":{"type":"string"}},"required":["name","gender"]}`)
)

const patientAdapterScript = `
const resp = http.post('/patients/query', { zjhm: input.idCardNo, branch: system.params.branch })
if (!resp.ok) errors.upstream('HIS returned ' + resp.status)
const d = resp.json()
return { name: d.brxm, gender: d.xb === '1' ? 'male' : 'female' }
`

// --- Tests ---

func (s *ModuleTestSuite) TestInvoke() {
	contract := s.createContract("patient.get", patientInputSchema, patientOutputSchema)
	system := s.createSystem("his-a", &integration.AuthConfig{
		Scheme: auth.SchemeBearer,
		Params: map[string]string{"token": "his-token"},
	})
	s.createAdapter(system, contract, patientAdapterScript)

	s.Run("StandardModelReturned", func() {
		result, err := s.invoker.Invoke(s.T().Context(), "patient.get",
			map[string]any{"idCardNo": "110101199001010011"},
			integration.WithSystem("his-a"),
		)
		s.Require().NoError(err, "Invocation should succeed")

		s.Equal("his-a", result.System(), "Result should carry the serving system")
		s.False(result.Cached(), "Fresh invocation should not be cached")

		var patient PatientInfo
		s.Require().NoError(result.Decode(&patient), "Output should decode into the business struct")
		s.Equal("张三", patient.Name, "Adapter should map the vendor field")
		s.Equal("male", patient.Gender, "Adapter should translate the vendor code")
	})

	s.Run("CredentialsDecryptedForUpstreamOnly", func() {
		s.Equal("Bearer his-token", s.seenAuth, "Upstream should receive the decrypted bearer token")

		stored := new(integration.System)
		err := s.db.NewSelect().Model(stored).Where(func(cb orm.ConditionBuilder) {
			cb.Equals("code", "his-a")
		}).Scan(s.T().Context())
		s.Require().NoError(err, "Stored system should load")
		s.Contains(stored.Auth.Params["token"], "enc:", "Stored token should be encrypted at rest")
	})

	s.Run("ScriptBindingsReachUpstream", func() {
		s.Contains(string(s.seenBody), "east-01", "system.params should be visible to the script")
		s.Contains(string(s.seenBody), "110101199001010011", "input should be visible to the script")
	})

	s.Run("TypedCallDecodes", func() {
		patient, err := integration.Call[PatientInfo](s.T().Context(), s.invoker, "patient.get",
			map[string]any{"idCardNo": "110101199001010011"},
			integration.WithSystem("his-a"),
		)
		s.Require().NoError(err, "Typed call should succeed")
		s.Equal("张三", patient.Name, "Typed call should decode the standard model")
	})

	s.Run("InvocationLogged", func() {
		logs := s.findLogs("patient.get")
		s.Require().NotEmpty(logs, "Success should be logged in mode=all")

		entry := logs[0]
		s.Equal(integration.FailureKind(""), entry.FailureKind, "Success should carry no failure kind")
		s.Require().NotEmpty(entry.HTTPTrace, "Wire trace should be captured")
		s.Equal(integration.MaskedSecret, entry.HTTPTrace[0].RequestHeaders["authorization"],
			"Captured Authorization header must be masked")
	})

	s.Run("StatsRecorded", func() {
		stats := s.concrete.Stats()
		s.Require().NotEmpty(stats, "Stats should have entries")

		found := false
		for _, stat := range stats {
			if stat.System == "his-a" && stat.Contract == "patient.get" {
				found = true

				s.Positive(stat.Successes, "Successes should be counted")
			}
		}

		s.True(found, "Stats should carry the (system, contract) pair")
	})
}

func (s *ModuleTestSuite) TestRouting() {
	echoScript := `return { sys: system.code }`

	c1 := s.createContract("route.c1", nil, nil)
	c2 := s.createContract("route.c2", nil, nil)
	sysA := s.createSystem("route-sys-a", nil)
	sysB := s.createSystem("route-sys-b", nil)
	sysC := s.createSystem("route-sys-c", nil)

	for _, system := range []*integration.System{sysA, sysB, sysC} {
		s.createAdapter(system, c1, echoScript)
		s.createAdapter(system, c2, echoScript)
	}

	s.createRoute("east", c1.ID, sysA.ID) // exact: c1 + east
	s.createRoute("east", "", sysB.ID)    // wildcard: any contract + east
	s.createRoute("", "", sysC.ID)        // default route

	servedBy := func(contract string, opts ...integration.InvokeOption) string {
		result, err := s.invoker.Invoke(s.T().Context(), contract, nil, opts...)
		s.Require().NoError(err, "Routed invocation should succeed")

		return result.System()
	}

	s.Run("ExactMatchWins", func() {
		s.Equal("route-sys-a", servedBy("route.c1", integration.WithRoute("east")),
			"Exact (key, contract) rule should win")
	})

	s.Run("WildcardCoversOtherContracts", func() {
		s.Equal("route-sys-b", servedBy("route.c2", integration.WithRoute("east")),
			"Contract-wildcard rule should serve other contracts")
	})

	s.Run("DefaultRouteWithoutTarget", func() {
		s.Equal("route-sys-c", servedBy("route.c1"),
			"No target option should resolve the empty route key")
	})

	s.Run("UnknownKeyFails", func() {
		_, err := s.invoker.Invoke(s.T().Context(), "route.c1", nil, integration.WithRoute("west"))
		s.Require().Error(err, "Unmatched route key should fail")
		s.ErrorIs(err, integration.ErrRouteNotFound, "Error should be route-not-found")
	})

	s.Run("AmbiguousTargetFails", func() {
		_, err := s.invoker.Invoke(s.T().Context(), "route.c1", nil,
			integration.WithSystem("route-sys-a"), integration.WithRoute("east"))
		s.Require().Error(err, "Both target options should fail")
		s.ErrorIs(err, integration.ErrTargetAmbiguous, "Error should be target-ambiguous")
	})
}

func (s *ModuleTestSuite) TestFailureClassification() {
	contract := s.createContract("classify.op", patientInputSchema, patientOutputSchema)
	system := s.createSystem("classify-sys", nil)

	invoke := func(script string, input any, opts ...integration.InvokeOption) error {
		adapter := s.createAdapter(system, contract, script)
		defer func() {
			_, err := s.db.NewDelete().Model(adapter).WherePK().Exec(s.T().Context())
			s.Require().NoError(err, "Adapter cleanup should succeed")
		}()

		opts = append(opts, integration.WithSystem("classify-sys"))
		_, err := s.invoker.Invoke(s.T().Context(), "classify.op", input, opts...)

		return err
	}

	validInput := map[string]any{"idCardNo": "1"}

	s.Run("InputInvalid", func() {
		err := invoke(patientAdapterScript, map[string]any{"wrong": true})
		s.Require().Error(err, "Schema-violating input should fail")
		s.ErrorIs(err, integration.ErrInputInvalid(""), "Error should carry the input-invalid code")
	})

	s.Run("UpstreamFailure", func() {
		err := invoke(`
			const resp = http.get('/fail')
			if (!resp.ok) errors.upstream('vendor said ' + resp.json().msg)
			return {}
		`, validInput)
		s.Require().Error(err, "Upstream 500 should fail")
		s.ErrorIs(err, integration.ErrUpstreamFailed(""), "Error should carry the upstream code")
		s.Contains(err.Error(), "boom", "Upstream message should surface")
	})

	s.Run("OutputInvalid", func() {
		err := invoke(`return { unexpected: 1 }`, validInput)
		s.Require().Error(err, "Schema-violating output should fail")
		s.ErrorIs(err, integration.ErrOutputInvalid(""), "Error should carry the output-invalid code")
	})

	s.Run("ScriptFailure", func() {
		err := invoke(`const x = null; return x.field`, validInput)
		s.Require().Error(err, "Throwing script should fail")
		s.ErrorIs(err, integration.ErrScriptFailed(""), "Error should carry the script code")
	})

	s.Run("Timeout", func() {
		err := invoke(`http.get('/slow'); return { name: 'x', gender: 'male' }`, validInput,
			integration.WithTimeout(80*time.Millisecond))
		s.Require().Error(err, "Slow upstream should exceed the run timeout")
		s.ErrorIs(err, integration.ErrInvocationTimeout, "Error should be the timeout sentinel")
	})

	s.Run("FailureKindsLogged", func() {
		kinds := make(map[integration.FailureKind]bool)
		for _, entry := range s.findLogs("classify.op") {
			kinds[entry.FailureKind] = true
		}

		for _, kind := range []integration.FailureKind{
			integration.FailureInputInvalid,
			integration.FailureUpstream,
			integration.FailureOutputInvalid,
			integration.FailureScript,
			integration.FailureTimeout,
		} {
			s.True(kinds[kind], "Failure kind %q should be logged", kind)
		}
	})
}

func (s *ModuleTestSuite) TestDefinitionGuards() {
	s.Run("UnknownContract", func() {
		_, err := s.invoker.Invoke(s.T().Context(), "no.such.contract", nil, integration.WithSystem("x"))
		s.Require().Error(err, "Unknown contract should fail")
		s.ErrorIs(err, integration.ErrContractNotFound, "Error should be contract-not-found")
	})

	s.Run("DisabledSystem", func() {
		contract := s.createContract("guard.op", nil, nil)
		system := s.createSystem("guard-sys", nil)
		s.createAdapter(system, contract, `return {}`)

		_, err := s.db.NewUpdate().Model(system).Set("is_enabled", false).WherePK().Exec(s.T().Context())
		s.Require().NoError(err, "System disable should persist")

		_, err = s.invoker.Invoke(s.T().Context(), "guard.op", nil, integration.WithSystem("guard-sys"))
		s.Require().Error(err, "Disabled system should refuse invocations")
		s.ErrorIs(err, integration.ErrSystemDisabled, "Error should be system-disabled")
	})

	s.Run("MissingAdapter", func() {
		s.createContract("guard.no-adapter", nil, nil)
		s.createSystem("guard-sys-2", nil)

		_, err := s.invoker.Invoke(s.T().Context(), "guard.no-adapter", nil, integration.WithSystem("guard-sys-2"))
		s.Require().Error(err, "Missing adapter should fail")
		s.ErrorIs(err, integration.ErrAdapterNotFound, "Error should be adapter-not-found")
	})
}

func (s *ModuleTestSuite) TestResponseCache() {
	contract := s.createContract("cache.op", nil, nil)
	system := s.createSystem("cache-sys", nil)
	s.createAdapter(system, contract, `return http.get('/counted').json()`)

	s.countedCalls = 0
	input := map[string]any{"q": 1}

	first, err := s.invoker.Invoke(s.T().Context(), "cache.op", input,
		integration.WithSystem("cache-sys"), integration.WithCache(time.Minute))
	s.Require().NoError(err, "First cached invocation should succeed")
	s.False(first.Cached(), "First invocation should hit the upstream")

	second, err := s.invoker.Invoke(s.T().Context(), "cache.op", input,
		integration.WithSystem("cache-sys"), integration.WithCache(time.Minute))
	s.Require().NoError(err, "Second cached invocation should succeed")
	s.True(second.Cached(), "Second invocation should come from the cache")
	s.Equal(first.Output(), second.Output(), "Cached output should match the original")
	s.Equal(1, s.countedCalls, "Upstream should be hit exactly once with caching")

	_, err = s.invoker.Invoke(s.T().Context(), "cache.op", input, integration.WithSystem("cache-sys"))
	s.Require().NoError(err, "Uncached invocation should succeed")
	s.Equal(2, s.countedCalls, "Without WithCache every invocation hits the upstream")
}

func (s *ModuleTestSuite) TestDryRun() {
	contract := s.createContract("dry.op", nil, patientOutputSchema)
	system := s.createSystem("dry-sys", &integration.AuthConfig{
		Scheme: auth.SchemeBearer,
		Params: map[string]string{"token": "dry-token"},
	})

	s.Run("UnsavedScriptRuns", func() {
		result := s.concrete.DryRun(s.T().Context(), contract, system, `
			const d = http.post('/patients/query', { probe: true }).json()
			return { name: d.brxm, gender: 'male' }
		`, nil)

		s.Empty(result.Error, "Dry run should succeed")
		s.Empty(result.FailureKind, "Dry run should carry no failure kind")
		s.Require().NotEmpty(result.Trace, "Dry run should capture the wire trace")
		s.Equal(integration.MaskedSecret, result.Trace[0].RequestHeaders["authorization"],
			"Dry-run trace must mask credentials")
		s.NotNil(result.Output, "Dry run should return the output")
	})

	s.Run("FailureIsClassifiedWithTrace", func() {
		result := s.concrete.DryRun(s.T().Context(), contract, system, `
			const resp = http.get('/fail')
			if (!resp.ok) errors.upstream('nope')
			return {}
		`, nil)

		s.Equal(integration.FailureUpstream, result.FailureKind, "Dry-run failure should be classified")
		s.NotEmpty(result.Trace, "Failed dry run should still carry the trace")
	})
}

func (s *ModuleTestSuite) TestConnectionProbe() {
	system := s.createSystem("probe-sys", nil)

	s.Run("Reachable", func() {
		check, err := s.concrete.TestConnection(s.T().Context(), system, "", "/whoami")
		s.Require().NoError(err, "Probe should not error on a live upstream")
		s.Require().NotNil(check.HTTP, "Base-url system should get an HTTP probe")
		s.True(check.HTTP.Reachable, "Live upstream should be reachable")
		s.Equal(http.StatusOK, check.HTTP.Status, "Probe should report the status")
		s.Nil(check.Database, "System without a data source should get no database probe")
	})

	s.Run("Unreachable", func() {
		dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		dead.Close()

		system := &integration.System{Code: "dead-sys", Name: "dead", BaseURL: dead.URL, IsEnabled: true}

		check, err := s.concrete.TestConnection(s.T().Context(), system, "", "/")
		s.Require().NoError(err, "Transport failure is data, not an error")
		s.Require().NotNil(check.HTTP, "Dead upstream should still get an HTTP probe")
		s.False(check.HTTP.Reachable, "Dead upstream should be unreachable")
		s.NotEmpty(check.HTTP.Error, "Probe should report the transport error")
	})
}

func (s *ModuleTestSuite) TestDatabaseSystem() {
	contract := s.createContract("db.op", nil, nil)

	system := &integration.System{
		Code:       "db-sys",
		Name:       "db-sys",
		DataSource: &integration.DataSourceConfig{Kind: config.SQLite},
		IsEnabled:  true,
	}

	_, err := s.db.NewInsert().Model(system).Exec(s.T().Context())
	s.Require().NoError(err, "Database system seed should insert")

	s.createAdapter(system, contract, `return { answer: sql.queryOne('SELECT 42 AS answer').answer }`)

	s.Run("ScopedSQLQueries", func() {
		result, err := s.invoker.Invoke(s.T().Context(), "db.op", nil, integration.WithSystem("db-sys"))
		s.Require().NoError(err, "Database-backed invocation should succeed")

		output, ok := result.Output().(map[string]any)
		s.Require().True(ok, "Output should be the standard model object")
		s.InEpsilon(float64(42), output["answer"], 0, "sql.queryOne should reach the vendor database")
	})

	s.Run("WritesAreRejected", func() {
		s.createAdapter(s.createSystemForScript("db-sys-w"), contract, `sql.exec('CREATE TABLE x (y INTEGER)'); return {}`)

		_, err := s.invoker.Invoke(s.T().Context(), "db.op", nil, integration.WithSystem("db-sys-w"))
		s.Require().Error(err, "Write through the scoped sql lib should fail")
		s.ErrorIs(err, integration.ErrScriptFailed(""), "Read-only violation should classify as a script failure")
	})

	s.Run("DatabaseProbe", func() {
		check, err := s.concrete.TestConnection(s.T().Context(), system, "", "")
		s.Require().NoError(err, "Database probe should not error")
		s.Require().NotNil(check.Database, "Data-source system should get a database probe")
		s.True(check.Database.Reachable, "In-memory SQLite should be reachable")
		s.NotEmpty(check.Database.Version, "Probe should report the server version")
		s.Nil(check.HTTP, "System without a base URL should get no HTTP probe")
	})

	s.Run("ReleaseSystemUnregisters", func() {
		s.Require().NoError(s.concrete.ReleaseSystem(s.T().Context(), "db-sys"), "Release should succeed")

		result, err := s.invoker.Invoke(s.T().Context(), "db.op", nil, integration.WithSystem("db-sys"))
		s.Require().NoError(err, "Invocation after release should lazily re-register the source")
		s.NotNil(result.Output(), "Re-registered source should serve queries")
	})
}

// createSystemForScript seeds a database-only system for a single subtest.
func (s *ModuleTestSuite) createSystemForScript(code string) *integration.System {
	system := &integration.System{
		Code:       code,
		Name:       code,
		DataSource: &integration.DataSourceConfig{Kind: config.SQLite},
		IsEnabled:  true,
	}

	_, err := s.db.NewInsert().Model(system).Exec(s.T().Context())
	s.Require().NoError(err, "Database system seed should insert")

	return system
}

func (s *ModuleTestSuite) TestDiagnoseRoutes() {
	echoScript := `return {}`

	healthy := s.createContract("diag.ok", nil, nil)
	orphan := s.createContract("diag.orphan", nil, nil)
	s.createContract("diag.uncovered", nil, nil)
	sysUp := s.createSystem("diag-sys-up", nil)
	sysDown := s.createSystem("diag-sys-down", nil)

	s.createAdapter(sysUp, healthy, echoScript)

	_, err := s.db.NewUpdate().Model(sysDown).Set("is_enabled", false).WherePK().Exec(s.T().Context())
	s.Require().NoError(err, "System disable should persist")

	s.createRoute("diag-east", healthy.ID, sysUp.ID) // healthy: adapter exists
	s.createRoute("diag-east", orphan.ID, sysUp.ID)  // dangling: no adapter for orphan
	s.createRoute("diag-west", "", sysDown.ID)       // disabled system + wildcard gaps
	s.createRoute("diag-south", healthy.ID, sysDown.ID)

	report, err := service.DiagnoseRoutes(s.T().Context(), s.db)
	s.Require().NoError(err, "Diagnosis should succeed")

	type probe struct {
		kind     integration.RouteFindingKind
		key      string
		contract string
		system   string
	}

	seen := make(map[probe]bool)
	for _, f := range report.Findings {
		seen[probe{kind: f.Kind, key: f.RouteKey, contract: f.ContractCode, system: f.SystemCode}] = true
	}

	s.Run("DanglingAdapterReported", func() {
		s.True(seen[probe{integration.RouteFindingDanglingAdapter, "diag-east", "diag.orphan", "diag-sys-up"}],
			"Contract-scoped route without an adapter should be reported")
	})

	s.Run("DisabledSystemReported", func() {
		s.True(seen[probe{integration.RouteFindingDisabledSystem, "diag-west", "", "diag-sys-down"}],
			"Wildcard route to a disabled system should be reported")
		s.True(seen[probe{integration.RouteFindingDisabledSystem, "diag-south", "diag.ok", "diag-sys-down"}],
			"Contract-scoped route to a disabled system should be reported")
	})

	s.Run("WildcardGapReported", func() {
		s.True(seen[probe{integration.RouteFindingWildcardGap, "diag-west", "diag.orphan", "diag-sys-down"}],
			"Wildcard route should report contracts its system cannot serve")
	})

	s.Run("UncoveredContractReported", func() {
		s.True(seen[probe{integration.RouteFindingUncoveredContract, "diag-east", "diag.uncovered", ""}],
			"An exact-only key should report contracts it does not cover")

		s.False(seen[probe{integration.RouteFindingUncoveredContract, "diag-east", "diag.orphan", ""}],
			"A contract with an exact rule under the key should not be reported, even a dangling one")

		s.False(seen[probe{integration.RouteFindingUncoveredContract, "diag-west", "diag.uncovered", ""}],
			"A key with a wildcard rule covers every contract")
	})

	s.Run("HealthyPairSilent", func() {
		for f := range seen {
			if f.contract == "diag.ok" && f.system == "diag-sys-up" {
				s.Failf("unexpected finding", "healthy route should produce no finding, got %+v", f)
			}
		}
	})
}

func (s *ModuleTestSuite) TestLogRetention() {
	insertLog := func(age time.Duration) *integration.InvocationLog {
		entry := &integration.InvocationLog{
			SystemCode:   "retention-sys",
			ContractCode: "retention.op",
		}

		_, err := s.db.NewInsert().Model(entry).Exec(s.T().Context())
		s.Require().NoError(err, "Log seed should insert")

		if age > 0 {
			_, err = s.db.NewUpdate().Model(entry).
				Set("created_at", time.Now().Add(-age)).
				WherePK().
				Exec(s.T().Context())
			s.Require().NoError(err, "Log backdating should persist")
		}

		return entry
	}

	old := insertLog(48 * time.Hour)
	fresh := insertLog(0)

	pruner := worker.NewLogPruner(s.db, &config.IntegrationConfig{
		Log: config.IntegrationLogConfig{Retention: 24 * time.Hour},
	})
	pruner.Run(s.T().Context())

	remaining := s.findLogs("retention.op")
	s.Require().Len(remaining, 1, "Only the fresh row should survive the sweep")
	s.Equal(fresh.ID, remaining[0].ID, "The fresh row should survive")
	s.NotEqual(old.ID, remaining[0].ID, "The aged row should be pruned")
}
