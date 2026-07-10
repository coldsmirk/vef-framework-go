package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &StartInstanceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// MockInstanceNoGenerator is a test implementation of InstanceNoGenerator.
type MockInstanceNoGenerator struct {
	counter int
}

func (g *MockInstanceNoGenerator) Generate(context.Context, string) (string, error) {
	g.counter++

	return fmt.Sprintf("TEST-%d", g.counter), nil
}

// FixedBusinessRefProvider returns one configured business reference.
type FixedBusinessRefProvider struct {
	Ref string
}

func (p *FixedBusinessRefProvider) OnInstanceCreated(
	context.Context,
	orm.DB,
	*approval.Flow,
	*approval.Instance,
) (string, error) {
	return p.Ref, nil
}

// StartInstanceTestSuite tests the StartInstanceHandler.
type StartInstanceTestSuite struct {
	suite.Suite

	ctx       context.Context
	db        orm.DB
	handler   cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	fixture   *FlowFixture
	projector *binding.Projector
}

func (s *StartInstanceTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine(s.db)
	validSvc := service.NewValidationService(nil)
	s.projector = binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), nil)

	s.handler = wrapWithBusAndDB(
		s.db,
		eventtest.NewFakeBus(),
		command.NewStartInstanceHandler(s.db, eng, &MockInstanceNoGenerator{}, validSvc,
			binding.NewNoopRefProvider(), s.projector, nil),
	)
}

func (s *StartInstanceTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *StartInstanceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *StartInstanceTestSuite) TestStartSuccess() {
	applicant := approval.UserInfo{ID: "user-1", Name: "User One"}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  map[string]any{"reason": "test"},
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start instance without error")
	s.Require().NotNil(instance, "TestStartSuccess should return a non-nil value")

	s.Assert().Equal(approval.InstanceRunning, instance.Status, "Instance should be running")
	s.Assert().Equal("user-1", instance.ApplicantID, "Should set applicant ID")
	s.Assert().NotEmpty(instance.InstanceNo, "Should generate instance number")
	s.Assert().NotEmpty(instance.Title, "Should generate title")

	// Verify action log created
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
		Scan(s.ctx), "TestStartSuccess should complete without error")
	s.Assert().GreaterOrEqual(len(logs), 1, "Should have at least 1 action log (submit)")
}

func (s *StartInstanceTestSuite) TestStartFlowNotFound() {
	applicant := approval.UserInfo{ID: "user-1", Name: "User One"}
	_, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "non-existent-flow",
		Applicant: applicant,
		Caller:    approval.SystemCaller,
	})
	s.Require().Error(err, "TestStartFlowNotFound should return an error")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return expected error")
}

func (s *StartInstanceTestSuite) TestStartFlowNotActive() {
	// Deactivate the flow
	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("is_active", false).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "TestStartFlowNotActive should complete without error")

	defer func() {
		_, _ = s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("is_active", true).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
			Exec(s.ctx)
	}()

	applicant := approval.UserInfo{ID: "user-1", Name: "User One"}
	_, err = s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		Caller:    approval.SystemCaller,
	})
	s.Require().Error(err, "TestStartFlowNotActive should return an error")
	s.Assert().ErrorIs(err, shared.ErrFlowNotActive, "Should return expected error")
}

func (s *StartInstanceTestSuite) TestStartWithFormData() {
	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, []approval.FormFieldDefinition{
		{Key: "amount", Kind: approval.FieldNumber, Label: "Amount"},
		{Key: "description", Kind: approval.FieldTextarea, Label: "Description"},
	})
	defer setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	applicant := approval.UserInfo{ID: "user-2", Name: "User Two"}
	formData := map[string]any{
		"amount":      1000,
		"description": "Business trip",
	}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  formData,
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start instance with form data")
	s.Require().NotNil(instance, "TestStartWithFormData should return a non-nil value")
	s.Assert().NotNil(instance.FormData, "Should store form data")
}

func (s *StartInstanceTestSuite) TestStartShouldRenderTemplateWithCompatibleKeys() {
	defer func() {
		_, err := s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("instance_title_template", "apv-cmd-test {{.instanceNo}}").
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should restore default instance title template")
	}()

	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("instance_title_template", "{{.instanceNo}}-{{.applicantName}}-{{.flowCode}}-{{index .formData \"reason\"}}").
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should update instance title template")

	applicant := approval.UserInfo{ID: "user-template", Name: "Template User"}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  map[string]any{"reason": "Travel"},
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start instance with lowerCamel title template keys")
	// Build the expectation from the generated instance number: the mock
	// generator's counter is shared across the whole suite, so a hard-coded
	// "TEST-1" would couple this test to its alphabetical position.
	s.Assert().Equal(instance.InstanceNo+"-Template User-apv-cmd-test-flow-Travel", instance.Title, "Should render lowerCamel template keys consistently")
}

