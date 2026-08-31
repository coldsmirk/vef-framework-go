package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	vef "github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// headNurseKind is the shape a hospital-style host reaches for: a kind that
// takes no designer input and answers from the applicant's department at run
// time — the case a role list cannot express.
const headNurseKind approval.AssigneeKind = "head_nurse"

// headNurseUserID is who HeadNurseResolver resolves to. No built-in kind can
// produce it, so a task assigned to it can only mean the host resolver ran.
const headNurseUserID = "head-nurse-1"

// HeadNurseResolver is a host approval.AssigneeResolver registered through the
// public helper. It reports the applicant's department back so the test can
// prove the resolve context carried the running instance, not just that the
// registration was wired.
type HeadNurseResolver struct{}

func (*HeadNurseResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      headNurseKind,
		Label:     "护士长",
		Selection: approval.SelectionNone,
	}
}

func (*HeadNurseResolver) Resolve(_ context.Context, rc *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	applicant := rc.Applicant()

	name := "Head Nurse"
	if applicant.DepartmentID != nil {
		name += " of " + *applicant.DepartmentID
	}

	return []approval.ResolvedAssignee{{User: approval.UserInfo{ID: headNurseUserID, Name: name}}}, nil
}

// wardBoardCCKind is the CC-side counterpart: a host kind that names a
// standing group of observers rather than an approver.
const wardBoardCCKind approval.CCKind = "ward_board"

// wardBoardUserID is who WardBoardCCResolver copies. No built-in CC kind can
// produce it, so a CC record for it can only mean the host resolver ran.
const wardBoardUserID = "ward-board-1"

// WardBoardCCResolver is a host approval.CCResolver registered through the
// public helper. CC resolution runs through a composite of its own, so the
// assignee chain proving out says nothing about this one.
type WardBoardCCResolver struct{}

func (*WardBoardCCResolver) Describe() approval.KindDescriptor[approval.CCKind] {
	return approval.KindDescriptor[approval.CCKind]{
		Kind:      wardBoardCCKind,
		Label:     "病区管理组",
		Selection: approval.SelectionNone,
	}
}

func (*WardBoardCCResolver) Resolve(context.Context, *approval.CCResolveContext) ([]string, error) {
	return []string{wardBoardUserID}, nil
}

// PrincipalResolverOverrideTestSuite proves the whole chain a host assignee
// kind travels: vef.ProvideApprovalAssigneeResolver → the fx group → the
// composite resolver → deploy validation accepting the kind → the engine
// creating the task it resolves to. Each link is unit-tested on its own; this
// is the one test that fails if any of them stops being connected.
type PrincipalResolverOverrideTestSuite struct {
	apptest.Suite

	ctx        context.Context
	db         orm.DB
	token      string
	categoryID string
}

func TestPrincipalResolverOverride(t *testing.T) {
	suite.Run(t, new(PrincipalResolverOverrideTestSuite))
}

func (s *PrincipalResolverOverrideTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.db, s.token = setupResourceApp(&s.Suite,
		vef.ProvideApprovalAssigneeResolver(func() approval.AssigneeResolver { return new(HeadNurseResolver) }),
		vef.ProvideApprovalCCResolver(func() approval.CCResolver { return new(WardBoardCCResolver) }),
	)

	cat := &approval.FlowCategory{
		TenantID: "default",
		Code:     "host-kind-cat",
		Name:     "Host Kind Category",
		IsActive: true,
	}
	_, err := s.db.NewInsert().Model(cat).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = cat.ID
}

func (s *PrincipalResolverOverrideTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
	s.TearDownApp()
}

// rpc sends one approval RPC call and requires it to succeed.
func (s *PrincipalResolverOverrideTestSuite) rpc(resource, action string, params map[string]any) result.Result {
	s.T().Helper()

	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: resource,
		Action:   action,
		Version:  "v1",
		Params:   params,
	}, s.token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, action+" RPC should return HTTP 200")

	res := s.ReadResult(resp)
	s.Require().True(res.IsOk(), "Should "+action+": "+res.Message)

	return res
}

// rpcID sends one approval RPC call and returns the id of what it created.
func (s *PrincipalResolverOverrideTestSuite) rpcID(resource, action string, params map[string]any) string {
	s.T().Helper()

	id, ok := s.ReadDataAsMap(s.rpc(resource, action, params).Data)["id"].(string)
	s.Require().True(ok, action+" should return an id")

	return id
}

