package command_test

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/formeditor"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		// env.DS is dereferenced lazily in SetupSuite — the registry calls this
		// factory once with a zero DBEnv (nil DS) purely to read the type name.
		return &StorageTableTestSuite{env: env, ctx: env.Ctx, db: env.DB}
	})
}

// StorageTableTestSuite exercises the StorageTable form-storage path end to end:
// deploy with storageMode=table, publish (generates the physical table and
// metadata), and start an instance (projects a row into the physical table).
type StorageTableTestSuite struct {
	suite.Suite

	env        *testx.DBEnv
	ctx        context.Context
	db         orm.DB
	kind       config.DBKind
	dispatcher *storage.Dispatcher
}

func (s *StorageTableTestSuite) SetupSuite() {
	s.kind = s.env.DS.Kind
	s.dispatcher = storage.NewDispatcher(storage.NewJSONStorage(), storage.NewTableStorage(s.kind))
}

func (s *StorageTableTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *StorageTableTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// formSchema is the default form-editor document deployed for the table-mode
// flow; the deploy pipeline's parser derives textarea/number/select fields
// from it.
func (s *StorageTableTestSuite) formSchema() json.RawMessage {
	return formEditorSchemaJSON(s.T(),
		formEditorWidget{Type: "textarea", Key: "reason", Label: "Reason"},
		formEditorWidget{Type: "number", Key: "amount", Label: "Amount"},
		formEditorWidget{Type: "select", Key: "tags", Label: "Tags"},
	)
}

// deployPublishedTableFlow deploys + publishes a table-mode flow in the default
// tenant with the suite's default form schema.
func (s *StorageTableTestSuite) deployPublishedTableFlow(code string) (flowID, versionID string) {
	return s.deployTableFlow("default", code, s.formSchema())
}

// deployTableFlow creates a category + flow in the given tenant, deploys a
// table-mode version with the supplied form-editor document, and publishes it
// through a handler wired with the real storage dispatcher. It returns the
// flow ID and version ID.
func (s *StorageTableTestSuite) deployTableFlow(tenant, code string, schema json.RawMessage) (flowID, versionID string) {
	category := &approval.FlowCategory{TenantID: tenant, Code: code + "-cat", Name: code}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "insert category")

	flow := &approval.Flow{
		TenantID:               tenant,
		CategoryID:             category.ID,
		Code:                   code,
		Name:                   code + " Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  code + " {{.instanceNo}}",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "insert flow")

	deploy := command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService(), formeditor.NewParser())
	version, err := deploy.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         flow.ID,
		StorageMode:    approval.StorageTable,
		FlowDefinition: simpleFlowDef(),
		FormSchema:     schema,
		Caller:         approval.SystemCaller,
	})
	s.Require().NoError(err, "deploy flow")
	s.Require().Equal(approval.StorageTable, version.StorageMode, "version persists table storage mode")

	publish := wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewPublishVersionHandler(s.db, nil, s.dispatcher))
	_, err = publish.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin",
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "publish version")

	return flow.ID, version.ID
}

// startTableInstance starts an instance for a published table-mode flow through
// a handler wired with the real storage dispatcher.
func (s *StorageTableTestSuite) startTableInstance(code string, formData map[string]any) *approval.Instance {
	eng := buildTestEngine(s.db)
	start := wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewStartInstanceHandler(
			s.db, eng, &MockInstanceNoGenerator{}, service.NewValidationService(nil),
			binding.NewNoopRefProvider(), binding.NewWriter(binding.NewIdentityResolver()), s.dispatcher,
		))

	instance, err := start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  code,
		Applicant: approval.UserInfo{ID: "user-1", Name: "User One"},
		FormData:  formData,
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "start instance")
	s.Require().NotNil(instance)

	return instance
}

// physicalTableName resolves the generated physical table name for a version.
func (s *StorageTableTestSuite) physicalTableName(versionID string) string {
	var formTable approval.FormTable
	s.Require().NoError(s.db.NewSelect().Model(&formTable).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("version_id", versionID) }).
		Scan(s.ctx))

	return formTable.PhysicalTableName
}

