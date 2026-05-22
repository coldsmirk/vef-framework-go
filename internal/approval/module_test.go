package approval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// stubRouteInspector lets verifyEventRouting exercise OnStart against a
// deterministic routing table without spinning up the real bus.
type stubRouteInspector struct {
	transactional map[string]bool
}

func (s *stubRouteInspector) HasTransactionalRoute(et string) bool {
	return s.transactional[et]
}

func allRequiredTransactional() map[string]bool {
	return map[string]bool{
		approval.EventTypeInstanceCreated:     true,
		approval.EventTypeInstanceCompleted:   true,
		approval.EventTypeInstanceWithdrawn:   true,
		approval.EventTypeInstanceRolledBack:  true,
		approval.EventTypeInstanceReturned:    true,
		approval.EventTypeInstanceResubmitted: true,
		approval.EventTypeNodeEntered:         true,
		approval.EventTypeNodeAutoPassed:      true,
		approval.EventTypeTaskCreated:         true,
		approval.EventTypeTaskApproved:        true,
		approval.EventTypeTaskHandled:         true,
		approval.EventTypeTaskRejected:        true,
		approval.EventTypeTaskTransferred:     true,
		approval.EventTypeTaskReassigned:      true,
		approval.EventTypeTaskTimedOut:        true,
		approval.EventTypeAssigneesAdded:      true,
		approval.EventTypeAssigneesRemoved:    true,
		approval.EventTypeTaskDeadlineWarning: true,
		approval.EventTypeTaskUrged:           true,
		approval.EventTypeCCNotified:          true,
		approval.EventTypeFlowCreated:         true,
		approval.EventTypeFlowUpdated:         true,
		approval.EventTypeFlowDeployed:        true,
		approval.EventTypeFlowToggled:         true,
		approval.EventTypeFlowPublished:       true,
	}
}

func TestVerifyEventRouting(t *testing.T) {
	t.Run("PassesWhenAllRequiredEventsHaveTransactionalRoute", func(t *testing.T) {
		inspector := &stubRouteInspector{transactional: allRequiredTransactional()}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()), "所有必需事件都有事务路由时不应当报错")

		lc.RequireStop()
	})

	t.Run("FailsWhenTaskCreatedMissesTransactionalRoute", func(t *testing.T) {
		ts := allRequiredTransactional()
		delete(ts, approval.EventTypeTaskCreated)
		inspector := &stubRouteInspector{transactional: ts}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "缺事务路由时必须返回错误")
		assert.ErrorIs(t, err, ErrEventRouteNotTransactional, "应当为 ErrEventRouteNotTransactional")
		assert.Contains(t, err.Error(), approval.EventTypeTaskCreated, "错误信息应当点名缺失的事件类型")
		assert.Contains(t, err.Error(), "outbox", "错误信息应当指引配置 outbox")
	})

	t.Run("FailsOnFirstMissingEventInDeclaredOrder", func(t *testing.T) {
		// 完全空表 → 第一个 required 即 EventTypeInstanceCreated 应当被点名
		inspector := &stubRouteInspector{}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		err := lc.Start(context.Background())
		require.Error(t, err, "完全没有事务路由时必须返回错误")
		assert.ErrorIs(t, err, ErrEventRouteNotTransactional)
		assert.Contains(t, err.Error(), approval.EventTypeInstanceCreated,
			"应当报告首个未配置事务路由的事件类型")
	})

	t.Run("DoesNotRequireBindingFailedTxRoute", func(t *testing.T) {
		// binding_failed 由异步 listener 发出（无 WithTx），不应进入校验列表。
		// 即使 inspector 显式不为它提供事务路由，verifyEventRouting 也应放行。
		ts := allRequiredTransactional()
		// EventTypeInstanceBindingFailed 不在 ts 里 —— 这正是默认状态
		_, exists := ts[approval.EventTypeInstanceBindingFailed]
		require.False(t, exists, "binding_failed 不应在事务路由要求列表中")

		inspector := &stubRouteInspector{transactional: ts}

		lc := fxtest.NewLifecycle(t)
		verifyEventRouting(lc, inspector)

		require.NoError(t, lc.Start(context.Background()),
			"binding_failed 缺事务路由时不应当让模块启动失败")

		lc.RequireStop()
	})
}
