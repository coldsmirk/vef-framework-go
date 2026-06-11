package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/decimal"
)

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
					{ID: "b1", Priority: 1},
					{ID: "b2", Priority: 1},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.ErrorIs(t, validateNodeConfig("n1", data), errDuplicateBranchPriority,
				"Two non-default branches sharing a priority should be rejected")
		})

		t.Run("AcceptsUniquePriorities", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1},
					{ID: "b2", Priority: 2},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data), "Unique branch priorities should pass")
		})

		t.Run("DefaultBranchDoesNotParticipate", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1},
					{ID: "bd", Priority: 1, IsDefault: true},
				},
			}

			assert.NoError(t, validateNodeConfig("n1", data),
				"A default branch sharing a priority with a non-default one should pass — defaults are not ordered")
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
