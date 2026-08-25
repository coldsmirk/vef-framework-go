package resource_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// FlowResourceTestSuite tests the flow resource operations via HTTP.
type FlowResourceTestSuite struct {
	apptest.Suite

	ctx        context.Context
	db         orm.DB
	token      string
	categoryID string
}

func TestFlowResource(t *testing.T) {
	suite.Run(t, new(FlowResourceTestSuite))
}

func (s *FlowResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, s.token = setupResourceApp(&s.Suite)

	// Insert a shared category for flow tests
	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "flow-test-cat",
		Name:     "Flow Test Category",
		IsActive: true,
	}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = cat.ID

	_, err = s.db.NewRaw(`CREATE TABLE resource_binding_order (
		tenant_id VARCHAR(32) NOT NULL,
		order_no VARCHAR(64) NOT NULL,
		approval_status VARCHAR(32),
		approval_instance_id VARCHAR(32),
		UNIQUE (tenant_id, order_no)
	)`).Exec(s.ctx)
	s.Require().NoError(err, "Should create the business table used by binding API tests")
}

func (s *FlowResourceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
	_, _ = s.db.NewRaw("DROP TABLE IF EXISTS resource_binding_order").Exec(s.ctx)
	s.TearDownApp()
}

func (s *FlowResourceTestSuite) TearDownTest() {
	// Clean flow data but keep the shared category
	deleteAll(s.ctx, s.db,
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeCC)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
		(*approval.FlowInitiator)(nil),
		(*approval.Flow)(nil),
	)
}

// createFlow creates a flow via RPC and returns its ID.
func (s *FlowResourceTestSuite) createFlow(code, name string) string {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "create",
		Version:  "v1",
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   code,
			"name":                   name,
			"categoryId":             s.categoryID,
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  fmt.Sprintf("%s {{.instanceNo}}", name),
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Flow create RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should create flow")

	data := s.ReadDataAsMap(res.Data)
	flowID, ok := data["id"].(string)
	s.Require().True(ok, "Flow ID should be a string")

	return flowID
}

// deployFlow deploys a flow definition via RPC and returns the version ID.
func (s *FlowResourceTestSuite) deployFlow(flowID string, def approval.FlowDefinition) string {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "deploy",
		Version:  "v1",
		Params: map[string]any{
			"flowId":         flowID,
			"flowDefinition": toMap(def),
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Flow deploy RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should deploy flow")

	data := s.ReadDataAsMap(res.Data)
	versionID, ok := data["id"].(string)
	s.Require().True(ok, "Version ID should be a string")

	return versionID
}

func (s *FlowResourceTestSuite) TestCreateFlow() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "create",
		Version:  "v1",
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   "test-flow-create",
			"name":                   "Test Flow",
			"categoryId":             s.categoryID,
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  "Test {{.instanceNo}}",
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Create flow RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should create flow successfully")

	data := s.ReadDataAsMap(res.Data)
	s.Assert().Equal("test-flow-create", data["code"], "Created flow should echo the submitted code")
	s.Assert().Equal("Test Flow", data["name"], "Created flow should echo the submitted name")
	s.Assert().NotEmpty(data["id"], "Should generate an ID")
}

func (s *FlowResourceTestSuite) TestCreateBusinessBoundFlow() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "create",
		Version:  "v1",
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   "test-flow-business-binding",
			"name":                   "Business Binding Flow",
			"categoryId":             s.categoryID,
			"bindingMode":            "business",
			"isAllInitiationAllowed": true,
			"businessBinding": map[string]any{
				"tableName":        "resource_binding_order",
				"keyColumns":       []string{"tenant_id", "order_no"},
				"statusColumn":     "approval_status",
				"instanceIdColumn": "approval_instance_id",
			},
			"instanceTitleTemplate": "Order {{.instanceNo}}",
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Business-bound flow create RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "A composite unique key should pass live schema validation")

	data := s.ReadDataAsMap(res.Data)
	binding := s.ReadDataAsMap(data["businessBinding"])
	s.Assert().Equal("resource_binding_order", binding["tableName"], "Response should expose the nested binding config")
	s.Assert().ElementsMatch([]any{"order_no", "tenant_id"}, binding["keyColumns"],
		"Response should expose the normalized composite key")
}

