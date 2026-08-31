package command_test

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/formeditor"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// BoomFormParser is a host FormSchemaParser stand-in whose failure carries no
// result.Error, exercising the deploy handler's bare-error wrap branch.
type BoomFormParser struct{}

func (*BoomFormParser) ParseFormFields(context.Context, json.RawMessage) ([]approval.FormFieldDefinition, error) {
	return nil, errors.New("boom")
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &DeployFlowTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// DeployFlowTestSuite tests the DeployFlowHandler.
type DeployFlowTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.DeployFlowHandler
	flowID  string
}

func (s *DeployFlowTestSuite) SetupSuite() {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "deploy-test",
		Name:     "Deploy Test Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "deploy-test-flow",
		Name:                   "Deploy Test Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
		CurrentVersion:         0,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")

	s.flowID = flow.ID
	s.handler = command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService(), formeditor.NewParser())
}

func (s *DeployFlowTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.FlowEdge)(nil),
		(*approval.FlowNodeCC)(nil),
		(*approval.FlowNodeAssignee)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
	)
}

func (s *DeployFlowTestSuite) TearDownSuite() {
	_, err := s.db.NewDelete().
		Model((*approval.Flow)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean flows")

	_, err = s.db.NewDelete().
		Model((*approval.FlowCategory)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.IsNotNull("id") }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean flow categories")
}

func (s *DeployFlowTestSuite) TestDeploySuccess() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		Caller:         approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow without error")
	s.Require().NotNil(result, "Should return created version")

	s.Assert().NotEmpty(result.ID, "Should generate version ID")
	s.Assert().Equal(s.flowID, result.FlowID, "Should set FlowID")
	s.Assert().Equal(1, result.Version, "Should set Version to CurrentVersion+1")
	s.Assert().Equal(approval.VersionDraft, result.Status, "Should set Status to draft")
	s.Assert().NotNil(result.FlowSchema, "Should set FlowSchema")

	// Verify version in DB
	var version approval.FlowVersion

	version.ID = result.ID
	err = s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find version in DB")
	s.Assert().Equal(s.flowID, version.FlowID, "DB record should have correct FlowID")

	// Verify nodes created
	var nodes []approval.FlowNode

	err = s.db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query nodes")
	s.Assert().Len(nodes, 2, "Should insert two nodes (start + end)")

	// Verify edge created
	var edges []approval.FlowEdge

	err = s.db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query edges")
	s.Assert().Len(edges, 1, "Should insert one edge")
}

func (s *DeployFlowTestSuite) TestDeploySnapshotsBusinessBinding() {
	instanceIDColumn := "apv_instance_id"
	bindingConfig := &approval.BusinessBindingConfig{
		TableName:        "biz_deploy_order",
		KeyColumns:       []string{" order_no ", "tenant_id"},
		StatusColumn:     "approval_status",
		InstanceIDColumn: &instanceIDColumn,
		StatusMapping: map[approval.InstanceStatus]string{
			approval.InstanceRunning: "  in_review  ",
		},
	}
	flow := &approval.Flow{BindingMode: approval.BindingBusiness, BusinessBinding: bindingConfig}
	flow.ID = s.flowID
	_, err := s.db.NewUpdate().Model(flow).Select("binding_mode", "business_binding").WherePK().Exec(s.ctx)

	s.Require().NoError(err, "Should configure the mutable flow binding before deploy")
	defer func() {
		_, _ = s.db.NewUpdate().Model((*approval.Flow)(nil)).
			Set("binding_mode", approval.BindingStandalone).
			Set("business_binding", nil).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.flowID) }).Exec(s.ctx)
	}()

	version, err := s.handler.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		Caller:         approval.SystemCaller,
	})
	s.Require().NoError(err, "Deploy should snapshot a complete business binding")
	s.Require().NotNil(version.BusinessBinding, "Deployed version should carry its binding snapshot")
	s.Assert().Equal([]string{"order_no", "tenant_id"}, version.BusinessBinding.KeyColumns,
		"Deployed binding snapshot should use normalized key order")
	s.Assert().Equal("in_review", version.BusinessBinding.StatusMapping[approval.InstanceRunning],
		"Deployed binding snapshot should use normalized status mapping")

	replacement := &approval.BusinessBindingConfig{
		TableName:        "biz_replacement_order",
		KeyColumns:       []string{"id"},
		StatusColumn:     "state",
		InstanceIDColumn: &instanceIDColumn,
	}
	_, err = s.db.NewUpdate().Model((*approval.Flow)(nil)).
		Set("business_binding", replacement).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.flowID) }).Exec(s.ctx)
	s.Require().NoError(err, "Should edit the mutable flow after deploy")

	reloaded := new(approval.FlowVersion)
	reloaded.ID = version.ID
	s.Require().NoError(s.db.NewSelect().Model(reloaded).WherePK().Scan(s.ctx),
		"Should reload the deployed version snapshot")
	s.Require().NotNil(reloaded.BusinessBinding, "Persisted version should retain its binding snapshot")
	s.Assert().Equal("biz_deploy_order", reloaded.BusinessBinding.TableName,
		"Mutable flow edits should not rewrite an existing version snapshot")
}