func (s *StartInstanceTestSuite) TestStartShouldRejectInvalidFormDataBySchema() {
	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput, Label: "Reason", IsRequired: true},
	})
	defer setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	_, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: approval.UserInfo{ID: "user-invalid", Name: "Invalid User"},
		FormData:  map[string]any{"unknown": "value"},
		Caller:    approval.SystemCaller,
	})
	s.Require().Error(err, "Should reject form data that violates form schema")

	var re result.Error
	require.ErrorAs(s.T(), err, &re, "Should return business error")
	s.Assert().Equal(shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
}

// flipFlowToBusinessBinding points the fixture flow at table with the full
// linkage-column set and returns a restore func for the standalone baseline.
func businessBindingForTable(table string) *approval.BusinessBindingConfig {
	instanceID, startedAt, finishedAt := "apv_instance_id", "apv_started_at", "apv_finished_at"

	return &approval.BusinessBindingConfig{
		TableName:        table,
		KeyColumns:       []string{"id"},
		StatusColumn:     "approval_status",
		InstanceIDColumn: &instanceID,
		StartedAtColumn:  &startedAt,
		FinishedAtColumn: &finishedAt,
	}
}

func (s *StartInstanceTestSuite) flipFlowToBusinessBinding(table string) func() {
	s.T().Helper()

	flow := &approval.Flow{
		BindingMode:     approval.BindingBusiness,
		BusinessBinding: businessBindingForTable(table),
	}
	flow.ID = s.fixture.FlowID
	_, err := s.db.NewUpdate().Model(flow).Select("binding_mode", "business_binding").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should flip the fixture flow to a business binding")
	_, err = s.db.NewUpdate().
		Model((*approval.FlowVersion)(nil)).
		Set("business_binding", flow.BusinessBinding).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.VersionID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set the published version binding snapshot")

	return func() {
		_, _ = s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("binding_mode", approval.BindingStandalone).
			Set("business_binding", nil).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
			Exec(s.ctx)
		_, _ = s.db.NewUpdate().
			Model((*approval.FlowVersion)(nil)).
			Set("business_binding", nil).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.VersionID) }).
			Exec(s.ctx)
	}
}

func (s *StartInstanceTestSuite) TestStartBusinessLinkageWriteBack() {
	_, err := s.db.NewRaw(`CREATE TABLE biz_start_order (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32),
		apv_started_at TIMESTAMP,
		apv_finished_at TIMESTAMP
	)`).Exec(s.ctx)
	s.Require().NoError(err, "Should create the business table")

	defer func() { _, _ = s.db.NewRaw(`DROP TABLE biz_start_order`).Exec(s.ctx) }()

	// The row carries leftovers from a previous approval round so the test can
	// prove the started projection overwrites and clears them.
	_, err = s.db.NewRaw(`INSERT INTO biz_start_order (id, approval_status, apv_instance_id, apv_finished_at)
		VALUES ('ord-1', 'rejected', 'inst-old', CURRENT_TIMESTAMP)`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the business row")

	restore := s.flipFlowToBusinessBinding("biz_start_order")
	defer restore()

	ref := "ord-1"
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:    "apv-cmd-test-flow",
		Applicant:   approval.UserInfo{ID: "user-1", Name: "User One"},
		BusinessRef: &ref,
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start a business-bound instance")

	var (
		status, instanceID            string
		startedAtNull, finishedAtNull bool
	)

	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, COALESCE(apv_instance_id, ''),
		apv_started_at IS NULL, apv_finished_at IS NULL
		FROM biz_start_order WHERE id = 'ord-1'`).
		Scan(s.ctx, &status, &instanceID, &startedAtNull, &finishedAtNull),
		"Should read back the business row")

	s.Assert().Equal("running", status, "Business status should flip to running at initiation")
	s.Assert().Equal(instance.ID, instanceID, "Business row should carry the new instance id")
	s.Assert().False(startedAtNull, "Business row should carry the start time")
	s.Assert().True(finishedAtNull, "The finish time left by the previous round should be cleared")
}

func (s *StartInstanceTestSuite) TestBusinessRefProviderReplacesBlankInput() {
	_, err := s.db.NewRaw(`CREATE TABLE biz_provider_order (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32),
		apv_started_at TIMESTAMP,
		apv_finished_at TIMESTAMP
	)`).Exec(s.ctx)

	s.Require().NoError(err, "Should create the provider-backed business table")
	defer func() { _, _ = s.db.NewRaw(`DROP TABLE biz_provider_order`).Exec(s.ctx) }()

	_, err = s.db.NewRaw(`INSERT INTO biz_provider_order (id, approval_status)
		VALUES ('ord-provider', 'submitted')`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the provider-backed business row")

	restore := s.flipFlowToBusinessBinding("biz_provider_order")
	defer restore()

	handler := wrapWithBusAndDB(
		s.db,
		eventtest.NewFakeBus(),
		command.NewStartInstanceHandler(
			s.db,
			buildTestEngine(s.db),
			new(MockInstanceNoGenerator),
			service.NewValidationService(nil),
			&FixedBusinessRefProvider{Ref: "ord-provider"},
			s.projector,
			nil,
		),
	)

	blankRef := "   "
	instance, err := handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:    "apv-cmd-test-flow",
		Applicant:   approval.UserInfo{ID: "provider-user", Name: "Provider User"},
		BusinessRef: &blankRef,
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "Provider should replace a whitespace-only business reference")
	s.Require().NotNil(instance.BusinessRef, "Provider reference should persist on the instance")
	s.Assert().Equal("ord-provider", *instance.BusinessRef,
		"Provider reference should replace the semantically empty caller value")

	var status, owner string
	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, apv_instance_id
		FROM biz_provider_order WHERE id = 'ord-provider'`).Scan(s.ctx, &status, &owner),
		"Should read the provider-backed business row")
	s.Assert().Equal("running", status, "Provider-backed target should receive the running state")
	s.Assert().Equal(instance.ID, owner, "Provider-backed target should receive the instance fence")
}

