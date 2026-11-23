package service

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// ValidateConditionFields cross-checks every field condition in the parsed node
// data against the version's parsed form fields:
//
//   - Aggregate conditions: the aggregate kind must be registered, the subject
//     must name a table field, and column-folding aggregates must name a numeric
//     column in that table.
//   - Scalar conditions: when the subject resolves to a form field (skipping
//     built-in applicant subjects and subjects absent from form fields, such as
//     host-defined globals), its operator must be valid for that field's kind
//     and multiplicity (approval.IsOperatorValidForField).
//
// The structural per-condition rules (closed operator vocabulary, aggregate
// operator/column shape) live in validateCondition; this pass adds what only
// the form fields and the boot-registered aggregator set can answer, so it runs
// at deploy — the one place they all meet.
func (s *FlowDefinitionService) ValidateConditionFields(nodeData map[string]approval.NodeData, fields []approval.FormFieldDefinition) error {
	fieldsByKey := formFieldIndex(fields)
	tables := tableFieldIndex(fields)

	for nodeID, data := range nodeData {
		condData, ok := data.(*approval.ConditionNodeData)
		if !ok {
			continue
		}

		for _, branch := range condData.Branches {
			for _, group := range branch.ConditionGroups {
				for _, cond := range group.Conditions {
					if cond.Kind != approval.ConditionField {
						continue
					}

					if cond.Aggregate != "" {
						if err := s.validateAggregateReference(nodeID, branch.ID, cond, tables); err != nil {
							return err
						}

						continue
					}

					if err := validateScalarConditionOperator(nodeID, branch.ID, cond, fieldsByKey); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// validateScalarConditionOperator verifies that a scalar field condition whose
// subject is a form field uses an operator compatible with that field's kind
// and multiplicity. Built-in applicant subjects and subjects absent from the
// form (host-supplied runtime globals) are not form fields and are skipped.
func validateScalarConditionOperator(
	nodeID, branchID string,
	cond approval.Condition,
	fieldsByKey map[string]approval.FormFieldDefinition,
) error {
	if cond.Subject == approval.SubjectApplicantID || cond.Subject == approval.SubjectApplicantDepartmentID {
		return nil
	}

	field, ok := fieldsByKey[cond.Subject]
	if !ok {
		return nil
	}

	if !approval.IsOperatorValidForField(field, cond.Operator) {
		return fmt.Errorf("%w: operator %q is not valid for field %q in branch %q of node %q",
			errOperatorNotForField, cond.Operator, cond.Subject, branchID, nodeID)
	}

	return nil
}

// validateAggregateReference resolves one aggregate condition against the
// table-field index built from the form schema.
func (s *FlowDefinitionService) validateAggregateReference(
	nodeID, branchID string,
	cond approval.Condition,
	tables map[string]map[string]approval.FormFieldDefinition,
) error {
	if !s.aggregateKinds.Contains(cond.Aggregate) {
		return fmt.Errorf("%w: %q in branch %q of node %q",
			errUnregisteredAggregate, cond.Aggregate, branchID, nodeID)
	}

	columns, ok := tables[cond.Subject]
	if !ok {
		return fmt.Errorf("%w: subject %q in branch %q of node %q",
			errAggregateSubjectNotTable, cond.Subject, branchID, nodeID)
	}

	if !cond.Aggregate.FoldsColumn() {
		return nil
	}

	column, ok := columns[cond.Column]
	if !ok {
		return fmt.Errorf("%w: column %q of table %q in branch %q of node %q",
			errAggregateColumnUnknown, cond.Column, cond.Subject, branchID, nodeID)
	}

	if column.Kind != approval.FieldNumber {
		return fmt.Errorf("%w: column %q of table %q is %q in branch %q of node %q",
			errAggregateColumnNotNumeric, cond.Column, cond.Subject, column.Kind, branchID, nodeID)
	}

	return nil
}

// formFieldIndex maps each top-level form field's key to its definition.
func formFieldIndex(fields []approval.FormFieldDefinition) map[string]approval.FormFieldDefinition {
	indexed := make(map[string]approval.FormFieldDefinition, len(fields))
	for _, field := range fields {
		indexed[field.Key] = field
	}

	return indexed
}

// tableFieldIndex maps each table field's key to its columns by key.
func tableFieldIndex(fields []approval.FormFieldDefinition) map[string]map[string]approval.FormFieldDefinition {
	tables := make(map[string]map[string]approval.FormFieldDefinition)

	for _, field := range fields {
		if field.Kind != approval.FieldTable {
			continue
		}

		columns := make(map[string]approval.FormFieldDefinition, len(field.Columns))
		for _, column := range field.Columns {
			columns[column.Key] = column
		}

		tables[field.Key] = columns
	}

	return tables
}
