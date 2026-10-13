package resource_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"

	vef "github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// stubGlobals is the snapshot StubGlobalsResolver resolves for every instance.
// The framework default resolves nothing at all, so finding this map persisted
// on the instance can only mean the host override ran.
var stubGlobals = map[string]any{"quotaLimit": float64(8000), "region": "north"}

// StubGlobalsResolver is a host approval.InstanceGlobalsResolver that records
// what the framework asked it to resolve. Recording the arguments is the point:
// a resolver that merely returned a constant would prove the override was
// wired, but not that the instance's own flow and the authenticated principal
// reached it — which is what a real resolver derives its answer from.
type StubGlobalsResolver struct {
	mu           sync.Mutex
	seenFlowCode string
	seenUserID   string
}

func (r *StubGlobalsResolver) Resolve(_ context.Context, principal *security.Principal, flowCode string) (map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seenFlowCode = flowCode

	if principal != nil {
		r.seenUserID = principal.ID
	}

	return stubGlobals, nil
}

// FormGlobalKeys exposes "region" — and only it — to instance forms, so the
// form-facing views must carry region and never quotaLimit.
func (*StubGlobalsResolver) FormGlobalKeys(string) []string {
	return []string{"region"}
}

func (r *StubGlobalsResolver) seen() (flowCode, userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.seenFlowCode, r.seenUserID
}

// GlobalsResolverOverrideTestSuite proves vef.ProvideApprovalGlobalsResolver
// replaces the framework's no-op resolver in a composed app graph, and that
// what it returns is snapshotted onto the instance the condition evaluator
// later reads.
type GlobalsResolverOverrideTestSuite struct {
	apptest.Suite

	ctx        context.Context
	db         orm.DB
	token      string
	categoryID string
	resolver   *StubGlobalsResolver
}

func TestGlobalsResolverOverride(t *testing.T) {
	suite.Run(t, new(GlobalsResolverOverrideTestSuite))
}

func (s *GlobalsResolverOverrideTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.resolver = new(StubGlobalsResolver)

	// Override the framework's default resolver through the real public helper
	// — the graph under test.
	s.db, s.token = setupResourceApp(&s.Suite,
		vef.ProvideApprovalGlobalsResolver(func() approval.InstanceGlobalsResolver { return s.resolver }),
	)

	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "globals-override-cat",
		Name:     "Globals Override Category",
		IsActive: true,
	}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = cat.ID
}

func (s *GlobalsResolverOverrideTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
	s.TearDownApp()
}

// rpc sends one approval RPC call and requires it to succeed.
func (s *GlobalsResolverOverrideTestSuite) rpc(resource, action string, params map[string]any) result.Result {
	s.T().Helper()

	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: resource,
		Action:   action,
		Version:  "v1",
		Params:   params,
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, action+" RPC should return HTTP 200")

	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should "+action)

	return res
}

// rpcID sends one approval RPC call and returns the id of what it created.
func (s *GlobalsResolverOverrideTestSuite) rpcID(resource, action string, params map[string]any) string {
	s.T().Helper()

	id, ok := s.ReadDataAsMap(s.rpc(resource, action, params).Data)["id"].(string)
	s.Require().True(ok, action+" should return an id")

	return id
}

func (s *GlobalsResolverOverrideTestSuite) TestStartSnapshotsResolvedGlobals() {
	const flowCode = "globals-override-flow"

	s.publishFlow(flowCode)

	instanceID := s.rpcID("approval/instance", "start", map[string]any{
		"tenantId": "default",
		"flowCode": flowCode,
	})

	var instance approval.Instance

	instance.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&instance).WherePK().Scan(s.ctx), "Should load the started instance")

	s.Equal(stubGlobals, instance.Globals,
		"Start must snapshot what the overridden resolver returned, proving vef.ProvideApprovalGlobalsResolver replaced the framework's no-op resolver")

	seenFlowCode, seenUserID := s.resolver.seen()
	s.Equal(flowCode, seenFlowCode, "The resolver must be told which flow is being started")
	s.NotEmpty(seenUserID, "The resolver must be handed the authenticated principal, never a nil caller")
}

// publishFlow creates, deploys, and publishes a standalone flow open to every
// initiator; callers address it by code.
func (s *GlobalsResolverOverrideTestSuite) publishFlow(flowCode string) {
	s.T().Helper()

	flowID := s.rpcID("approval/flow", "create", map[string]any{
		"tenantId":               "default",
		"code":                   flowCode,
		"name":                   flowCode,
		"categoryId":             s.categoryID,
		"bindingMode":            "standalone",
		"isAllInitiationAllowed": true,
		"instanceTitleTemplate":  flowCode + " {{.instanceNo}}",
	})

	versionID := s.rpcID("approval/flow", "deploy", map[string]any{
		"flowId":         flowID,
		"flowDefinition": toMap(simpleFlowDef()),
	})

	s.rpc("approval/flow", "publish_version", map[string]any{"versionId": versionID})
}

func (s *GlobalsResolverOverrideTestSuite) TestStartFormCarriesApplicantAndFormGlobals() {
	const flowCode = "globals-start-form-flow"

	s.publishFlow(flowCode)

	form := s.ReadDataAsMap(s.rpc("approval/my", "get_start_form", map[string]any{
		"tenantId": "default",
		"flowCode": flowCode,
	}).Data)

	applicant, ok := form["applicant"].(map[string]any)
	s.Require().True(ok, "The start form should carry the applicant snapshot")
	s.Equal("test-admin", applicant["id"], "The applicant should be the caller, resolved as Start resolves it")
	s.Equal(map[string]any{"region": "north"}, form["globals"],
		"The start form should carry only the globals the resolver exposes to forms")
}

func (s *GlobalsResolverOverrideTestSuite) TestInstanceDetailCarriesFormGlobals() {
	const flowCode = "globals-detail-flow"

	s.publishFlow(flowCode)

	instanceID := s.rpcID("approval/instance", "start", map[string]any{
		"tenantId": "default",
		"flowCode": flowCode,
	})

	detail := s.ReadDataAsMap(s.rpc("approval/my", "get_instance_detail", map[string]any{"instanceId": instanceID}).Data)

	instance, ok := detail["instance"].(map[string]any)
	s.Require().True(ok, "The detail should carry the instance")
	s.Equal(map[string]any{"region": "north"}, instance["globals"],
		"The detail should carry the form-visible part of the instance's snapshot, never the routing-only quotaLimit")
}
