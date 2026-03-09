package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// DeployFlowCmd deploys a flow definition to an existing flow.
type DeployFlowCmd struct {
	cqrs.BaseCommand

	FlowID         string
	Description    *string
	FlowDefinition approval.FlowDefinition
	FormDefinition *approval.FormDefinition
}

// AssigneeProvider is the interface for accessing assignees from typed node data.
type AssigneeProvider interface {
	// GetAssignees returns the assignee definitions configured on this node.
	GetAssignees() []approval.AssigneeDefinition
}

// CCProvider is the interface for accessing CC list from typed node data.
type CCProvider interface {
	// GetCCs returns the CC recipient definitions configured on this node.
	GetCCs() []approval.CCDefinition
}

// DeployFlowHandler handles the DeployFlowCmd command.
type DeployFlowHandler struct {
	db         orm.DB
	flowDefSvc *service.FlowDefinitionService
}

// NewDeployFlowHandler creates a new DeployFlowHandler.
func NewDeployFlowHandler(db orm.DB, flowDefSvc *service.FlowDefinitionService) *DeployFlowHandler {
	return &DeployFlowHandler{db: db, flowDefSvc: flowDefSvc}
}

func (h *DeployFlowHandler) Handle(ctx context.Context, cmd DeployFlowCmd) (*approval.FlowVersion, error) {
	if err := h.flowDefSvc.ValidateFlowDefinition(&cmd.FlowDefinition); err != nil {
		return nil, fmt.Errorf("%w: %w", shared.ErrInvalidFlowDesign, err)
	}

	db := contextx.DB(ctx, h.db)

	var flow approval.Flow

	flow.ID = cmd.FlowID
	if err := db.NewSelect().
		Model(&flow).
		Select("current_version").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrFlowNotFound
		}

		return nil, fmt.Errorf("load flow: %w", err)
	}

	version := approval.FlowVersion{
		FlowID:      flow.ID,
		Version:     flow.CurrentVersion + 1,
		Status:      approval.VersionDraft,
		Description: cmd.Description,
		FlowSchema:  &cmd.FlowDefinition,
		FormSchema:  cmd.FormDefinition,
	}
	if _, err := db.NewInsert().
		Model(&version).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Phase 1: Parse all node data and build node models (fail fast on parse errors)
	type parsedNode struct {
		node approval.FlowNode
		data approval.NodeData
	}

	parsedNodes := make([]parsedNode, 0, len(cmd.FlowDefinition.Nodes))
	for _, nodeDef := range cmd.FlowDefinition.Nodes {
		nodeData, err := nodeDef.ParseData()
		if err != nil {
			return nil, fmt.Errorf("parse node %q data: %w", nodeDef.ID, err)
		}

		node := approval.FlowNode{
			FlowVersionID: version.ID,
			Key:           nodeDef.ID,
			Kind:          nodeDef.Kind,
		}
		nodeData.ApplyTo(&node)

		parsedNodes = append(parsedNodes, parsedNode{node: node, data: nodeData})
	}

	// Phase 2: Batch insert nodes and build nodeKey -> nodeID mapping
	nodes := make([]approval.FlowNode, len(parsedNodes))
	for i := range parsedNodes {
		nodes[i] = parsedNodes[i].node
	}

	if len(nodes) > 0 {
		if _, err := db.NewInsert().Model(&nodes).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert nodes: %w", err)
		}
	}

	nodeKeyToID := make(map[string]string, len(nodes))
	for i := range nodes {
		nodeKeyToID[nodes[i].Key] = nodes[i].ID
	}

	// Phase 3: Collect and batch insert assignees and CCs
	var (
		allAssignees []approval.FlowNodeAssignee
		allCCs       []approval.FlowNodeCC
	)

	for i, pn := range parsedNodes {
		nodeID := nodes[i].ID

		if ap, ok := pn.data.(AssigneeProvider); ok {
			for _, assigneeDef := range ap.GetAssignees() {
				allAssignees = append(allAssignees, approval.FlowNodeAssignee{
					NodeID:    nodeID,
					Kind:      assigneeDef.Kind,
					IDs:       assigneeDef.IDs,
					FormField: assigneeDef.FormField,
					SortOrder: assigneeDef.SortOrder,
				})
			}
		}

		if cp, ok := pn.data.(CCProvider); ok {
			for _, ccDef := range cp.GetCCs() {
				allCCs = append(allCCs, approval.FlowNodeCC{
					NodeID:    nodeID,
					Kind:      ccDef.Kind,
					IDs:       ccDef.IDs,
					FormField: ccDef.FormField,
					Timing:    ccDef.Timing,
				})
			}
		}
	}

	if len(allAssignees) > 0 {
		if _, err := db.NewInsert().Model(&allAssignees).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert node assignees: %w", err)
		}
	}

	if len(allCCs) > 0 {
		if _, err := db.NewInsert().Model(&allCCs).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert node ccs: %w", err)
		}
	}

	// Phase 4: Validate and batch insert edges
	edges := make([]approval.FlowEdge, 0, len(cmd.FlowDefinition.Edges))
	for _, edgeDef := range cmd.FlowDefinition.Edges {
		sourceID, ok := nodeKeyToID[edgeDef.Source]
		if !ok {
			return nil, fmt.Errorf("%w: unknown source node key %q", shared.ErrInvalidFlowDesign, edgeDef.Source)
		}

		targetID, ok := nodeKeyToID[edgeDef.Target]
		if !ok {
			return nil, fmt.Errorf("%w: unknown target node key %q", shared.ErrInvalidFlowDesign, edgeDef.Target)
		}

		edges = append(edges, approval.FlowEdge{
			FlowVersionID: version.ID,
			Key:           edgeDef.ID,
			SourceNodeID:  sourceID,
			SourceNodeKey: edgeDef.Source,
			TargetNodeID:  targetID,
			TargetNodeKey: edgeDef.Target,
			SourceHandle:  edgeDef.SourceHandle,
		})
	}

	if len(edges) > 0 {
		if _, err := db.NewInsert().Model(&edges).Exec(ctx); err != nil {
			return nil, fmt.Errorf("insert edges: %w", err)
		}
	}

	return &version, nil
}