func (s *PrincipalResolverOverrideTestSuite) TestHostAssigneeKindIsOfferedAndExecuted() {
	const flowCode = "host-kind-flow"

	// The designer reads its options from here, so a kind absent from this
	// catalog is a kind nobody can configure.
	options := s.ReadDataAsMap(s.rpc("approval/flow", "list_kind_options", map[string]any{}).Data)

	assignees, ok := options["assignees"].([]any)
	s.Require().True(ok, "The catalog must carry the assignee vocabulary")

	var described map[string]any

	for _, entry := range assignees {
		if row, isMap := entry.(map[string]any); isMap && row["kind"] == string(headNurseKind) {
			described = row
		}
	}

	s.Require().NotNil(described, "A host-registered kind must appear in the designer catalog")
	s.Equal("护士长", described["label"], "The catalog must carry the label the resolver describes")
	s.Equal(string(approval.SelectionNone), described["selection"], "The catalog must carry the input the kind requires")

	// CC travels its own composite, so its catalog entry is a separate fact.
	ccs, ok := options["ccs"].([]any)
	s.Require().True(ok, "The catalog must carry the CC vocabulary")

	var describedCC map[string]any

	for _, entry := range ccs {
		if row, isMap := entry.(map[string]any); isMap && row["kind"] == string(wardBoardCCKind) {
			describedCC = row
		}
	}

	s.Require().NotNil(describedCC, "A host-registered CC kind must appear in the designer catalog")
	s.Equal("病区管理组", describedCC["label"], "The CC catalog must carry the label the resolver describes")

	flowID := s.rpcID("approval/flow", "create", map[string]any{
		"tenantId":               "default",
		"code":                   flowCode,
		"name":                   "Host Kind",
		"categoryId":             s.categoryID,
		"bindingMode":            "standalone",
		"isAllInitiationAllowed": true,
		"instanceTitleTemplate":  "Host Kind {{.instanceNo}}",
	})

	// Deploy accepts the kind only because a resolver is registered for it:
	// the vocabulary validation reads the same composite the engine resolves
	// through, and the kind carries no ids because its selection mode says so.
	versionID := s.rpcID("approval/flow", "deploy", map[string]any{
		"flowId": flowID,
		"flowDefinition": toMap(approval.FlowDefinition{
			Nodes: []approval.NodeDefinition{
				{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
				{
					// A CC node rather than a CC rule on the approval node:
					// node CC with a timing fires when the node *completes*,
					// and the approval node below is still waiting on its
					// assignee. A CC node delivers on entry, so one start
					// exercises both lanes.
					ID:   "cc-1",
					Kind: approval.NodeCC,
					Data: mustMarshal(approval.CCNodeData{
						Name: "抄送",
						CCs:  []approval.CCDefinition{{Kind: wardBoardCCKind, Timing: approval.CCTimingAlways}},
					}),
				},
				{
					ID:   "approval-1",
					Kind: approval.NodeApproval,
					Data: mustMarshal(approval.ApprovalNodeData{
						Name:      "审批",
						Assignees: []approval.AssigneeDefinition{{Kind: headNurseKind, SortOrder: 1}},
					}),
				},
				{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "edge-1", Source: "start-1", Target: "cc-1"},
				{ID: "edge-2", Source: "cc-1", Target: "approval-1"},
				{ID: "edge-3", Source: "approval-1", Target: "end-1"},
			},
		}),
	})

	s.rpc("approval/flow", "publish_version", map[string]any{"versionId": versionID})

	instanceID := s.rpcID("approval/instance", "start", map[string]any{
		"tenantId": "default",
		"flowCode": flowCode,
	})

	var tasks []approval.Task

	s.Require().NoError(s.db.NewSelect().
		Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID)
		}).
		Scan(s.ctx), "Should load the instance's tasks")

	s.Require().Len(tasks, 1, "The host kind must resolve to exactly one assignee")
	s.Equal(headNurseUserID, tasks[0].AssigneeID,
		"The task must be assigned to whom the host resolver returned, proving the engine resolved through it")
	s.Contains(tasks[0].AssigneeName, "Head Nurse",
		"The snapshot must carry what the host resolver reported")

	var ccRecords []approval.CCRecord

	s.Require().NoError(s.db.NewSelect().
		Model(&ccRecords).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID)
		}).
		Scan(s.ctx), "Should load the instance's CC records")

	s.Require().Len(ccRecords, 1, "The host CC kind must resolve to exactly one recipient")
	s.Equal(wardBoardUserID, ccRecords[0].CCUserID,
		"The CC record must name whom the host CC resolver returned, proving the engine resolved through it")
}
