package service

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// ValidateConditionAggregates cross-checks every aggregate field condition in
// the parsed node data against the version's parsed form fields: the subject
// must name a table field and, for column-folding aggregates, the column must
// exist in that table and be a number field. The structural per-condition
// rules (known aggregate, operator whitelist, column presence contract) live
// in validateCondition; this pass adds what only the form fields and the
// boot-registered aggregator set can answer, so it runs at deploy — the one
// place they all meet.
//
// The column contract is derived from AggregateKind.FoldsColumn, so a new
// aggregate kind extends this validation without modifying it.
func (s *FlowDefinitionService) ValidateConditionAggregates(nodeData map[string]approval.NodeData, fields []approval.FormFieldDefinition) error {
	tables := tableFieldIndex(fields)

	for nodeID, data := range nodeData {
		condData, ok := data.(*approval.ConditionNodeData)
		if !ok {
			continue
		}

		for _, branch := range condData.Branches {
			for _, group := range branch.ConditionGroups {
				for _, cond := range group.Conditions {
					if cond.Kind != approval.ConditionField || cond.Aggregate == "" {
						continue
					}

					if err := s.validateAggregateReference(nodeID, branch.ID, cond, tables); err != nil {
						return err
					}
				}
			}
		}
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
	if _, registered := s.aggregateKinds[cond.Aggregate]; !registered {
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