func (s *FlowResourceTestSuite) TestDeployFlow() {
	flowID := s.createFlow("test-flow-deploy", "Deploy Test")
	versionID := s.deployFlow(flowID, simpleFlowDef())
	s.Assert().NotEmpty(versionID, "Should return version ID")

	// Verify version in DB
	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "TestDeployFlow should complete without error")
	s.Assert().Equal(1, version.Version, "Should be version 1")
	s.Assert().Equal(approval.VersionDraft, version.Status, "Should be draft status")
}

func (s *FlowResourceTestSuite) TestPublishVersion() {
	flowID := s.createFlow("test-flow-publish", "Publish Test")
	versionID := s.deployFlow(flowID, approvalFlowDef())

	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "publish_version",
		Version:  "v1",
		Params: map[string]any{
			"versionId": versionID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Publish version RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should publish version successfully")

	// Verify version status
	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "TestPublishVersion should complete without error")
	s.Assert().Equal(approval.VersionPublished, version.Status, "Should be published")

	// Verify flow.currentVersion updated
	var flow approval.Flow

	flow.ID = flowID
	s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx), "TestPublishVersion should complete without error")
	s.Assert().Equal(1, flow.CurrentVersion, "Current version should be 1 after publish")
}

func (s *FlowResourceTestSuite) TestGetGraph() {
	flowID := s.createFlow("test-flow-graph", "Graph Test")
	versionID := s.deployFlow(flowID, approvalFlowDef())

	// Publish first
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "publish_version",
		Version:  "v1",
		Params: map[string]any{
			"versionId": versionID,
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Publish version RPC (prerequisite for graph) should return HTTP 200")

	// Get graph
	resp = s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "get_graph",
		Version:  "v1",
		Params: map[string]any{
			"flowId": flowID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Get graph RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should get flow graph")

	data := s.ReadDataAsMap(res.Data)
	s.Assert().NotNil(data["nodes"], "Graph should contain nodes")
	s.Assert().NotNil(data["edges"], "Graph should contain edges")
}

// TestDeployPreservesLargeIntegerInFormSchema is the end-to-end proof of the
// number-preserving request binding (api.Params.UnmarshalJSON with UseNumber +
// mapx hooks): a deploy whose formSchema carries an integer beyond 2^53 must
// round-trip through the full HTTP → api.Params → mapx → DeployFlowParams
// (FormSchema json.RawMessage) → jsonb pipeline with its exact digits intact. A
// float64 collapse would round 9007199254740993 to ...992, so the persisted
// form_schema is checked for the exact digits.
func (s *FlowResourceTestSuite) TestDeployPreservesLargeIntegerInFormSchema() {
	flowID := s.createFlow("test-flow-bigint", "BigInt Deploy")

	// A valid form-editor document whose select option carries an integer above
	// 2^53. The document is stored verbatim, so the digits must survive exactly.
	const bigInt = "9007199254740993"

	const rounded = "9007199254740992" // what a float64 round-trip would produce

	formSchema := json.RawMessage(`{"version":2,"presentations":{"pc":{"children":[` +
		`{"id":"F1","type":"select","key":"level","label":"Level",` +
		`"dataSource":{"kind":"static","options":[{"label":"Big","value":` + bigInt + `}]}}]}}}`)

	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "deploy",
		Version:  "v1",
		Params: map[string]any{
			"flowId":         flowID,
			"flowDefinition": toMap(simpleFlowDef()),
			"formSchema":     formSchema,
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Deploy RPC should return HTTP 200")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should deploy flow with a big-integer form schema")

	data := s.ReadDataAsMap(res.Data)
	versionID, ok := data["id"].(string)
	s.Require().True(ok, "Version ID should be a string")

	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "Should load the deployed version")

	stored := string(version.FormSchema)
	s.Assert().Contains(stored, bigInt, "form_schema must persist the integer beyond 2^53 with its exact digits")
	s.Assert().NotContains(stored, rounded, "form_schema must not have collapsed the integer through float64")
}

func (s *FlowResourceTestSuite) TestDeployInvalidDefinition() {
	flowID := s.createFlow("test-flow-invalid", "Invalid Deploy Test")

	// Deploy with empty definition (no nodes)
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/flow",
		Action:   "deploy",
		Version:  "v1",
		Params: map[string]any{
			"flowId": flowID,
			"flowDefinition": map[string]any{
				"nodes": []any{},
				"edges": []any{},
			},
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Deploy RPC should return HTTP 200 even for invalid definitions (error in result body)")
	res := s.ReadResult(resp)
	s.Assert().False(res.IsOk(), "Should fail to deploy with invalid definition")
}