func (s *StorageTableTestSuite) TestPublishGeneratesTableAndMetadata() {
	_, versionID := s.deployPublishedTableFlow("storage-tbl-publish")

	var formTable approval.FormTable
	s.Require().NoError(
		s.db.NewSelect().Model(&formTable).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("version_id", versionID) }).
			Scan(s.ctx),
		"form table metadata should exist for the version",
	)
	// The name keeps a readable prefix but is uniquely suffixed with the version
	// id, so it can never collide across tenants/flows — assert the prefix and
	// identifier-safety rather than a fixed value.
	s.Assert().True(strings.HasPrefix(formTable.PhysicalTableName, "apv_form_storage_tbl_publish_"),
		"physical table name should keep a readable prefix; got %q", formTable.PhysicalTableName)
	s.Assert().NoError(approval.ValidateBusinessIdentifier(formTable.PhysicalTableName))

	var columns []approval.FormTableColumn
	s.Require().NoError(
		s.db.NewSelect().Model(&columns).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("form_table_id", formTable.ID) }).
			OrderBy("sort_order").
			Scan(s.ctx),
		"column metadata should exist",
	)

	names := make([]string, len(columns))
	for i, c := range columns {
		names[i] = c.ColumnName
	}

	s.Assert().Equal([]string{"id", "instance_id", "reason", "amount", "tags", "created_at"}, names)

	// The physical table is real: a COUNT against it must succeed (0 rows yet).
	var count int
	s.Require().NoError(
		s.db.NewRaw("SELECT COUNT(*) FROM "+formTable.PhysicalTableName).Scan(s.ctx, &count),
		"physical table should be queryable",
	)
	s.Assert().Equal(0, count)
}

func (s *StorageTableTestSuite) TestPublishIsIdempotent() {
	_, versionID := s.deployPublishedTableFlow("storage-tbl-idem")

	// Re-running OnVersionPublished (e.g. a retry) must not error or duplicate.
	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx))

	var flow approval.Flow

	flow.ID = version.FlowID
	s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx))

	s.Require().NoError(s.dispatcher.ProvisionTable(s.ctx, s.db, &flow, &version), "second provision is a no-op")
	s.Require().NoError(s.dispatcher.RecordMetadata(s.ctx, s.db, &flow, &version), "second record is a no-op")

	count, err := s.db.NewSelect().Model((*approval.FormTable)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("version_id", versionID) }).
		Count(s.ctx)
	s.Require().NoError(err)
	s.Assert().EqualValues(1, count, "exactly one form table metadata row")
}

func (s *StorageTableTestSuite) TestStartInstanceProjectsRow() {
	_, versionID := s.deployPublishedTableFlow("storage-tbl-start")

	instance := s.startTableInstance("storage-tbl-start", map[string]any{
		"reason": "need it",
		"amount": 12.5,
		"tags":   []any{"x", "y"},
	})

	tableName := s.physicalTableName(versionID)

	// Read the projected row back out of the physical table.
	type projected struct {
		InstanceID string  `bun:"instance_id"`
		Reason     *string `bun:"reason"`
		Amount     *string `bun:"amount"`
		Tags       *string `bun:"tags"`
	}

	var row projected
	s.Require().NoError(
		s.db.NewRaw(
			"SELECT instance_id, reason, amount, tags FROM "+tableName+" WHERE instance_id = ?",
			instance.ID,
		).Scan(s.ctx, &row),
		"projected row should be present",
	)

	s.Assert().Equal(instance.ID, row.InstanceID)
	s.Require().NotNil(row.Reason)
	s.Assert().Equal("need it", *row.Reason)
	s.Require().NotNil(row.Tags)
	s.Assert().JSONEq(`["x","y"]`, *row.Tags, "multi-value field is stored as JSON text")
	s.Require().NotNil(row.Amount)

	// The instance's JSONB form_data is still populated (existing read paths).
	s.Assert().Equal("need it", instance.FormData["reason"])
}

// TestResubmitReplacesProjectionRow pins the one-row-per-instance contract: a
// second projection write for the same instance (the resubmit path) must refresh
// the existing row, never append a duplicate.
func (s *StorageTableTestSuite) TestResubmitReplacesProjectionRow() {
	_, versionID := s.deployPublishedTableFlow("storage-tbl-replace")

	instance := s.startTableInstance("storage-tbl-replace", map[string]any{"reason": "first", "amount": 1})

	// Mirror what ResubmitInstanceHandler does: refresh the instance's form data and
	// re-project it through SyncInstanceProjection — the same single entry point
	// every handler funnels form_data mutations through.
	instance.FormData = map[string]any{"reason": "second", "amount": 2}
	s.Require().NoError(
		s.dispatcher.SyncInstanceProjection(s.ctx, s.db, instance),
		"second projection (resubmit) should succeed",
	)

	tableName := s.physicalTableName(versionID)

	var count int
	s.Require().NoError(s.db.NewRaw(
		"SELECT COUNT(*) FROM "+tableName+" WHERE instance_id = ?", instance.ID,
	).Scan(s.ctx, &count))
	s.Assert().Equal(1, count, "resubmit refreshes the single projection row, never appends a duplicate")

	type projected struct {
		Reason *string `bun:"reason"`
	}

	var row projected
	s.Require().NoError(s.db.NewRaw(
		"SELECT reason FROM "+tableName+" WHERE instance_id = ?", instance.ID,
	).Scan(s.ctx, &row))
	s.Require().NotNil(row.Reason)
	s.Assert().Equal("second", *row.Reason, "the surviving row reflects the latest write")
}

