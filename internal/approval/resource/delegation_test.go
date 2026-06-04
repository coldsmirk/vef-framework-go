package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/security"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// delegationStart/delegationEnd are a valid window (start < end) supplied on
// every update so the merged row satisfies the ck_apv_delegation__time_range
// CHECK constraint; a real client always sends these.
const (
	delegationStart = "2030-01-01T00:00:00Z"
	delegationEnd   = "2030-06-01T00:00:00Z"
)

// DelegationOwnershipTestSuite exercises the owner-scoping enforced on the
// delegation update/delete handlers. It runs on the default in-memory SQLite
// datasource so the full RPC → auth → crud path is covered without Docker.
type DelegationOwnershipTestSuite struct {
	apptest.Suite

	ctx context.Context
	db  orm.DB
}

func TestDelegationOwnership(t *testing.T) {
	suite.Run(t, new(DelegationOwnershipTestSuite))
}

func (s *DelegationOwnershipTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.SetupApp(
		fx.Replace(
			&security.JWTConfig{Secret: security.DefaultJWTSecret, Audience: "test_app"},
			newApprovalConfig(),
		),
		fx.Provide(func() context.Context { return s.ctx }),
		iapproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.UserInfoResolver { return &MockUserInfoResolver{} }, fx.As(new(approval.UserInfoResolver))),
			fx.Annotate(func() approval.PrincipalDepartmentResolver { return &MockPrincipalDepartmentResolver{} }, fx.As(new(approval.PrincipalDepartmentResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Decorate(
			fx.Annotate(func() security.PermissionChecker { return &MockPermissionChecker{} }, fx.As(new(security.PermissionChecker))),
		),
		fx.Populate(&s.db),
	)
}

func (s *DelegationOwnershipTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// TestDelegationOwnership covers the update/delete owner guard, including the
// delegator-reassignment escalation: a non-super-admin owner passes the
// owner check on the old row but must not be able to move ownership to another
// user via the client-supplied delegatorId.
func (s *DelegationOwnershipTestSuite) TestDelegationOwnership() {
	deleteAll(s.ctx, s.db, (*approval.Delegation)(nil))

	ownerToken := s.GenerateToken(newTenantUser("alice", "Alice", "user"))
	otherToken := s.GenerateToken(newTenantUser("mallory", "Mallory", "user"))
	adminToken := s.GenerateToken(newTenantUser("root", "Root", approval.SuperAdminRole))

	s.Run("OwnerCannotReassignDelegator", func() {
		s.seedDelegation("d-reassign", "alice", "bob")

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{Resource: "approval/delegation", Action: "update", Version: "v1"},
			Params: map[string]any{
				"id":          "d-reassign",
				"delegatorId": "victim", // attacker attempts to reassign ownership
				"delegateeId": "carol",  // a legitimate field change in the same request
				"startTime":   delegationStart,
				"endTime":     delegationEnd,
			},
		}, ownerToken)

		s.Require().Equal(http.StatusOK, resp.StatusCode, "owner update should succeed")
		s.True(s.ReadResult(resp).IsOk(), "owner update should report success")

		got := s.loadDelegation("d-reassign")
		s.Equal("alice", got.DelegatorID, "delegator must stay pinned to the original owner, not the client-supplied victim")
		s.Equal("carol", got.DelegateeID, "a legitimate delegatee change should still apply")
	})

	s.Run("NonOwnerCannotUpdate", func() {
		s.seedDelegation("d-update", "alice", "bob")

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{Resource: "approval/delegation", Action: "update", Version: "v1"},
			Params: map[string]any{
				"id":          "d-update",
				"delegatorId": "alice",
				"delegateeId": "mallory-pick",
				"startTime":   delegationStart,
				"endTime":     delegationEnd,
			},
		}, otherToken)

		s.NotEqual(http.StatusOK, resp.StatusCode, "a non-owner update must be rejected")

		got := s.loadDelegation("d-update")
		s.Equal("bob", got.DelegateeID, "a rejected update must not mutate the row")
	})

	s.Run("NonOwnerCannotDelete", func() {
		s.seedDelegation("d-delete", "alice", "bob")

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{Resource: "approval/delegation", Action: "delete", Version: "v1"},
			Params:     map[string]any{"id": "d-delete"},
		}, otherToken)

		s.NotEqual(http.StatusOK, resp.StatusCode, "a non-owner delete must be rejected")
		s.True(s.delegationExists("d-delete"), "a rejected delete must leave the row intact")
	})

	s.Run("OwnerCanUpdateOwnDelegation", func() {
		s.seedDelegation("d-legit", "alice", "bob")

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{Resource: "approval/delegation", Action: "update", Version: "v1"},
			Params: map[string]any{
				"id":          "d-legit",
				"delegatorId": "alice",
				"delegateeId": "dave",
				"startTime":   delegationStart,
				"endTime":     delegationEnd,
			},
		}, ownerToken)

		s.Require().Equal(http.StatusOK, resp.StatusCode, "owner update should succeed")
		s.True(s.ReadResult(resp).IsOk(), "owner update should report success")

		got := s.loadDelegation("d-legit")
		s.Equal("alice", got.DelegatorID, "the owner stays the delegator")
		s.Equal("dave", got.DelegateeID, "the owner can change the delegatee")
	})

	s.Run("SuperAdminCanReassignDelegator", func() {
		s.seedDelegation("d-admin", "alice", "bob")

		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{Resource: "approval/delegation", Action: "update", Version: "v1"},
			Params: map[string]any{
				"id":          "d-admin",
				"delegatorId": "frank",
				"delegateeId": "grace",
				"startTime":   delegationStart,
				"endTime":     delegationEnd,
			},
		}, adminToken)

		s.Require().Equal(http.StatusOK, resp.StatusCode, "super-admin update should succeed")
		s.True(s.ReadResult(resp).IsOk(), "super-admin update should report success")

		got := s.loadDelegation("d-admin")
		s.Equal("frank", got.DelegatorID, "a super-admin may reassign the delegator")
	})
}

func (s *DelegationOwnershipTestSuite) seedDelegation(id, delegator, delegatee string) {
	deleg := &approval.Delegation{
		DelegatorID: delegator,
		DelegateeID: delegatee,
		StartTime:   timex.Now(),
		EndTime:     timex.Now().AddHours(24),
		IsActive:    true,
	}
	deleg.ID = id

	_, err := s.db.NewInsert().Model(deleg).Exec(s.ctx)
	s.Require().NoError(err, "seed delegation %q", id)
}

func (s *DelegationOwnershipTestSuite) loadDelegation(id string) approval.Delegation {
	var got approval.Delegation

	err := s.db.NewSelect().Model(&got).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("id", id) }).
		Scan(s.ctx)
	s.Require().NoError(err, "load delegation %q", id)

	return got
}

func (s *DelegationOwnershipTestSuite) delegationExists(id string) bool {
	exists, err := s.db.NewSelect().Model((*approval.Delegation)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("id", id) }).
		Exists(s.ctx)
	s.Require().NoError(err, "check delegation %q", id)

	return exists
}