func (s *DeployFlowTestSuite) TestDeployWithDescription() {
	desc := "版本描述"
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		Description:    &desc,
		FlowDefinition: simpleFlowDef(),
		Caller:         approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow without error")
	s.Require().NotNil(result.Description, "Should have Description")
	s.Assert().Equal("版本描述", *result.Description, "Should set Description")

	// Verify in DB
	var version approval.FlowVersion

	version.ID = result.ID
	err = s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find version in DB")
	s.Require().NotNil(version.Description, "DB should have Description")
	s.Assert().Equal("版本描述", *version.Description, "DB should persist Description")
}

func (s *DeployFlowTestSuite) TestDeployFlowNotFound() {
	cmd := command.DeployFlowCmd{
		FlowID:         "non-existent-flow-id",
		FlowDefinition: simpleFlowDef(),
		Caller:         approval.SystemCaller,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail for non-existent flow")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return ErrFlowNotFound")
}

func (s *DeployFlowTestSuite) TestDeployInvalidFlowDesign() {
	cmd := command.DeployFlowCmd{
		FlowID: s.flowID,
		FlowDefinition: approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{
					ID:   "orphan-1",
					Kind: approval.NodeApproval,
					Data: mustMarshal(approval.ApprovalNodeData{
						Name: "审批",
					}),
				},
			},
		},
		Caller: approval.SystemCaller,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail for invalid flow design")
	s.Assert().ErrorIs(err, shared.ErrInvalidFlowDesign, "Should return ErrInvalidFlowDesign")
}

func (s *DeployFlowTestSuite) TestDeployInvalidAddAssigneeTypeInNodeData() {
	cmd := command.DeployFlowCmd{
		FlowID: s.flowID,
		FlowDefinition: approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
				{ID: "approval-1", Kind: approval.NodeApproval, Data: []byte(`{"name":"审批","isAddAssigneeAllowed":true,"addAssigneeTypes":["before","invalid"]}`)},
				{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "edge-1", Source: "start-1", Target: "approval-1"},
				{ID: "edge-2", Source: "approval-1", Target: "end-1"},
			},
		},
		Caller: approval.SystemCaller,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail for invalid add assignee type in node data")
	s.Assert().ErrorContains(err, "invalid AddAssigneeType", "Should surface invalid add assignee type")
}

// TestDeployRejectsDanglingFieldPermission proves ValidateFieldPermissions is
// wired into the deploy pipeline: a node whose fieldPermissions reference a form
// field key the derived schema does not define fails the deploy with
// ErrInvalidFlowDesign, before any version row is written. The field permission
// / form-field pairing can only be checked where the flow definition and the
// derived form fields meet — this pins that they actually do.
func (s *DeployFlowTestSuite) TestDeployRejectsDanglingFieldPermission() {
	cmd := command.DeployFlowCmd{
		FlowID: s.flowID,
		FlowDefinition: approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
				{
					ID:   "approval-1",
					Kind: approval.NodeApproval,
					Data: mustMarshal(approval.ApprovalNodeData{
						Name:                "审批",
						Assignees:           []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{"user-1"}, SortOrder: 1}},
						ExecutionType:       approval.ExecutionManual,
						EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
						// "ghost" is not a key of the derived form (only "reason" is),
						// so this permission is a dangling reference.
						FieldPermissions: map[string]approval.Permission{"ghost": approval.PermissionEditable},
						ApprovalMethod:   approval.ApprovalSequential,
						PassRule:         approval.PassAll,
					}),
				},
				{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "edge-1", Source: "start-1", Target: "approval-1"},
				{ID: "edge-2", Source: "approval-1", Target: "end-1"},
			},
		},
		FormSchema: formEditorSchemaJSON(s.T(), formEditorWidget{Type: "textfield", Key: "reason", Label: "Reason"}),
		Caller:     approval.SystemCaller,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should fail when a field permission references an undefined form field")
	s.Assert().ErrorIs(err, shared.ErrInvalidFlowDesign, "A dangling field permission must surface as invalid flow design")
	s.Assert().ErrorContains(err, "ghost", "The error must name the dangling field key")

	count, err := s.db.NewSelect().
		Model((*approval.FlowVersion)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_id", s.flowID) }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count versions")
	s.Assert().Zero(count, "A field-permission validation failure must abort the deploy before any version is created")
}