// TestStartInstanceProjectsDateField covers the date-field projection path: a
// filled date round-trips, and an OPTIONAL empty-string date must not fail the
// INSERT. The widgets override the parser's DATE inference with an explicit
// "text" column type, pinning the lossless TEXT projection (a temporal column
// would reject ” on strict dialects and scan back dialect-dependently).
func (s *StorageTableTestSuite) TestStartInstanceProjectsDateField() {
	schema := formEditorSchemaJSON(s.T(),
		formEditorWidget{Type: "date", Key: "event_date", Label: "Event Date", ColumnType: "text"},
		formEditorWidget{Type: "date", Key: "opt_date", Label: "Optional Date", ColumnType: "text"},
	)
	_, versionID := s.deployTableFlow("default", "storage-tbl-date", schema)

	instance := s.startTableInstance("storage-tbl-date", map[string]any{
		"event_date": "2026-06-22",
		"opt_date":   "",
	})

	tableName := s.physicalTableName(versionID)

	type projected struct {
		EventDate *string `bun:"event_date"`
	}

	var row projected
	s.Require().NoError(s.db.NewRaw(
		"SELECT event_date FROM "+tableName+" WHERE instance_id = ?", instance.ID,
	).Scan(s.ctx, &row))
	s.Require().NotNil(row.EventDate)
	s.Assert().Equal("2026-06-22", *row.EventDate, "a filled date round-trips through the TEXT column")
}

// TestPhysicalTableNamesAreUniqueAcrossTenants pins the cross-tenant isolation
// guarantee: two flows sharing a code in different tenants must project into
// distinct physical tables, never a single shared one.
func (s *StorageTableTestSuite) TestPhysicalTableNamesAreUniqueAcrossTenants() {
	_, versionA := s.deployTableFlow("default", "shared-code", s.formSchema())
	_, versionB := s.deployTableFlow("other-tenant", "shared-code", s.formSchema())

	nameA := s.physicalTableName(versionA)
	nameB := s.physicalTableName(versionB)

	s.Assert().NotEqual(nameA, nameB,
		"flows sharing a code across tenants must not share a physical table")

	// Both tables exist independently and are queryable.
	for _, name := range []string{nameA, nameB} {
		var count int
		s.Require().NoError(s.db.NewRaw("SELECT COUNT(*) FROM "+name).Scan(s.ctx, &count))
		s.Assert().Equal(0, count)
	}
}

// TestApproveSyncsProjection pins the H3 fix on the edit path: when an approver
// edits a field, the table-mode physical projection must refresh too, not only
// apv_instance.form_data. A PassAll node with a still-pending peer keeps the node
// running so the approval applies the form edit without completing the node (no
// graph traversal needed). The other edit paths (reject / transfer / rollback)
// funnel through the same SyncInstanceProjection call, so approve is the
// representative regression guard.
func (s *StorageTableTestSuite) TestApproveSyncsProjection() {
	flowID, versionID := s.deployPublishedTableFlow("storage-tbl-approve")

	node := &approval.FlowNode{
		FlowVersionID:    versionID,
		Key:              "approve-projection-node",
		Kind:             approval.NodeApproval,
		Name:             "Approve",
		ApprovalMethod:   approval.ApprovalParallel,
		PassRule:         approval.PassAll,
		FieldPermissions: map[string]approval.Permission{"reason": approval.PermissionEditable},
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "create approval node")

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        flowID,
		FlowVersionID: versionID,
		Title:         "Approve Projection",
		InstanceNo:    "APV-TBL-APPROVE-1",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
		FormData:      map[string]any{"reason": "before", "amount": 1},
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "create running instance")

	// Seed the initial projection row the way StartInstance would.
	s.Require().NoError(s.dispatcher.SyncInstanceProjection(s.ctx, s.db, inst), "seed initial projection")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, node.ID).ID,
		AssigneeID: "approver-1",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "create approver task")

	// A still-pending peer keeps the PassAll node running after the approval, so
	// no node-completion / graph traversal is needed for this projection check.
	peer := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, node.ID).ID,
		AssigneeID: "approver-1-peer",
		SortOrder:  2,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(peer).Exec(s.ctx)
	s.Require().NoError(err, "create peer task")

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	approve := wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, s.dispatcher))

	_, err = approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "approver-1", Name: "Approver"},
		Opinion:  "ok",
		FormData: map[string]any{"reason": "edited-by-approver"},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "approve with form edit")

	// The physical projection must reflect the approver's edit.
	tableName := s.physicalTableName(versionID)

	type projected struct {
		Reason *string `bun:"reason"`
	}

	var row projected
	s.Require().NoError(s.db.NewRaw(
		"SELECT reason FROM "+tableName+" WHERE instance_id = ?", inst.ID,
	).Scan(s.ctx, &row), "read projected row")
	s.Require().NotNil(row.Reason)
	s.Assert().Equal("edited-by-approver", *row.Reason,
		"approve must refresh the table-mode projection, not only apv_instance.form_data")
}

