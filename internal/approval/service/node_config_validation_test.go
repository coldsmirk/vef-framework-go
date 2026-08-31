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

				err := NewFlowDefinitionService().validateNodeConfig("n1", data)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr, "Ratio %v should be rejected as out of the (0, 100] percentage range", tt.ratio)
				} else {
					assert.NoError(t, err, "Ratio %v should be a valid percentage", tt.ratio)
				}
			})
		}

		t.Run("IgnoredForOtherRules", func(t *testing.T) {
			data := &approval.ApprovalNodeData{PassRule: approval.PassAll}
			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data), "Pass ratio should not be required outside the ratio rule")
		})
	})

	t.Run("SameApplicantAction", func(t *testing.T) {
		for _, action := range []approval.SameApplicantAction{
			approval.SameApplicantAutoPass,
			approval.SameApplicantSelfApprove,
			approval.SameApplicantTransferSuperior,
			approval.SameApplicantExclude,
		} {
			data := &approval.ApprovalNodeData{PassRule: approval.PassAll, SameApplicantAction: action}
			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data), "Action %q should be a valid same-applicant action", action)
		}

		data := &approval.ApprovalNodeData{PassRule: approval.PassAll, SameApplicantAction: "recuse"}
		assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errInvalidSameApplicantAction,
			"An out-of-enum same-applicant action should be rejected")
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

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errDuplicateBranchPriority,
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

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data), "Unique branch priorities should pass")
		})

		t.Run("DefaultBranchDoesNotParticipate", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: condGroups()},
					{ID: "bd", Priority: 1, IsDefault: true},
				},
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
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

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errBranchConditionsRequired,
				"A non-default branch with no condition groups would match unconditionally and must be rejected")
		})

		t.Run("RejectsEmptyConditionGroup", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: []approval.ConditionGroup{{}}},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errConditionGroupEmpty,
				"An empty condition group evaluates to true and must be rejected")
		})

		t.Run("AcceptsDefaultBranchWithoutGroups", func(t *testing.T) {
			data := &approval.ConditionNodeData{
				Branches: []approval.ConditionBranch{
					{ID: "b1", Priority: 1, ConditionGroups: condGroups()},
					{ID: "bd", Priority: 99, IsDefault: true},
				},
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
				"The default branch is the fallback and legitimately carries no conditions")
		})
	})

	t.Run("AddAssigneeTypes", func(t *testing.T) {
		t.Run("RejectsUnknownType", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				AddAssigneeTypes: []approval.AddAssigneeType{"sideways"},
			}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errInvalidAddAssigneeType,
				"Out-of-enum add-assignee types should fail at deploy")
		})

		t.Run("RejectsParallelOnSequential", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ApprovalMethod:   approval.ApprovalSequential,
				AddAssigneeTypes: []approval.AddAssigneeType{approval.AddAssigneeParallel},
			}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errSequentialParallelAdd,
				"A sequential queue has no parallel lane to join")
		})

		t.Run("AcceptsParallelOnParallel", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ApprovalMethod:   approval.ApprovalParallel,
				AddAssigneeTypes: []approval.AddAssigneeType{approval.AddAssigneeParallel},
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
				"Parallel additions are the normal case on parallel nodes")
		})

		t.Run("AcceptsBeforeAfterOnSequential", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ApprovalMethod:   approval.ApprovalSequential,
				AddAssigneeTypes: []approval.AddAssigneeType{approval.AddAssigneeBefore, approval.AddAssigneeAfter},
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
				"Before/after splice into the sequential queue and stay valid")
		})
	})

	// The assignee and CC vocabularies are open registries: deploy accepts
	// exactly the kinds with a registered resolver and enforces the input each
	// kind's descriptor declares, so a host kind is validated like a built-in.
	t.Run("AssigneeDefinitions", func(t *testing.T) {
		t.Run("RejectsUnregisteredKind", func(t *testing.T) {
			data := &approval.ApprovalNodeData{}
			data.Assignees = []approval.AssigneeDefinition{{Kind: "expert_panel", IDs: []string{"p1"}}}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errInvalidAssigneeKind,
				"A kind no resolver is registered for could never be executed")
		})

		t.Run("RejectsSelectingKindWithoutIDs", func(t *testing.T) {
			data := &approval.ApprovalNodeData{}
			data.Assignees = []approval.AssigneeDefinition{{Kind: approval.AssigneeUser}}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errAssigneeIDsRequired,
				"A rule that selects nobody resolves to nobody and must be rejected at deploy")
		})

		t.Run("RejectsFormFieldWithoutFieldName", func(t *testing.T) {
			data := &approval.ApprovalNodeData{}
			data.Assignees = []approval.AssigneeDefinition{{Kind: approval.AssigneeFormField}}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errAssigneeFormFieldRequired,
				"A form-field rule must name the field it reads")
		})

		t.Run("AcceptsParameterlessKinds", func(t *testing.T) {
			data := &approval.ApprovalNodeData{}
			data.Assignees = []approval.AssigneeDefinition{
				{Kind: approval.AssigneeSelf},
				{Kind: approval.AssigneeSuperior},
				{Kind: approval.AssigneeDepartmentLeader},
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
				"A kind resolved from the applicant needs no designer input")
		})

		t.Run("AcceptsHostKind", func(t *testing.T) {
			svc := NewFlowDefinitionService(WithAssigneeKinds(
				approval.KindDescriptor[approval.AssigneeKind]{Kind: "head_nurse", Label: "Head nurse", Selection: approval.SelectionNone},
			))
			data := &approval.ApprovalNodeData{}
			data.Assignees = []approval.AssigneeDefinition{{Kind: "head_nurse"}}

			assert.NoError(t, svc.validateNodeConfig("n1", data),
				"A registered host kind must deploy without a framework change")
		})
	})

	t.Run("CCDefinitions", func(t *testing.T) {
		t.Run("RejectsUnregisteredKind", func(t *testing.T) {
			data := &approval.CCNodeData{CCs: []approval.CCDefinition{{Kind: "expert_panel", IDs: []string{"p1"}}}}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errInvalidCCKind,
				"A CC kind no resolver is registered for could never be executed")
		})

		t.Run("RejectsSelectingKindWithoutIDs", func(t *testing.T) {
			data := &approval.CCNodeData{CCs: []approval.CCDefinition{{Kind: approval.CCRole}}}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errCCIDsRequired,
				"A CC rule that selects nobody notifies nobody")
		})

		t.Run("RejectsInvalidTiming", func(t *testing.T) {
			data := &approval.CCNodeData{CCs: []approval.CCDefinition{
				{Kind: approval.CCUser, IDs: []string{"u1"}, Timing: "whenever"},
			}}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errInvalidCCTiming,
				"Timing stays a closed enum: it is the engine's schedule, not a resolver's concern")
		})

		t.Run("AcceptsHostKind", func(t *testing.T) {
			svc := NewFlowDefinitionService(WithCCKinds(
				approval.KindDescriptor[approval.CCKind]{Kind: "head_nurse", Label: "Head nurse", Selection: approval.SelectionNone},
			))
			data := &approval.CCNodeData{CCs: []approval.CCDefinition{{Kind: "head_nurse"}}}

			assert.NoError(t, svc.validateNodeConfig("n1", data),
				"A registered host CC kind must deploy without a framework change")
		})
	})

	t.Run("AggregateShape", func(t *testing.T) {
		aggBranches := func(cond approval.Condition) *approval.ConditionNodeData {
			return &approval.ConditionNodeData{Branches: []approval.ConditionBranch{
				{ID: "b1", Priority: 1, ConditionGroups: []approval.ConditionGroup{{Conditions: []approval.Condition{cond}}}},
				{ID: "bd", Priority: 99, IsDefault: true},
			}}
		}

		t.Run("AcceptsUnknownKindStructurally", func(t *testing.T) {
			// Kind registration is a service-level concern checked by
			// ValidateConditionAggregates against the boot-registered set — a
			// closed gate here would break host-supplied aggregates.
			cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: "median", Column: "qty", Operator: approval.OperatorGreater, Value: 1}
			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", aggBranches(cond)),
				"structural validation must stay open to host-registered aggregate kinds")
		})

		t.Run("RejectsNonNumericOperator", func(t *testing.T) {
			cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateSum, Column: "qty", Operator: approval.OperatorContains, Value: 1}
			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", aggBranches(cond)), errAggregateOperator,
				"set/text operators are meaningless over a numeric fold")
		})

		t.Run("RejectsCountWithColumn", func(t *testing.T) {
			cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateCount, Column: "qty", Operator: approval.OperatorEquals, Value: 1}
			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", aggBranches(cond)), errAggregateColumnForbidden,
				"count folds rows; a column would be silently ignored")
		})

		t.Run("RejectsSumWithoutColumn", func(t *testing.T) {
			cond := approval.Condition{Kind: approval.ConditionField, Subject: "items", Aggregate: approval.AggregateSum, Operator: approval.OperatorEquals, Value: 1}
			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", aggBranches(cond)), errAggregateColumnRequired,
				"sum/avg fold a column and must name one")
		})

		t.Run("RejectsAggregateOnExpression", func(t *testing.T) {
			cond := approval.Condition{Kind: approval.ConditionExpression, Expression: "true", Aggregate: approval.AggregateSum}
			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", aggBranches(cond)), errAggregateOnExpression,
				"expression conditions fold inside the expression, not via the aggregate field")
		})
	})

	t.Run("HandleRestrictions", func(t *testing.T) {
		t.Run("RejectsAutoRejectExecution", func(t *testing.T) {
			data := &approval.HandleNodeData{
				ExecutionType: approval.ExecutionAutoReject,
			}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errHandleExecutionAutoReject,
				"Handle nodes must not be able to reject the whole instance via execution type")
		})

		t.Run("RejectsAutoRejectTimeout", func(t *testing.T) {
			data := &approval.HandleNodeData{
				TimeoutAction: approval.TimeoutActionAutoReject,
			}

			assert.ErrorIs(t, NewFlowDefinitionService().validateNodeConfig("n1", data), errHandleTimeoutAutoReject,
				"Handle nodes must not be able to reject the whole instance via timeout action")
		})

		t.Run("AcceptsAutoPass", func(t *testing.T) {
			data := &approval.HandleNodeData{
				ExecutionType: approval.ExecutionAutoPass,
				TimeoutAction: approval.TimeoutActionAutoPass,
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
				"Auto-pass execution and timeout remain valid for handle nodes")
		})

		t.Run("ApprovalNodeKeepsAutoReject", func(t *testing.T) {
			data := &approval.ApprovalNodeData{
				ExecutionType: approval.ExecutionAutoReject,
				TimeoutAction: approval.TimeoutActionAutoReject,
			}

			assert.NoError(t, NewFlowDefinitionService().validateNodeConfig("n1", data),
				"Approval nodes are decision points and keep the auto-reject options")
		})
	})
}
