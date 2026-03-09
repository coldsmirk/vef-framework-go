package resource_test

import (
	"context"
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
}

func (s *FlowResourceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
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
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "create",
			Version:  "v1",
		},
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

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
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
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowId":         flowID,
			"flowDefinition": toMap(def),
		},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should deploy flow")

	data := s.ReadDataAsMap(res.Data)
	versionID, ok := data["id"].(string)
	s.Require().True(ok, "Version ID should be a string")

	return versionID
}

func (s *FlowResourceTestSuite) TestCreateFlow() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "create",
			Version:  "v1",
		},
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

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should create flow successfully")

	data := s.ReadDataAsMap(res.Data)
	s.Assert().Equal("test-flow-create", data["code"], "Should match expected value")
	s.Assert().Equal("Test Flow", data["name"], "Should match expected value")
	s.Assert().NotEmpty(data["id"], "Should generate an ID")
}

func (s *FlowResourceTestSuite) TestDeployFlow() {
	flowID := s.createFlow("test-flow-deploy", "Deploy Test")
	versionID := s.deployFlow(flowID, simpleFlowDef())
	s.Assert().NotEmpty(versionID, "Should return version ID")

	// Verify version in DB
	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(1, version.Version, "Should be version 1")
	s.Assert().Equal(approval.VersionDraft, version.Status, "Should be draft status")
}

func (s *FlowResourceTestSuite) TestPublishVersion() {
	flowID := s.createFlow("test-flow-publish", "Publish Test")
	versionID := s.deployFlow(flowID, approvalFlowDef())

	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "publish_version",
			Version:  "v1",
		},
		Params: map[string]any{
			"versionId": versionID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should publish version successfully")

	// Verify version status
	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.VersionPublished, version.Status, "Should be published")

	// Verify flow.currentVersion updated
	var flow approval.Flow

	flow.ID = flowID
	s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(1, flow.CurrentVersion, "Current version should be 1 after publish")
}

func (s *FlowResourceTestSuite) TestGetGraph() {
	flowID := s.createFlow("test-flow-graph", "Graph Test")
	versionID := s.deployFlow(flowID, approvalFlowDef())

	// Publish first
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "publish_version",
			Version:  "v1",
		},
		Params: map[string]any{
			"versionId": versionID,
		},
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")

	// Get graph
	resp = s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "get_graph",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowId": flowID,
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res := s.ReadResult(resp)
	s.Assert().True(res.IsOk(), "Should get flow graph")

	data := s.ReadDataAsMap(res.Data)
	s.Assert().NotNil(data["nodes"], "Graph should contain nodes")
	s.Assert().NotNil(data["edges"], "Graph should contain edges")
}

func (s *FlowResourceTestSuite) TestDeployInvalidDefinition() {
	flowID := s.createFlow("test-flow-invalid", "Invalid Deploy Test")

	// Deploy with empty definition (no nodes)
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "approval/flow",
			Action:   "deploy",
			Version:  "v1",
		},
		Params: map[string]any{
			"flowId": flowID,
			"flowDefinition": map[string]any{
				"nodes": []any{},
				"edges": []any{},
			},
		},
	}, s.token)

	s.Assert().Equal(http.StatusOK, resp.StatusCode, "Should match expected value")
	res := s.ReadResult(resp)
	s.Assert().False(res.IsOk(), "Should fail to deploy with invalid definition")
}
