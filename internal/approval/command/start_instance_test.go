package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
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

// StartInstanceTestSuite tests the StartInstanceHandler.
type StartInstanceTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	fixture *FlowFixture
}

func (s *StartInstanceTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine(s.db)
	validSvc := service.NewValidationService(nil)

	s.handler = wrapWithBusAndDB(
		s.db,
		eventtest.NewFakeBus(),
		command.NewStartInstanceHandler(s.db, eng, &MockInstanceNoGenerator{}, validSvc, binding.NewNoopRefProvider(), binding.NewWriter(binding.NewIdentityResolver()), nil),
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
func (s *StartInstanceTestSuite) flipFlowToBusinessBinding(table string) func() {
	s.T().Helper()

	instanceID, startedAt, finishedAt := "apv_instance_id", "apv_started_at", "apv_finished_at"
	flow := &approval.Flow{
		BindingMode: approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			TableName:        table,
			KeyColumns:       []string{"id"},
			StatusColumn:     "approval_status",
			InstanceIDColumn: &instanceID,
			StartedAtColumn:  &startedAt,
			FinishedAtColumn: &finishedAt,
		},
	}
	flow.ID = s.fixture.FlowID
	_, err := s.db.NewUpdate().Model(flow).Select("binding_mode", "business_binding").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should flip the fixture flow to a business binding")

	return func() {
		_, _ = s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("binding_mode", approval.BindingStandalone).
			Set("business_binding", nil).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
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
