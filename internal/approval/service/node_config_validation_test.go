package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/decimal"
)

// condGroups returns a minimal valid condition payload for a non-default
// branch, letting fixtures target other validation rules.
func condGroups() []approval.ConditionGroup {
	return []approval.ConditionGroup{{Conditions: []approval.Condition{
		{Kind: approval.ConditionField, Subject: "amount", Operator: approval.OperatorEquals, Value: 1},
	}}}
}

func TestValidateNodeConfig(t *testing.T) {
	t.Run("PassRatio", func(t *testing.T) {
		tests := []struct {
			name    string
			ratio   float64
			wantErr error
		}{
			{"RejectsZero", 0, errPassRatioOutOfRange},
			{"RejectsNegative", -5, errPassRatioOutOfRange},
			{"RejectsAboveHundred", 100.5, errPassRatioOutOfRange},
			{"AcceptsFractionalPercent", 0.5, nil},
			{"AcceptsFifty", 50, nil},
			{"AcceptsHundred", 100, nil},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data := &approval.ApprovalNodeData{
					PassRule:  approval.PassRatio,
					PassRatio: decimal.NewFromFloat(tt.ratio),
				}

				err := validateNodeConfig("n1", data)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr, "Ratio %v should be rejected as out of the (0, 100] percentage range", tt.ratio)
				} else {
					assert.NoError(t, err, "Ratio %v should be a valid percentage", tt.ratio)
				}
			})
		}

		t.Run("IgnoredForOtherRules", func(t *testing.T) {
			data := &approval.ApprovalNodeData{PassRule: approval.PassAll}
			assert.NoError(t, validateNodeConfig("n1", data), "Pass ratio should not be required outside the ratio rule")
		})
	})

	t.Run("BranchPriorities", func(t *testing.T) {
		t.Run("RejectsDuplicateAmongNonDefault", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: condGroups()},
					{ID: "b2", Priority: 1, ConditionGroups: condGroups()},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errDuplicateBranchPriority,
				"Two non-default branches sharing a priority should be rejected")
		})

		t.Run("AcceptsUniquePriorities", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: condGroups()},
					{ID: "b2", Priority: 2, ConditionGroups: condGroups()},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data), "Unique branch priorities should pass")
		})

		t.Run("DefaultBranchDoesNotParticipate", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: condGroups()},
					{ID: "bd", Priority: 1, IsDefault: true},
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"A default branch sharing a priority with a non-default one should pass — defaults are not ordered")
		})
	})

	t.Run("BranchConditions", func(t *testing.T) {
		t.Run("RejectsNonDefaultBranchWithoutGroups", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errBranchConditionsRequired,
				"A non-default branch with no condition groups would match unconditionally and must be rejected")
		})

		t.Run("RejectsEmptyConditionGroup", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: []approval.ConditionGroup{{}}},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errConditionGroupEmpty,
				"An empty condition group evaluates to true and must be rejected")
		})

		t.Run("AcceptsDefaultBranchWithoutGroups", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: condGroups()},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"The default branch is the fallback and legitimately carries no conditions")
		})
	})

	t.Run("AddAssigneeTypes", func(t *testing.T) {
		t.Run("RejectsUnknownType", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				AddAssigneeTypes: []approval.AddAssigneeType{"sideways"},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errInvalidAddAssigneeType,
				"Out-of-enum add-assignee types should fail at deploy")
		})

		t.Run("RejectsParallelOnSequential", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ApprovalMethod:   approval.ApprovalSequential,
				AddAssigneeTypes: []approval.AddAssigneeType{approval.AddAssigneeParallel},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errSequentialParallelAdd,
				"A sequential queue has no parallel lane to join")
		})

		t.Run("AcceptsParallelOnParallel", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ApprovalMethod:   approval.ApprovalParallel,
				AddAssigneeTypes: []approval.AddAssigneeType{approval.AddAssigneeParallel},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"Parallel additions are the normal case on parallel nodes")
		})

		t.Run("AcceptsBeforeAfterOnSequential", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ApprovalMethod:   approval.ApprovalSequential,
				AddAssigneeTypes: []approval.AddAssigneeType{approval.AddAssigneeBefore, approval.AddAssigneeAfter},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"Before/after splice into the sequential queue and stay valid")
		})
	})

	t.Run("HandleRestrictions", func(t *testing.T) {
		t.Run("RejectsAutoRejectExecution", func(t *testing.T) {
			data := &approval.HandleNodeData{
				TaskNodeData: approval.TaskNodeData{ExecutionType: approval.ExecutionAutoReject},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errHandleExecutionAutoReject,
				"Handle nodes must not be able to reject the whole instance via execution type")
		})

		t.Run("RejectsAutoRejectTimeout", func(t *testing.T) {
			data := &approval.HandleNodeData{
				TaskNodeData: approval.TaskNodeData{TimeoutAction: approval.TimeoutActionAutoReject},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errHandleTimeoutAutoReject,
				"Handle nodes must not be able to reject the whole instance via timeout action")
		})

		t.Run("AcceptsAutoPass", func(t *testing.T) {
			data := &approval.HandleNodeData{
				TaskNodeData: approval.TaskNodeData{
					ExecutionType: approval.ExecutionAutoPass,
					TimeoutAction: approval.TimeoutActionAutoPass,
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"Auto-pass execution and timeout remain valid for handle nodes")
		})

		t.Run("ApprovalNodeKeepsAutoReject", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				TaskNodeData: approval.TaskNodeData{
					ExecutionType: approval.ExecutionAutoReject,
					TimeoutAction: approval.TimeoutActionAutoReject,
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"Approval nodes are decision points and keep the auto-reject options")
		})
	})
}