func (s *StartInstanceTestSuite) TestStartRollsBackWhenLinkageWriteFails() {
	// The flow points at a table that does not exist, so the synchronous
	// started write-back fails inside the start transaction and the whole
	// initiation must roll back — no instance may survive.
	restore := s.flipFlowToBusinessBinding("biz_missing_table")
	defer restore()

	ref := "ord-x"
	err := s.db.RunInTx(s.ctx, func(txCtx context.Context, tx orm.DB) error {
		_, handleErr := s.handler.Handle(contextx.SetDB(txCtx, tx), command.StartInstanceCmd{
			FlowCode:    "apv-cmd-test-flow",
			Applicant:   approval.UserInfo{ID: "user-1", Name: "User One"},
			BusinessRef: &ref,
			Caller:      approval.SystemCaller,
		})

		return handleErr
	})
	s.Require().Error(err, "Start must fail when the linkage write-back fails")

	count, countErr := s.db.NewSelect().
		Model((*approval.Instance)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("business_ref", "ord-x") }).
		Count(s.ctx)
	s.Require().NoError(countErr, "Should count instances after the failed start")
	s.Assert().Zero(count, "The rolled-back initiation must not leave an instance behind")
}

func (s *StartInstanceTestSuite) TestBusinessTargetOwnershipAndStaleInstanceFencing() {
	_, err := s.db.NewRaw(`CREATE TABLE biz_owned_order (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32),
		apv_started_at TIMESTAMP,
		apv_finished_at TIMESTAMP
	)`).Exec(s.ctx)

	s.Require().NoError(err, "Should create the ownership-fencing business table")
	defer func() { _, _ = s.db.NewRaw(`DROP TABLE biz_owned_order`).Exec(s.ctx) }()

	_, err = s.db.NewRaw(`INSERT INTO biz_owned_order (id, approval_status)
		VALUES ('ord-owned', 'submitted')`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the ownership-fencing business row")

	restore := s.flipFlowToBusinessBinding("biz_owned_order")
	defer restore()

	ref := "ord-owned"
	first, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:    "apv-cmd-test-flow",
		Applicant:   approval.UserInfo{ID: "owner-1", Name: "Owner One"},
		BusinessRef: &ref,
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "First approval should claim the business target")

	err = s.db.RunInTx(s.ctx, func(txCtx context.Context, tx orm.DB) error {
		_, startErr := s.handler.Handle(contextx.SetDB(txCtx, tx), command.StartInstanceCmd{
			FlowCode:    "apv-cmd-test-flow",
			Applicant:   approval.UserInfo{ID: "owner-2", Name: "Owner Two"},
			BusinessRef: &ref,
			Caller:      approval.SystemCaller,
		})

		return startErr
	})
	s.Require().ErrorIs(err, shared.ErrBindingTargetBusy,
		"A non-final owner should block another approval for the same business record")

	count, err := s.db.NewSelect().Model((*approval.Instance)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("business_ref", ref) }).Count(s.ctx)
	s.Require().NoError(err, "Should count business-bound instances after the rejected claim")
	s.Assert().Equal(int64(1), count, "Rejected target claim should roll its instance back")

	finishedAt := timex.Now()
	first.Status = approval.InstanceApproved
	first.FinishedAt = &finishedAt
	_, err = s.db.NewUpdate().Model(first).Select("status", "finished_at").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Test setup should close the first approval instance")

	second, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:    "apv-cmd-test-flow",
		Applicant:   approval.UserInfo{ID: "owner-2", Name: "Owner Two"},
		BusinessRef: &ref,
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "A completed owner should allow a new approval round to claim the target")

	var status, owner string
	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, apv_instance_id
		FROM biz_owned_order WHERE id = 'ord-owned'`).Scan(s.ctx, &status, &owner),
		"Should read the re-owned business row")
	s.Assert().Equal("running", status, "New approval round should project its running state")
	s.Assert().Equal(second.ID, owner, "New approval round should replace the business owner fence")

	first.Status = approval.InstanceRejected
	first.FinishedAt = &finishedAt
	s.Require().NoError(s.projector.Project(s.ctx, s.db, first),
		"A stale instance transition should be ignored after target ownership changes")

	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, apv_instance_id
		FROM biz_owned_order WHERE id = 'ord-owned'`).Scan(s.ctx, &status, &owner),
		"Should read the business row after the stale projection attempt")
	s.Assert().Equal("running", status, "Stale instance must not overwrite the new owner's state")
	s.Assert().Equal(second.ID, owner, "Stale instance must not overwrite the new owner fence")
}

