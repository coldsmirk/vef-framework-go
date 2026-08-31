package vef

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/app"
	"github.com/coldsmirk/vef-framework-go/approval"
	iapp "github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/security"
)

// HostMiddleware is shaped like an application's own middleware: it names the
// public contract, which is all a host in another module can reach.
type HostMiddleware struct{}

func (*HostMiddleware) Name() string { return "host-middleware" }

func (*HostMiddleware) Order() int { return 460 }

func (*HostMiddleware) Apply(fiber.Router) {}

// HostAuthenticator is shaped like an application's own login mechanism.
type HostAuthenticator struct{}

func (*HostAuthenticator) Supports(authType string) bool { return authType == "host" }

func (*HostAuthenticator) Authenticate(context.Context, security.Authentication) (*security.Principal, error) {
	return nil, nil
}

// TestProvideMiddlewareReachesTheRouter pins the property a host depends on:
// the type ProvideMiddleware puts into the group is the very type internal/app
// collects. fx matches a group by exact type and drops a mismatch without an
// error, so a break here surfaces as a route that silently never registers —
// declaring the collector exactly as the framework does is what catches it.
func TestProvideMiddlewareReachesTheRouter(t *testing.T) {
	type collector struct {
		fx.In

		Middlewares []iapp.Middleware `group:"vef:app:middlewares"`
	}

	var collected []iapp.Middleware

	fxApp := fx.New(
		ProvideMiddleware(func() app.Middleware { return new(HostMiddleware) }),
		fx.Invoke(func(c collector) { collected = c.Middlewares }),
		fx.NopLogger,
	)

	require.NoError(t, fxApp.Err(), "The middleware graph should resolve")
	require.Len(t, collected, 1,
		"A middleware registered through the public contract must reach the group the router assembles from")
	require.Equal(t, "host-middleware", collected[0].Name(),
		"The collected middleware should be the one that was registered")
}

// TestProvideAuthenticatorReachesTheAuthManager pins the same property for the
// login mechanism group: a custom authenticator must arrive where AuthManager
// aggregates, or security/auth.login rejects its type as unsupported.
func TestProvideAuthenticatorReachesTheAuthManager(t *testing.T) {
	type collector struct {
		fx.In

		Authenticators []security.Authenticator `group:"vef:security:authenticators"`
	}

	var collected []security.Authenticator

	fxApp := fx.New(
		ProvideAuthenticator(func() security.Authenticator { return new(HostAuthenticator) }),
		fx.Invoke(func(c collector) { collected = c.Authenticators }),
		fx.NopLogger,
	)

	require.NoError(t, fxApp.Err(), "The authenticator graph should resolve")
	require.Len(t, collected, 1,
		"An authenticator registered through the public contract must reach the group AuthManager aggregates")
	require.True(t, collected[0].Supports("host"),
		"The collected authenticator should be the one that was registered")
}

// HostAssigneeResolver, HostCCResolver, and HostInitiatorResolver are shaped
// like an application's own principal kinds: each names only the public
// contract, which is all a host in another module can reach. The kinds they
// describe need no designer input — the "resolved from the applicant at run
// time" shape a host reaches for when a role list cannot express the rule.
type HostAssigneeResolver struct{}

func (*HostAssigneeResolver) Describe() approval.KindDescriptor[approval.AssigneeKind] {
	return approval.KindDescriptor[approval.AssigneeKind]{
		Kind:      "head_nurse",
		Label:     "Head nurse",
		Selection: approval.SelectionNone,
	}
}

func (*HostAssigneeResolver) Resolve(context.Context, *approval.AssigneeResolveContext) ([]approval.ResolvedAssignee, error) {
	return nil, nil
}

type HostCCResolver struct{}

func (*HostCCResolver) Describe() approval.KindDescriptor[approval.CCKind] {
	return approval.KindDescriptor[approval.CCKind]{
		Kind:      "head_nurse",
		Label:     "Head nurse",
		Selection: approval.SelectionNone,
	}
}

func (*HostCCResolver) Resolve(context.Context, *approval.CCResolveContext) ([]string, error) {
	return nil, nil
}

type HostInitiatorResolver struct{}

func (*HostInitiatorResolver) Describe() approval.KindDescriptor[approval.InitiatorKind] {
	return approval.KindDescriptor[approval.InitiatorKind]{
		Kind:      "ward_supervisor",
		Label:     "Ward supervisor",
		Selection: approval.SelectionNone,
	}
}

func (*HostInitiatorResolver) Permits(context.Context, *approval.InitiatorResolveContext) (bool, error) {
	return false, nil
}

// TestProvideApprovalResolversReachTheirGroups pins the same property for the
// three principal vocabularies. A resolver that misses its group is not an
// error: the kind simply never becomes deployable and never appears in the
// designer, which reads as "the framework ignores my extension".
func TestProvideApprovalResolversReachTheirGroups(t *testing.T) {
	type collector struct {
		fx.In

		Assignees  []approval.AssigneeResolver  `group:"vef:approval:assignee_resolvers"`
		CCs        []approval.CCResolver        `group:"vef:approval:cc_resolvers"`
		Initiators []approval.InitiatorResolver `group:"vef:approval:initiator_resolvers"`
	}

	var collected collector

	fxApp := fx.New(
		ProvideApprovalAssigneeResolver(func() approval.AssigneeResolver { return new(HostAssigneeResolver) }),
		ProvideApprovalCCResolver(func() approval.CCResolver { return new(HostCCResolver) }),
		ProvideApprovalInitiatorResolver(func() approval.InitiatorResolver { return new(HostInitiatorResolver) }),
		fx.Invoke(func(c collector) { collected = c }),
		fx.NopLogger,
	)

	require.NoError(t, fxApp.Err(), "The resolver graph should resolve")

	require.Len(t, collected.Assignees, 1, "A host assignee resolver must reach the group the composite assembles from")
	require.Equal(t, approval.AssigneeKind("head_nurse"), collected.Assignees[0].Describe().Kind,
		"The collected assignee resolver should be the one that was registered")

	require.Len(t, collected.CCs, 1, "A host CC resolver must reach the group the composite assembles from")
	require.Equal(t, approval.CCKind("head_nurse"), collected.CCs[0].Describe().Kind,
		"The collected CC resolver should be the one that was registered")

	require.Len(t, collected.Initiators, 1, "A host initiator resolver must reach the group the composite assembles from")
	require.Equal(t, approval.InitiatorKind("ward_supervisor"), collected.Initiators[0].Describe().Kind,
		"The collected initiator resolver should be the one that was registered")
}