func (s *DeployFlowTestSuite) TestDeployWithAssigneesAndCCs() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: approvalFlowDef(),
		Caller:         approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow with assignees and CCs")

	// Find the approval node
	var nodes []approval.FlowNode

	err = s.db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID).
				Equals("kind", approval.NodeApproval)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query approval nodes")
	s.Require().Len(nodes, 1, "Should have one approval node")

	approvalNodeID := nodes[0].ID

	// Verify assignees
	var assignees []approval.FlowNodeAssignee

	err = s.db.NewSelect().
		Model(&assignees).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", approvalNodeID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query assignees")
	s.Require().Len(assignees, 1, "Should insert one assignee")
	s.Assert().Equal(approval.AssigneeUser, assignees[0].Kind, "Should set assignee kind")
	s.Assert().Equal([]string{"user-1", "user-2"}, assignees[0].IDs, "Should set assignee IDs")
	s.Assert().Equal(1, assignees[0].SortOrder, "Should set assignee sort order")

	// Verify CCs
	var ccs []approval.FlowNodeCC

	err = s.db.NewSelect().
		Model(&ccs).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", approvalNodeID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query CCs")
	s.Require().Len(ccs, 1, "Should insert one CC")
	s.Assert().Equal(approval.CCUser, ccs[0].Kind, "Should set CC kind")
	s.Assert().Equal([]string{"cc-user-1"}, ccs[0].IDs, "Should set CC IDs")
	s.Assert().Equal(approval.CCTimingAlways, ccs[0].Timing, "Should set CC timing")
}

// TestDeployStoresLongKindNames pins that a kind identifier round-trips
// through the schema. The kind vocabularies are open registries — a host
// registers "the applicant's head nurse" and the flow that names it must
// persist — and even the framework's own `department_leader` is 17 characters,
// so a narrow `kind` column silently made a built-in kind undeployable on the
// dialects that enforce a width.
func (s *DeployFlowTestSuite) TestDeployStoresLongKindNames() {
	hostKind := approval.AssigneeKind("attending_physician_on_duty")

	definition := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
			{
				ID:   "approval-1",
				Kind: approval.NodeApproval,
				Data: mustMarshal(approval.ApprovalNodeData{
					Name: "审批",
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeDepartmentLeader, SortOrder: 1},
					},
					CCs: []approval.CCDefinition{
						{Kind: approval.CCDepartment, IDs: []string{"dept-1"}, Timing: approval.CCTimingAlways},
					},
				}),
			},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}

	result, err := s.handler.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: definition,
		Caller:         approval.SystemCaller,
	})
	s.Require().NoError(err, "A built-in kind whose name exceeds a narrow column must still deploy")

	var nodes []approval.FlowNode

	err = s.db.NewSelect().
		Model(&nodes).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID).
				Equals("kind", approval.NodeApproval)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query approval nodes")
	s.Require().Len(nodes, 1, "Should have one approval node")

	var assignees []approval.FlowNodeAssignee

	err = s.db.NewSelect().
		Model(&assignees).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", nodes[0].ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query assignees")
	s.Require().Len(assignees, 1, "Should insert one assignee")
	s.Assert().Equal(approval.AssigneeDepartmentLeader, assignees[0].Kind, "The stored kind must not be truncated")

	// A host kind is longer still: the column must hold whatever identifier a
	// registered resolver describes itself with.
	_, err = s.db.NewInsert().
		Model(&approval.FlowNodeAssignee{NodeID: assignees[0].NodeID, Kind: hostKind, IDs: []string{}, SortOrder: 2}).
		Exec(s.ctx)
	s.Require().NoError(err, "A host kind identifier must fit the column")
}

func (s *DeployFlowTestSuite) TestDeployEdgesWithNodeKeys() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		Caller:         approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow")

	var edges []approval.FlowEdge

	err = s.db.NewSelect().
		Model(&edges).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", result.ID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query edges")
	s.Require().Len(edges, 1, "Should have one edge")

	s.Assert().Equal("start-1", edges[0].SourceNodeKey, "Should set SourceNodeKey")
	s.Assert().Equal("end-1", edges[0].TargetNodeKey, "Should set TargetNodeKey")
	s.Assert().NotEmpty(edges[0].SourceNodeID, "Should set SourceNodeID")
	s.Assert().NotEmpty(edges[0].TargetNodeID, "Should set TargetNodeID")
}

