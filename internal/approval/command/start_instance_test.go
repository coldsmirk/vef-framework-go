package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
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

func (g *MockInstanceNoGenerator) Generate(_ context.Context, _ string) (string, error) {
	g.counter++

	return fmt.Sprintf("TEST-%d", g.counter), nil
}

// StartInstanceTestSuite tests the StartInstanceHandler.
type StartInstanceTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.StartInstanceHandler
	fixture *FlowFixture
}

func (s *StartInstanceTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	pub := dispatcher.NewEventPublisher()
	validSvc := service.NewValidationService(nil)

	s.handler = command.NewStartInstanceHandler(s.db, eng, &MockInstanceNoGenerator{}, pub, validSvc)
}

func (s *StartInstanceTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *StartInstanceTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *StartInstanceTestSuite) TestStartSuccess() {
	applicant := approval.OperatorInfo{ID: "user-1", Name: "User One"}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  map[string]any{"reason": "test"},
	})
	s.Require().NoError(err, "Should start instance without error")
	s.Require().NotNil(instance, "Should not be nil")

	s.Assert().Equal(approval.InstanceRunning, instance.Status, "Instance should be running")
	s.Assert().Equal("user-1", instance.ApplicantID, "Should set applicant ID")
	s.Assert().NotEmpty(instance.InstanceNo, "Should generate instance number")
	s.Assert().NotEmpty(instance.Title, "Should generate title")

	// Verify action log created
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
		Scan(s.ctx), "Should not return error")
	s.Assert().GreaterOrEqual(len(logs), 1, "Should have at least 1 action log (submit)")
}

func (s *StartInstanceTestSuite) TestStartFlowNotFound() {
	applicant := approval.OperatorInfo{ID: "user-1", Name: "User One"}
	_, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "non-existent-flow",
		Applicant: applicant,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return expected error")
}

func (s *StartInstanceTestSuite) TestStartFlowNotActive() {
	// Deactivate the flow
	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("is_active", false).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	defer func() {
		_, _ = s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("is_active", true).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.fixture.FlowID) }).
			Exec(s.ctx)
	}()

	applicant := approval.OperatorInfo{ID: "user-1", Name: "User One"}
	_, err = s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrFlowNotActive, "Should return expected error")
}

func (s *StartInstanceTestSuite) TestStartWithFormData() {
	setPublishedFormSchema(s.T(), s.ctx, s.db, s.fixture.VersionID, &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "amount", Kind: approval.FieldNumber, Label: "Amount"},
			{Key: "description", Kind: approval.FieldTextarea, Label: "Description"},
		},
	})
	defer setPublishedFormSchema(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	applicant := approval.OperatorInfo{ID: "user-2", Name: "User Two"}
	formData := map[string]any{
		"amount":      1000,
		"description": "Business trip",
	}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  formData,
	})
	s.Require().NoError(err, "Should start instance with form data")
	s.Require().NotNil(instance, "Should not be nil")
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

	applicant := approval.OperatorInfo{ID: "user-template", Name: "Template User"}
	instance, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: applicant,
		FormData:  map[string]any{"reason": "Travel"},
	})
	s.Require().NoError(err, "Should start instance with lowerCamel title template keys")
	s.Assert().Equal("TEST-1-Template User-apv-cmd-test-flow-Travel", instance.Title, "Should render lowerCamel template keys consistently")
}

func (s *StartInstanceTestSuite) TestStartShouldRejectInvalidFormDataBySchema() {
	setPublishedFormSchema(s.T(), s.ctx, s.db, s.fixture.VersionID, &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldInput, Label: "Reason", IsRequired: true},
		},
	})
	defer setPublishedFormSchema(s.T(), s.ctx, s.db, s.fixture.VersionID, nil)

	_, err := s.handler.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: approval.OperatorInfo{ID: "user-invalid", Name: "Invalid User"},
		FormData:  map[string]any{"unknown": "value"},
	})
	s.Require().Error(err, "Should reject form data that violates form schema")

	var re result.Error
	require.ErrorAs(s.T(), err, &re, "Should return business error")
	s.Assert().Equal(shared.ErrCodeFormValidationFailed, re.Code, "Should return form validation error code")
}
