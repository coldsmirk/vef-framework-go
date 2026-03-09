package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &PublishVersionTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// PublishVersionTestSuite tests the PublishVersionHandler.
type PublishVersionTestSuite struct {
	suite.Suite

	ctx            context.Context
	db             orm.DB
	publishHandler *command.PublishVersionHandler
	deployHandler  *command.DeployFlowHandler
	flowID         string
}

func (s *PublishVersionTestSuite) SetupSuite() {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "publish-test",
		Name:     "Publish Test Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	flow := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "publish-test-flow",
		Name:                   "Publish Test Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
		CurrentVersion:         0,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")

	s.flowID = flow.ID
	s.deployHandler = command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService())
	s.publishHandler = command.NewPublishVersionHandler(s.db, dispatcher.NewEventPublisher())
}

func (s *PublishVersionTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.EventOutbox)(nil),
		(*approval.FlowEdge)(nil),
		(*approval.FlowNode)(nil),
		(*approval.FlowVersion)(nil),
	)

	// Reset flow CurrentVersion to 0 for test isolation
	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("current_version", 0).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.flowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should reset flow CurrentVersion")
}

func (s *PublishVersionTestSuite) TearDownSuite() {
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

// deployVersion deploys a simple flow definition and returns the draft version.
func (s *PublishVersionTestSuite) deployVersion() *approval.FlowVersion {
	version, err := s.deployHandler.Handle(s.ctx, command.DeployFlowCmd{
		FlowID:         s.flowID,
		FlowDefinition: simpleFlowDef(),
	})
	s.Require().NoError(err, "Should deploy version")

	return version
}

func (s *PublishVersionTestSuite) TestPublishSuccess() {
	version := s.deployVersion()

	_, err := s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin-user",
	})
	s.Require().NoError(err, "Should publish version without error")

	// Verify version status and published fields
	var published approval.FlowVersion

	published.ID = version.ID
	err = s.db.NewSelect().Model(&published).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find published version")
	s.Assert().Equal(approval.VersionPublished, published.Status, "Should set status to published")
	s.Assert().NotNil(published.PublishedAt, "Should set PublishedAt")
	s.Require().NotNil(published.PublishedBy, "Should set PublishedBy")
	s.Assert().Equal("admin-user", *published.PublishedBy, "Should set correct PublishedBy")

	// Verify event outbox record created
	var outboxRecords []approval.EventOutbox

	err = s.db.NewSelect().
		Model(&outboxRecords).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("event_type", "approval.flow.published")
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query event outbox")
	s.Require().Len(outboxRecords, 1, "Should insert one event outbox record")
	s.Assert().Equal(approval.EventOutboxPending, outboxRecords[0].Status, "Should set event status to pending")
}

func (s *PublishVersionTestSuite) TestPublishVersionNotFound() {
	_, err := s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  "non-existent-version-id",
		OperatorID: "admin-user",
	})
	s.Require().Error(err, "Should fail for non-existent version")
	s.Assert().ErrorIs(err, shared.ErrVersionNotFound, "Should return ErrVersionNotFound")
}

func (s *PublishVersionTestSuite) TestPublishAlreadyPublished() {
	version := s.deployVersion()

	// Publish first time
	_, err := s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin-user",
	})
	s.Require().NoError(err, "Should publish version first time")

	// Publish same version again - should fail
	_, err = s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin-user",
	})
	s.Require().Error(err, "Should fail for already published version")
	s.Assert().ErrorIs(err, shared.ErrVersionNotDraft, "Should return ErrVersionNotDraft")
}

func (s *PublishVersionTestSuite) TestPublishUpdatesFlowCurrentVersion() {
	version := s.deployVersion()

	_, err := s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  version.ID,
		OperatorID: "admin-user",
	})
	s.Require().NoError(err, "Should publish version")

	// Verify flow's CurrentVersion is updated
	var flow approval.Flow

	flow.ID = s.flowID
	err = s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find flow")
	s.Assert().Equal(version.Version, flow.CurrentVersion, "Should update flow CurrentVersion")
}

func (s *PublishVersionTestSuite) TestPublishArchivesOldVersions() {
	// Deploy and publish first version
	v1 := s.deployVersion()
	_, err := s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  v1.ID,
		OperatorID: "admin-user",
	})
	s.Require().NoError(err, "Should publish first version")

	// Deploy and publish second version
	v2 := s.deployVersion()
	_, err = s.publishHandler.Handle(s.ctx, command.PublishVersionCmd{
		VersionID:  v2.ID,
		OperatorID: "admin-user",
	})
	s.Require().NoError(err, "Should publish second version")

	// Verify first version is archived
	var oldVersion approval.FlowVersion

	oldVersion.ID = v1.ID
	err = s.db.NewSelect().Model(&oldVersion).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find old version")
	s.Assert().Equal(approval.VersionArchived, oldVersion.Status, "Should archive old published version")

	// Verify second version is published
	var newVersion approval.FlowVersion

	newVersion.ID = v2.ID
	err = s.db.NewSelect().Model(&newVersion).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find new version")
	s.Assert().Equal(approval.VersionPublished, newVersion.Status, "Should keep new version published")

	// Verify flow CurrentVersion matches v2
	var flow approval.Flow

	flow.ID = s.flowID
	err = s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx)
	s.Require().NoError(err, "Should find flow")
	s.Assert().Equal(v2.Version, flow.CurrentVersion, "Should update flow CurrentVersion to v2")
}