func (s *DeployFlowTestSuite) TestDeployPersistsFormSchemaVerbatimAndDerivesFields() {
	schema := formEditorSchemaJSON(s.T(),
		formEditorWidget{Type: "textfield", Key: "reason", Label: "Reason"},
		formEditorWidget{Type: "number", Key: "amount", Label: "Amount"},
	)

	result, err := s.handler.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		FormSchema:     schema,
		Caller:         approval.SystemCaller,
	})
	s.Require().NoError(err, "Should deploy flow with a form schema")

	var version approval.FlowVersion

	version.ID = result.ID
	err = s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find version in DB")

	// The designer document is opaque to the framework and must round-trip
	// verbatim (JSON-semantically — the jsonb column normalizes whitespace).
	s.Assert().JSONEq(string(schema), string(version.FormSchema), "form_schema should persist the designer document verbatim")

	// form_fields carries the parser-derived flat list the framework consumes.
	s.Require().Len(version.FormFields, 2, "form_fields should hold both derived fields")
	s.Assert().Equal("reason", version.FormFields[0].Key, "First derived field should keep its key")
	s.Assert().Equal(approval.FieldInput, version.FormFields[0].Kind, "textfield should project to the input kind")
	s.Assert().Equal("Reason", version.FormFields[0].Label, "Derived field should keep its label")
	s.Assert().Equal("amount", version.FormFields[1].Key, "Second derived field should keep its key")
	s.Assert().Equal(approval.FieldNumber, version.FormFields[1].Kind, "number should project to the number kind")
}

func (s *DeployFlowTestSuite) TestDeployParserErrorAbortsDeploy() {
	// switch binds a boolean the approval value contract cannot carry, so the
	// built-in parser rejects it with an outward invalid-form-design error.
	schema := formEditorSchemaJSON(s.T(), formEditorWidget{Type: "switch", Key: "toggle"})

	_, err := s.handler.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		FormSchema:     schema,
		Caller:         approval.SystemCaller,
	})
	s.Require().Error(err, "Should fail when the form schema cannot be parsed")
	s.Assert().ErrorIs(err, shared.ErrInvalidFormDesign, "Parser errors should surface as invalid form design")

	// The built-in parser's specific message survives the wrap: a bare context
	// wrap keeps it first, so the caller sees which field / widget was rejected.
	s.Assert().ErrorContains(err, "toggle", "The built-in parser's message must name the offending field key")
	s.Assert().ErrorContains(err, "switch", "The built-in parser's message must name the offending widget type")

	count, err := s.db.NewSelect().
		Model((*approval.FlowVersion)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_id", s.flowID) }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count versions")
	s.Assert().Zero(count, "A parser failure must abort the deploy before any version is created")
}

// TestDeployHostParserBareErrorWrapsAsInvalidFormDesign pins the host-parser
// opacity fix: a parser whose error carries no result.Error is wrapped in the
// form-design sentinel so it surfaces as an invalid-form-design outcome rather
// than a raw 500.
func (s *DeployFlowTestSuite) TestDeployHostParserBareErrorWrapsAsInvalidFormDesign() {
	handler := command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService(), new(BoomFormParser))

	_, err := handler.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		FormSchema:     json.RawMessage(`{"version":2}`),
		Caller:         approval.SystemCaller,
	})
	s.Require().Error(err, "A host parser failure must abort the deploy")
	s.Assert().ErrorIs(err, shared.ErrInvalidFormDesign, "A bare parser error must be wrapped in the form-design sentinel")

	count, err := s.db.NewSelect().
		Model((*approval.FlowVersion)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_id", s.flowID) }).
		Count(s.ctx)
	s.Require().NoError(err, "Should count versions")
	s.Assert().Zero(count, "A parser failure must abort the deploy before any version is created")
}

func (s *DeployFlowTestSuite) TestDeployDoesNotUpdateFlowCurrentVersion() {
	cmd := command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
		Caller:         approval.SystemCaller,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deploy flow")

	// Verify flow's CurrentVersion is unchanged (still 0)
	var flow approval.Flow

	flow.ID = s.flowID
	err = s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find flow")
	s.Assert().Equal(0, flow.CurrentVersion, "Deploy should not update flow CurrentVersion")
}
