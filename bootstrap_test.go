package vef

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/security"
)

// StubAssigneeService satisfies the approval module's organizational lookup
// contract. The wiring test only resolves the graph, so no method ever runs.
type StubAssigneeService struct{}

func (*StubAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, nil
}

func (*StubAssigneeService) GetDepartmentLeaders(context.Context, string) ([]approval.UserInfo, error) {
	return nil, nil
}

func (*StubAssigneeService) GetRoleUsers(context.Context, string) ([]approval.UserInfo, error) {
	return nil, nil
}

// StubUserInfoResolver satisfies approval.UserInfoResolver.
type StubUserInfoResolver struct{}

func (*StubUserInfoResolver) ResolveUsers(context.Context, []string) (map[string]approval.UserInfo, error) {
	return nil, nil
}

// StubPrincipalDepartmentResolver satisfies approval.PrincipalDepartmentResolver.
type StubPrincipalDepartmentResolver struct{}

func (*StubPrincipalDepartmentResolver) Resolve(
	context.Context,
	*security.Principal,
) (departmentID, departmentName *string, err error) {
	return nil, nil, nil
}

// StubInstanceNoGenerator satisfies approval.InstanceNoGenerator.
type StubInstanceNoGenerator struct{}

func (*StubInstanceNoGenerator) Generate(context.Context, string) (string, error) {
	return "", nil
}

// approvalHostDependencies supplies the interfaces the approval module leaves to
// the host application. Listing them here doubles as the executable statement of
// that contract: a framework change that makes the module demand a new host
// implementation fails this test, which is exactly the breaking change a host
// needs to hear about.
func approvalHostDependencies() fx.Option {
	return fx.Supply(
		fx.Annotate(new(StubAssigneeService), fx.As(new(approval.AssigneeService))),
		fx.Annotate(new(StubUserInfoResolver), fx.As(new(approval.UserInfoResolver))),
		fx.Annotate(new(StubPrincipalDepartmentResolver), fx.As(new(approval.PrincipalDepartmentResolver))),
		fx.Annotate(new(StubInstanceNoGenerator), fx.As(new(approval.InstanceNoGenerator))),
	)
}

// TestBootOptionsGraphResolves proves every graph Run can assemble — the core
// boot sequence alone, plus each optional feature module, plus both together —
// has a provider for every type its constructors and fx.Invoke targets ask for.
//
// fx resolves dependencies lazily, so a missing provider is neither a compile
// error nor a failure in any module's own tests: it surfaces only when an
// application actually boots that module, as a start-up abort. The instance this
// test was written for is a migration hook declaring `ctx context.Context` as a
// parameter — fx provides only Lifecycle, Shutdowner and DotGraph, never a
// context — which made both optional modules unbootable while their own test
// harnesses passed, because those harnesses provided a context.Context of their
// own and so repaired the graph the production boot could not.
//
// fx.ValidateApp builds and resolves the graph without invoking constructors or
// running lifecycle hooks, so this needs no config file and no database.
func TestBootOptionsGraphResolves(t *testing.T) {
	cases := []struct {
		name    string
		options []fx.Option
	}{
		{name: "Core"},
		{name: "Approval", options: []fx.Option{ApprovalModule, approvalHostDependencies()}},
		{name: "Integration", options: []fx.Option{IntegrationModule}},
		{
			name:    "AllModules",
			options: []fx.Option{ApprovalModule, IntegrationModule, approvalHostDependencies()},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fx.ValidateApp(bootOptions(tc.options...)...)
			require.NoError(t, err, "boot graph must resolve every dependency for %s", tc.name)
		})
	}
}

// HookOrderRecorder captures the order fx executes start hooks in, by function
// name, which is the only place that order is observable.
type HookOrderRecorder struct {
	names []string
}

func (r *HookOrderRecorder) LogEvent(e fxevent.Event) {
	if executing, ok := e.(*fxevent.OnStartExecuting); ok {
		r.names = append(r.names, executing.FunctionName)
	}
}

func (r *HookOrderRecorder) indexOf(substr string) int {
	return slices.IndexFunc(r.names, func(name string) bool {
		return strings.Contains(name, substr)
	})
}

// hostProvisioningHook stands in for host code that provisions state in a start
// hook, written the way a host most easily writes it: a bare fx.Invoke passed
// straight to vef.Run rather than wrapped in an fx.Module.
func hostProvisioningHook(lc fx.Lifecycle) {
	lc.Append(fx.StartHook(func() {}))
}

// TestSchedulerStartsAfterModuleAndHostStartHooks pins the ordering contract
// behind bootmodules.Assemble: the shared cron scheduler must not start until
// every module and every host option has finished provisioning, because jobs
// registered with cron.WithStartImmediately tick the moment it does.
//
// fx runs start hooks in append order, and a hook appended from a constructor
// lands wherever that constructor first resolves — so while the scheduler's
// start hook lived in newScheduler it ran at the first job registration, ahead
// of the migrations for the event outbox, the sequence store, and the approval
// schema. Both markers below are load-bearing. The outbox migration covers the
// module case. hostProvisioningHook covers the case a first attempt at this fix
// got wrong: fx runs every child-module invoke before any root-level one, so
// starting the scheduler from an fx.Module leaves it ahead of exactly this
// shape, and only an fx.Invoke in the trailing slot orders after it.
func TestSchedulerStartsAfterModuleAndHostStartHooks(t *testing.T) {
	recorder := new(HookOrderRecorder)

	_, stop := apptest.NewTestApp(t,
		fx.WithLogger(func() fxevent.Logger { return recorder }),
		fx.Invoke(hostProvisioningHook),
	)
	defer stop()

	scheduler := recorder.indexOf("internal/cron.StartScheduler")
	require.GreaterOrEqual(t, scheduler, 0,
		"the scheduler start hook must run during boot; recorded hooks: %v", recorder.names)

	for _, marker := range []string{"internal/event.runOutboxMigration", "hostProvisioningHook"} {
		index := recorder.indexOf(marker)
		require.GreaterOrEqual(t, index, 0,
			"%s must run during boot; recorded hooks: %v", marker, recorder.names)

		assert.Greater(t, scheduler, index,
			"the scheduler must start after %s; recorded hooks: %v", marker, recorder.names)
	}
}
