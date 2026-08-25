package apiexport

import (
	"reflect"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// HolidayParams is a request payload shaped like a generated one.
type HolidayParams struct {
	api.P

	ID       string       `json:"id"`
	Name     string       `json:"name" validate:"required,max=128" label:"名称"`
	Remark   *string      `json:"remark,omitempty" validate:"omitempty" label:"备注"`
	Schedule ScheduleSpec `json:"schedule"`
	Tags     []string     `json:"tags,omitempty"`
	Ignored  string       `json:"-"`
	//nolint:unused // present precisely so the manifest is shown to skip it
	internal string
}

// ScheduleSpec is a nested struct the manifest must describe in its own right.
type ScheduleSpec struct {
	StartsAt time.Time `json:"startsAt"`
	EveryMs  int64     `json:"everyMs"`
}

// TestHandlerFunc is the resolver's wrapper shape: the exporter reads a
// handler through this interface rather than reflecting a bare function.
type TestHandlerFunc struct {
	Factory bool
	Value   reflect.Value
}

func (h *TestHandlerFunc) IsFactory() bool { return h.Factory }

func (h *TestHandlerFunc) H() reflect.Value { return h.Value }

// directHandler is the shape a hand-written resource method takes.
func directHandler(fiber.Ctx, HolidayParams) error { return nil }

// factoryHandler is the shape every crud operation takes: a factory that
// receives its dependencies once and returns the request handler.
func factoryHandler(orm.DB) (func(fiber.Ctx, orm.DB, HolidayParams) error, error) {
	return nil, nil
}

func operation(resource, action string, handler any) *api.Operation {
	return &api.Operation{
		Resource: resource, Action: action, Version: "v1",
		Auth: &api.AuthConfig{
			Strategy: api.AuthStrategyBearer,
			Options:  map[string]any{shared.AuthOptionRequiredPermission: "md.holiday.create"},
		},
		Timeout:     30 * time.Second,
		RateLimit:   &api.RateLimitConfig{Max: 100, Period: 5 * time.Minute},
		EnableAudit: true,
		Handler:     handler,
	}
}

func TestBuild(t *testing.T) {
	manifest := Build([]*api.Operation{
		operation("md/holiday", "create", &TestHandlerFunc{Value: reflect.ValueOf(directHandler)}),
		operation("md/allowance", "find_page", &TestHandlerFunc{Factory: true, Value: reflect.ValueOf(factoryHandler)}),
	})

	t.Run("SortsResourcesByName", func(t *testing.T) {
		require.Len(t, manifest.Resources, 2, "each resource must appear once")
		require.Equal(t, "md/allowance", manifest.Resources[0].Name, "resources must be ordered so the manifest is diffable")
		require.Equal(t, "md/holiday", manifest.Resources[1].Name, "resources must be ordered so the manifest is diffable")
	})

	t.Run("CarriesTheOperationContract", func(t *testing.T) {
		op := manifest.Resources[1].Operations[0]

		require.Equal(t, "create", op.Action, "the action names the endpoint")
		require.Equal(t, api.AuthStrategyBearer, op.Auth, "the auth strategy is per operation")
		require.Equal(t, "md.holiday.create", op.Permission, "the permission must be read out of the resolved auth options")
		require.True(t, op.Audit, "the audit flag must survive into the manifest")
		require.Equal(t, int64(30000), op.TimeoutMs, "durations must be reported in milliseconds")
		require.Equal(t, 100, op.RateLimit.Max, "the rate limit budget must be reported")
		require.Equal(t, int64(300000), op.RateLimit.PeriodMs, "the rate limit window must be reported in milliseconds")
	})

	t.Run("FindsTheParamsOfADirectHandler", func(t *testing.T) {
		require.Equal(t, "github.com/coldsmirk/vef-framework-go/internal/apiexport.HolidayParams",
			manifest.Resources[1].Operations[0].Params, "a plain handler's payload must be found among its parameters")
	})

	t.Run("FindsTheParamsBehindAFactory", func(t *testing.T) {
		require.Equal(t, "github.com/coldsmirk/vef-framework-go/internal/apiexport.HolidayParams",
			manifest.Resources[0].Operations[0].Params,
			"a crud operation's payload sits on the function its factory returns, not on the factory")
	})

	t.Run("DescribesTheParamsFields", func(t *testing.T) {
		described := manifest.Types["github.com/coldsmirk/vef-framework-go/internal/apiexport.HolidayParams"]

		require.Equal(t, []Field{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string", Label: "名称", Validate: "required,max=128"},
			{Name: "remark", Type: "string", Optional: true, Label: "备注", Validate: "omitempty"},
			{Name: "schedule", Type: "github.com/coldsmirk/vef-framework-go/internal/apiexport.ScheduleSpec"},
			{Name: "tags", Type: "[]string", Optional: true},
		}, described.Fields, "the manifest must describe the wire shape: json names, optionality, labels and rules")
	})

	t.Run("RecordsNestedStructsInTheirOwnRight", func(t *testing.T) {
		nested, recorded := manifest.Types["github.com/coldsmirk/vef-framework-go/internal/apiexport.ScheduleSpec"]

		require.True(t, recorded, "a struct a payload references must be described too")
		require.Equal(t, []Field{
			{Name: "startsAt", Type: "time.Time"},
			{Name: "everyMs", Type: "int64"},
		}, nested.Fields, "a standard library type must stay a leaf rather than be expanded")
	})

	t.Run("SkipsTheSentinelAndUnexportedFields", func(t *testing.T) {
		described := manifest.Types["github.com/coldsmirk/vef-framework-go/internal/apiexport.HolidayParams"]

		for _, field := range described.Fields {
			require.NotEqual(t, "P", field.Name, "the api.P marker describes decoding, not the contract")
			require.NotEqual(t, "internal", field.Name, "an unexported field never reaches the wire")
			require.NotEqual(t, "-", field.Name, "a json-excluded field never reaches the wire")
		}
	})
}

func TestBuildIsDeterministic(t *testing.T) {
	operations := []*api.Operation{
		operation("md/holiday", "update", &TestHandlerFunc{Value: reflect.ValueOf(directHandler)}),
		operation("md/holiday", "create", &TestHandlerFunc{Value: reflect.ValueOf(directHandler)}),
	}

	first := Build(operations)
	second := Build([]*api.Operation{operations[1], operations[0]})

	require.Equal(t, first, second,
		"a manifest must not depend on the order operations were registered in, or its diff reports phantom changes")
	require.Equal(t, "create", first.Resources[0].Operations[0].Action, "operations must be ordered by action")
}

func TestBuildWithoutParams(t *testing.T) {
	manifest := Build([]*api.Operation{
		operation("sys/monitor", "get_cpu", &TestHandlerFunc{Value: reflect.ValueOf(func(fiber.Ctx) error { return nil })}),
	})

	require.Empty(t, manifest.Resources[0].Operations[0].Params, "an operation taking no payload must report none")
	require.Empty(t, manifest.Types, "an operation taking no payload must contribute no types")
}

// SelfEmbedding embeds a pointer to itself, which is legal, compiling Go and
// the shape a tree or linked-list node takes.
type SelfEmbedding struct {
	*SelfEmbedding

	Value string `json:"value"`
}

// MutualA and MutualB embed each other, the two-type form of the same cycle.
type MutualA struct {
	*MutualB

	Left string `json:"left"`
}

// MutualB completes the cycle with MutualA.
type MutualB struct {
	*MutualA

	Right string `json:"right"`
}

// StampedParams carries a type that marshals itself, which must be reported by
// name rather than expanded.
type StampedParams struct {
	api.P

	Stamp   timex.DateTime `json:"stamp"`
	Nested  SelfEmbedding  `json:"nested"`
	Mutual  MutualA        `json:"mutual"`
	Comment string         `json:"comment"`
}

func stampedHandler(fiber.Ctx, StampedParams) error { return nil }

// TestBuildTerminatesOnEmbeddedCycles pins that a payload reaching a
// self-embedding type is described rather than exhausting the stack. The
// failure this guards against is a fatal runtime error, not an error value —
// it would take down the process the export runs in.
func TestBuildTerminatesOnEmbeddedCycles(t *testing.T) {
	manifest := Build([]*api.Operation{
		operation("md/stamped", "create", &TestHandlerFunc{Value: reflect.ValueOf(stampedHandler)}),
	})

	described := manifest.Types["github.com/coldsmirk/vef-framework-go/internal/apiexport.StampedParams"]
	require.NotEmpty(t, described.Fields, "the payload must still be described")

	t.Run("ReportsASelfMarshalingTypeByName", func(t *testing.T) {
		require.Equal(t, "timex.DateTime", described.Fields[0].Type,
			"a type with its own MarshalText reaches the wire as a string, so listing time.Time's unexported fields would describe an empty object where the API wants a string")
		require.NotContains(t, manifest.Types, "github.com/coldsmirk/vef-framework-go/timex.DateTime",
			"a self-marshaling type must not be recorded as a struct at all")
	})

	t.Run("FlattensACycleOnce", func(t *testing.T) {
		node := manifest.Types["github.com/coldsmirk/vef-framework-go/internal/apiexport.SelfEmbedding"]
		require.Equal(t, []Field{{Name: "value", Type: "string"}}, node.Fields,
			"the embedded pointer to itself contributes nothing the second time around")
	})

	t.Run("FlattensAMutualCycleOnce", func(t *testing.T) {
		mutual := manifest.Types["github.com/coldsmirk/vef-framework-go/internal/apiexport.MutualA"]
		require.Equal(t, []Field{
			{Name: "right", Type: "string"},
			{Name: "left", Type: "string"},
		}, mutual.Fields, "each side of the cycle contributes its own field exactly once")
	})
}

// TestBuildExpandsFirstPartyStructs pins that a business type from a module
// whose path has no dot — which is what `module acme` produces — is expanded
// rather than mistaken for a standard library type.
func TestBuildExpandsFirstPartyStructs(t *testing.T) {
	manifest := Build([]*api.Operation{
		operation("md/holiday", "create", &TestHandlerFunc{Value: reflect.ValueOf(directHandler)}),
	})

	require.Contains(t, manifest.Types, "github.com/coldsmirk/vef-framework-go/internal/apiexport.ScheduleSpec",
		"a plain struct field must be recorded in its own right")
}
