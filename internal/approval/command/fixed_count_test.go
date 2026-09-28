package command_test

import (
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func (s *DesignerDefaultsTestSuite) TestFixedCountRule() {
	for _, tc := range []struct {
		name      string
		approvers []string
	}{
		{name: "ThreeOfFive", approvers: []string{"expert-1", "expert-2", "expert-3", "expert-4", "expert-5"}},
		{name: "AllWhenOnlyTwoAvailable", approvers: []string{"expert-1", "expert-2"}},
	} {
		s.Run(tc.name, func() {
			def := bareApprovalFlowDef(tc.approvers[0])
			def.Nodes[1].Data = mustMarshal(approval.ApprovalNodeData{
				Name: "Expert review",
				Assignees: []approval.AssigneeDefinition{
					{Kind: approval.AssigneeUser, IDs: tc.approvers, SortOrder: 1},
				},
				PassRule:  approval.PassFixedCount,
				PassCount: 3,
			})

			fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, tc.name, def)
			node := approval.FlowNode{ID: fixture.NodeIDs["approval-1"]}
			s.Require().NoError(s.db.NewSelect().Model(&node).WherePK().Scan(s.ctx), "Approval node should load")
			s.Assert().Equal(approval.PassFixedCount, node.PassRule, "Rule should be stored on the node")
			s.Assert().Equal(3, node.PassCount, "Fixed count should be stored on the node")

			instance := s.startInstance(fixture, tc.name)
			for i, approver := range tc.approvers[:min(3, len(tc.approvers))] {
				var task approval.Task
				s.Require().NoError(s.db.NewSelect().Model(&task).Where(func(cb orm.ConditionBuilder) {
					cb.Equals("instance_id", instance.ID).
						Equals("assignee_id", approver).
						Equals("status", approval.TaskPending)
				}).Scan(s.ctx), "Approver should have a pending task")

				_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
					TaskID:   task.ID,
					Operator: approval.UserInfo{ID: approver, Name: approver},
					Opinion:  "agree",
					Caller:   approval.SystemCaller,
				})
				s.Require().NoError(err, "Approval should succeed")

				updated := approval.Instance{ID: instance.ID}
				s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Instance should reload")

				if i+1 == min(3, len(tc.approvers)) {
					s.Assert().Equal(approval.InstanceApproved, updated.Status, "Threshold should complete the flow")
				} else {
					s.Assert().Equal(approval.InstanceRunning, updated.Status, "Flow should wait for the threshold")
				}
			}
		})
	}
}