func (s *StorageTableTestSuite) TestDetailTableProjectsChildRows() {
	schema := formEditorSchemaJSON(s.T(),
		formEditorWidget{Type: "textarea", Key: "reason", Label: "Reason"},
		formEditorWidget{Type: "subform", Key: "items", Label: "Items", Columns: []formEditorWidget{
			{Type: "textfield", Key: "name", Label: "Name"},
			{Type: "number", Key: "qty", Label: "Qty"},
		}},
	)
	flowID, versionID := s.deployTableFlow("default", "storage-tbl-detail", schema)

	var formTables []approval.FormTable
	s.Require().NoError(s.db.NewSelect().Model(&formTables).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("version_id", versionID) }).
		OrderBy("source_field_key").
		Scan(s.ctx), "metadata should exist for main and child tables")
	s.Require().Len(formTables, 2, "one main table plus one child table")
	s.Assert().Equal("", formTables[0].SourceFieldKey, "main table carries the empty sentinel")
	s.Assert().Equal("items", formTables[1].SourceFieldKey, "child table carries its source field")
	s.Assert().Equal("apv_form_"+versionID+"__items", formTables[1].PhysicalTableName,
		"child names anchor on the version id, independent of the flow code")

	instance := s.startTableInstance("storage-tbl-detail", map[string]any{
		"reason": "trip",
		"items": []any{
			map[string]any{"name": "hotel", "qty": 2},
			map[string]any{"name": "taxi", "qty": 1},
		},
	})

	type childRow struct {
		RowIndex int    `bun:"row_index"`
		Name     string `bun:"name"`
	}

	var rows []childRow
	s.Require().NoError(s.db.NewRaw(
		"SELECT row_index, name FROM "+formTables[1].PhysicalTableName+" WHERE instance_id = ? ORDER BY row_index",
		instance.ID,
	).Scan(s.ctx, &rows), "child rows should be queryable")
	s.Require().Len(rows, 2, "each detail line projects one child row")
	s.Assert().Equal([]childRow{{0, "hotel"}, {1, "taxi"}}, rows, "row order follows the submitted list")

	// Resubmit-style rewrite replaces, never appends.
	var version approval.FlowVersion

	version.ID = versionID
	s.Require().NoError(s.db.NewSelect().Model(&version).WherePK().Scan(s.ctx), "load version")

	var flow approval.Flow

	flow.ID = flowID
	s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx), "load flow")

	s.Require().NoError(s.dispatcher.For(approval.StorageTable).Write(s.ctx, s.db, &flow, &version, instance.ID, map[string]any{
		"reason": "trip-v2",
		"items":  []any{map[string]any{"name": "train", "qty": 3}},
	}), "rewrite should succeed")

	rows = nil
	s.Require().NoError(s.db.NewRaw(
		"SELECT row_index, name FROM "+formTables[1].PhysicalTableName+" WHERE instance_id = ? ORDER BY row_index",
		instance.ID,
	).Scan(s.ctx, &rows), "child rows should be queryable after rewrite")
	s.Require().Len(rows, 1, "a rewrite replaces the previous detail lines")
	s.Assert().Equal(childRow{0, "train"}, rows[0], "the fresh line lands at row_index 0")
}
