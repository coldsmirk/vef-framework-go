package query

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/page"
)

// TestRegisterHandlers verifies that every query type provided in the module is
// also registered with the bus. A handler that is provided via fx.Provide but
// omitted from registerHandlers would compile and wire cleanly yet leave the
// query silently unhandled at runtime (the resource layer dispatches through the
// bus, so the endpoint would fail with ErrHandlerNotFound while unit tests that
// call the handler directly still pass). This mirrors the command package's
// TestRegisterHandlers and is the regression guard for that drift.
func TestRegisterHandlers(t *testing.T) {
	bus := cqrs.NewBus(nil)

	// Zero-value handler instances — safe for registration; we never dispatch to
	// them, only verify that cqrs.Register was called for each query type. The
	// argument order matches registerHandlers' parameter list.
	registerHandlers(
		bus,
		new(GetFlowGraphHandler),
		new(FindMyInitiatedHandler),
		new(FindMyPendingTasksHandler),
		new(FindMyCompletedTasksHandler),
		new(FindMyCCRecordsHandler),
		new(GetMyPendingCountsHandler),
		new(GetMyInstanceDetailHandler),
		new(FindAvailableFlowsHandler),
		new(FindAdminInstancesHandler),
		new(FindAdminTasksHandler),
		new(GetAdminInstanceDetailHandler),
		new(FindAdminActionLogsHandler),
		new(FindFlowsHandler),
		new(FindFlowVersionsHandler),
		new(FindFlowInitiatorsHandler),
		new(GetMetricsHandler),
	)

	// For each expected query type, send a zero-value query and confirm the bus
	// did not return ErrHandlerNotFound. A nil-receiver panic means the handler
	// was registered but its deps are nil — that still proves registration.
	cases := []struct {
		name string
		send func() error
	}{
		{"GetFlowGraph", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[GetFlowGraphQuery, *shared.FlowGraph](context.Background(), bus, GetFlowGraphQuery{})

			return err
		}},
		{"FindMyInitiated", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindMyInitiatedQuery, *page.Page[my.InitiatedInstance]](context.Background(), bus, FindMyInitiatedQuery{})

			return err
		}},
		{"FindMyPendingTasks", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindMyPendingTasksQuery, *page.Page[my.PendingTask]](context.Background(), bus, FindMyPendingTasksQuery{})

			return err
		}},
		{"FindMyCompletedTasks", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindMyCompletedTasksQuery, *page.Page[my.CompletedTask]](context.Background(), bus, FindMyCompletedTasksQuery{})

			return err
		}},
		{"FindMyCCRecords", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindMyCCRecordsQuery, *page.Page[my.CCRecord]](context.Background(), bus, FindMyCCRecordsQuery{})

			return err
		}},
		{"GetMyPendingCounts", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[GetMyPendingCountsQuery, *my.PendingCounts](context.Background(), bus, GetMyPendingCountsQuery{})

			return err
		}},
		{"GetMyInstanceDetail", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[GetMyInstanceDetailQuery, *my.InstanceDetail](context.Background(), bus, GetMyInstanceDetailQuery{})

			return err
		}},
		{"FindAvailableFlows", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindAvailableFlowsQuery, *page.Page[my.AvailableFlow]](context.Background(), bus, FindAvailableFlowsQuery{})

			return err
		}},
		{"FindAdminInstances", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindAdminInstancesQuery, *page.Page[admin.Instance]](context.Background(), bus, FindAdminInstancesQuery{})

			return err
		}},
		{"FindAdminTasks", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindAdminTasksQuery, *page.Page[admin.Task]](context.Background(), bus, FindAdminTasksQuery{})

			return err
		}},
		{"GetAdminInstanceDetail", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[GetAdminInstanceDetailQuery, *admin.InstanceDetail](context.Background(), bus, GetAdminInstanceDetailQuery{})

			return err
		}},
		{"FindAdminActionLogs", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindAdminActionLogsQuery, *page.Page[admin.ActionLog]](context.Background(), bus, FindAdminActionLogsQuery{})

			return err
		}},
		{"FindFlows", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindFlowsQuery, *page.Page[approval.Flow]](context.Background(), bus, FindFlowsQuery{})

			return err
		}},
		{"FindFlowVersions", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindFlowVersionsQuery, []approval.FlowVersion](context.Background(), bus, FindFlowVersionsQuery{})

			return err
		}},
		{"FindFlowInitiators", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[FindFlowInitiatorsQuery, []approval.FlowInitiator](context.Background(), bus, FindFlowInitiatorsQuery{})

			return err
		}},
		{"GetMetrics", func() (err error) {
			defer recoverDispatch(&err)

			_, err = cqrs.Send[GetMetricsQuery, *admin.Metrics](context.Background(), bus, GetMetricsQuery{})

			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.send()
			assert.False(t, errors.Is(err, cqrs.ErrHandlerNotFound),
				"Handler for %s must be registered with the bus", tc.name)
		})
	}
}

// recoverDispatch converts a nil-receiver dispatch panic into a nil error so the
// registration check treats a panic as "handler was registered and found". Only
// cqrs.ErrHandlerNotFound means the handler is missing from the bus.
func recoverDispatch(errp *error) {
	//nolint:revive // recoverDispatch is only ever invoked via defer, so recover() catches the dispatch panic correctly.
	if recover() != nil {
		// Panic means the handler was found but its nil deps panicked on dispatch.
		// This proves registration succeeded.
		*errp = nil
	}
}