func (s *StartInstanceTestSuite) TestPublishedVersionKeepsBindingSnapshotAfterFlowEdit() {
	tables := []string{"biz_snapshot_old", "biz_snapshot_new"}
	defer func() {
		for _, table := range tables {
			_, _ = s.db.NewRaw(`DROP TABLE ` + table).Exec(s.ctx)
		}
	}()

	for _, table := range tables {
		_, err := s.db.NewRaw(`CREATE TABLE ` + table + ` (
			id VARCHAR(64) PRIMARY KEY,
			approval_status VARCHAR(32),
			apv_instance_id VARCHAR(32),
			apv_started_at TIMESTAMP,
			apv_finished_at TIMESTAMP
		)`).Exec(s.ctx)

		s.Require().NoError(err, "Should create business table %s", table)

		_, err = s.db.NewRaw(`INSERT INTO ` + table + ` (id, approval_status)
			VALUES ('ord-snapshot', 'submitted')`).Exec(s.ctx)
		s.Require().NoError(err, "Should seed business table %s", table)
	}

	restore := s.flipFlowToBusinessBinding("biz_snapshot_old")
	defer restore()

	flow := &approval.Flow{BindingMode: approval.BindingBusiness, BusinessBinding: businessBindingForTable("biz_snapshot_new")}
	flow.ID = s.fixture.FlowID
	_, err := s.db.NewUpdate().Model(flow).Select("binding_mode", "business_binding").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should edit the mutable flow binding without touching its published version")

	ref := "ord-snapshot"
	_, err = s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:    "apv-cmd-test-flow",
		Applicant:   approval.UserInfo{ID: "snapshot-user", Name: "Snapshot User"},
		BusinessRef: &ref,
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "Instance should start against the published binding snapshot")

	var oldStatus, newStatus string
	s.Require().NoError(s.db.NewRaw(`SELECT approval_status FROM biz_snapshot_old WHERE id = 'ord-snapshot'`).Scan(s.ctx, &oldStatus),
		"Should read the version-pinned business row")
	s.Require().NoError(s.db.NewRaw(`SELECT approval_status FROM biz_snapshot_new WHERE id = 'ord-snapshot'`).Scan(s.ctx, &newStatus),
		"Should read the mutable-flow business row")
	s.Assert().Equal("running", oldStatus, "Published version should continue writing its binding snapshot")
	s.Assert().Equal("submitted", newStatus, "Mutable flow edit should not redirect an existing version")
}

func (s *StartInstanceTestSuite) TestEventualImmediateCompletionProjectsOnlyLatestState() {
	_, err := s.db.NewRaw(`CREATE TABLE biz_eventual_final (
		id VARCHAR(64) PRIMARY KEY,
		approval_status VARCHAR(32),
		apv_instance_id VARCHAR(32),
		apv_started_at TIMESTAMP,
		apv_finished_at TIMESTAMP
	)`).Exec(s.ctx)

	s.Require().NoError(err, "Should create the eventual-completion business table")
	defer func() { _, _ = s.db.NewRaw(`DROP TABLE biz_eventual_final`).Exec(s.ctx) }()

	_, err = s.db.NewRaw(`INSERT INTO biz_eventual_final (id, approval_status)
		VALUES ('ord-final', 'submitted')`).Exec(s.ctx)
	s.Require().NoError(err, "Should seed the eventual-completion business row")

	fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, "binding-eventual-final", simpleFlowDef())
	businessBinding := businessBindingForTable("biz_eventual_final")
	flow := &approval.Flow{BindingMode: approval.BindingBusiness, BusinessBinding: businessBinding}
	flow.ID = fixture.FlowID
	_, err = s.db.NewUpdate().Model(flow).Select("binding_mode", "business_binding").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should configure the immediate-completion flow binding")
	_, err = s.db.NewUpdate().Model((*approval.FlowVersion)(nil)).
		Set("business_binding", businessBinding).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(fixture.VersionID) }).Exec(s.ctx)
	s.Require().NoError(err, "Should configure the published binding snapshot")

	cfg := &config.ApprovalConfig{BusinessBinding: config.ApprovalBusinessBindingConfig{
		Consistency: config.ApprovalBindingEventual,
		BatchSize:   10,
	}}
	projector := binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), cfg)
	hooks := engine.NewLifecycleHookRunner(projector, nil)
	eng := buildTestEngineWithHooks(s.db, hooks)
	start := wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewStartInstanceHandler(
		s.db,
		eng,
		&MockInstanceNoGenerator{},
		service.NewValidationService(nil),
		binding.NewNoopRefProvider(),
		projector,
		nil,
	))

	ref := "ord-final"
	instance, err := start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:    "binding-eventual-final-flow",
		Applicant:   approval.UserInfo{ID: "final-user", Name: "Final User"},
		BusinessRef: &ref,
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "Immediate-completion instance should commit its desired projection")
	s.Assert().Equal(approval.InstanceApproved, instance.Status, "Start-to-end flow should finish as approved")
	s.Require().NotNil(instance.BusinessProjectionID, "Business-bound instance should link to its projection")

	projection := new(approval.BusinessProjection)
	projection.ID = *instance.BusinessProjectionID
	s.Require().NoError(s.db.NewSelect().Model(projection).WherePK().Scan(s.ctx),
		"Should load the immediate-completion projection")
	s.Assert().Equal(approval.InstanceApproved, projection.DesiredStatus,
		"Projection should retain only the latest final desired state")
	s.Assert().Equal(int64(2), projection.DesiredRevision,
		"Running bind and final transition should advance one durable target revision")
	s.Assert().Zero(projection.AppliedRevision,
		"Eventual mode should not write the business row inside the approval action")
	s.Require().NotNil(projection.NextAttemptAt,
		"A pending eventual revision should be immediately eligible for worker convergence")

	var status, owner string
	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, COALESCE(apv_instance_id, '')
		FROM biz_eventual_final WHERE id = 'ord-final'`).Scan(s.ctx, &status, &owner),
		"Should read the business row before worker convergence")
	s.Assert().Equal("submitted", status, "Approval commit should not synchronously mutate eventual business state")
	s.Assert().Empty(owner, "Approval commit should not synchronously stamp eventual ownership")

	worker := binding.NewWorker(s.db, eventtest.NewFakeBus(), binding.NewWriter(), cfg)
	processed, err := worker.ProcessPending(s.ctx)
	s.Require().NoError(err, "Eventual worker should apply the latest desired state")
	s.Assert().Equal(1, processed, "Eventual worker should process the one pending target")

	s.Require().NoError(s.db.NewRaw(`SELECT approval_status, apv_instance_id
		FROM biz_eventual_final WHERE id = 'ord-final'`).Scan(s.ctx, &status, &owner),
		"Should read the converged business row")
	s.Assert().Equal("approved", status, "Worker should project the final state without an intermediate running write")
	s.Assert().Equal(instance.ID, owner, "Worker should stamp the final instance owner")
}
