package command

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/approval/storage"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// DeployFlowCmd deploys a flow definition to an existing flow. FormSchema is
// the host-owned form designer document, persisted verbatim; the handler's
// FormSchemaParser derives the flat field list the framework consumes.
type DeployFlowCmd struct {
	cqrs.BaseCommand

	FlowID         string
	Description    *string
	StorageMode    approval.StorageMode
	FlowDefinition approval.FlowDefinition
	FormSchema     json.RawMessage
	Caller         approval.CallerContext
}

// assigneeProvider is the interface for accessing assignees from typed node data.
type assigneeProvider interface {
	GetAssignees() []approval.AssigneeDefinition
}

// ccProvider is the interface for accessing CC list from typed node data.
type ccProvider interface {
	GetCCs() []approval.CCDefinition
}

// DeployFlowHandler handles the DeployFlowCmd command.
type DeployFlowHandler struct {
	db         orm.DB
	flowDefSvc *service.FlowDefinitionService
	formParser approval.FormSchemaParser
}

// NewDeployFlowHandler creates a new DeployFlowHandler.
func NewDeployFlowHandler(db orm.DB, flowDefSvc *service.FlowDefinitionService, formParser approval.FormSchemaParser) *DeployFlowHandler {
	return &DeployFlowHandler{db: db, flowDefSvc: flowDefSvc, formParser: formParser}
}

func (h *DeployFlowHandler) Handle(ctx context.Context, cmd DeployFlowCmd) (*approval.FlowVersion, error) {
	parsedNodeData, err := h.flowDefSvc.ValidateFlowDefinition(&cmd.FlowDefinition)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", shared.ErrInvalidFlowDesign, err)
	}

	// Derive the flat field list from the host-owned designer document; the
	// schema itself stays opaque and is persisted verbatim below. The plain
	// context wrap keeps the parser's outward result.Error first in the chain,
	// so the API caller sees its specific message (the built-in parser's
	// errors already carry shared.ErrCodeInvalidFormDesign).
	fields, err := h.formParser.ParseFormFields(cmd.FormSchema)
	if err != nil {
		return nil, fmt.Errorf("parse form schema: %w", err)
	}

	if err := h.flowDefSvc.ValidateFormFields(fields); err != nil {
		return nil, fmt.Errorf("%w: %w", shared.ErrInvalidFormDesign, err)
	}

	// Aggregate conditions reference form fields, so they can only be fully
	// validated where the flow definition and form fields meet.
	if err := h.flowDefSvc.ValidateConditionAggregates(parsedNodeData, fields); err != nil {
		return nil, fmt.Errorf("%w: %w", shared.ErrInvalidFlowDesign, err)
	}

	// Field permissions reference form fields the same way aggregate
	// conditions do, so they are validated at the same point.
	if err := h.flowDefSvc.ValidateFieldPermissions(parsedNodeData, fields); err != nil {
		return nil, fmt.Errorf("%w: %w", shared.ErrInvalidFlowDesign, err)
	}

	// An omitted storage mode resolves to the JSON default (the displayed
	// designer default); any other unrecognized value is a client error.
	storageMode := cmp.Or(cmd.StorageMode, approval.StorageJSON)
	if !storageMode.IsValid() {
		return nil, shared.ErrInvalidStorageMode
	}

	// In table mode every form field key becomes a physical column, so reject a
	// schema whose keys cannot map to safe, unique, non-reserved identifiers now
	// — surfaced as a form-design error the admin sees on save — instead of
	// deferring the failure to publish, where it would surface opaquely.
	if storageMode == approval.StorageTable {
		if err := storage.ValidateTableFormSchema(fields); err != nil {
			return nil, fmt.Errorf("%w: %w", shared.ErrInvalidFormDesign, err)
		}
	}

	db := contextx.DB(ctx, h.db)

	var flow approval.Flow

	flow.ID = cmd.FlowID
	if err := db.NewSelect().
		Model(&flow).
		Select("current_version", "tenant_id", "code", "name").
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrFlowNotFound
		}

		return nil, fmt.Errorf("load flow: %w", err)
	}

	if err := cmd.Caller.Authorize(flow.TenantID); err != nil {
		return nil, shared.ErrFlowNotFound
	}

	version := approval.FlowVersion{
		FlowID:      flow.ID,
		Version:     flow.CurrentVersion + 1,
		Status:      approval.VersionDraft,
		Description: cmd.Description,
		StorageMode: storageMode,
		FlowSchema:  &cmd.FlowDefinition,
		FormSchema:  cmd.FormSchema,
		FormFields:  fields,
	}
	if _, err := db.NewInsert().
		Model(&version).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Phase 1: Build node models from the data already parsed (and approved)
	// by ValidateFlowDefinition — the persisted configuration is exactly what
	// passed validation, by construction.
	type parsedNode struct {
		node approval.FlowNode
		data approval.NodeData
	}

	parsedNodes := make([]parsedNode, 0, len(cmd.FlowDefinition.Nodes))
	for _, nodeDef := range cmd.FlowDefinition.Nodes {
		nodeData := parsedNodeData[nodeDef.ID]

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

		if ap, ok := pn.data.(assigneeProvider); ok {
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

		if cp, ok := pn.data.(ccProvider); ok {
			for _, ccDef := range cp.GetCCs() {
				allCCs = append(allCCs, approval.FlowNodeCC{
					NodeID:    nodeID,
					Kind:      ccDef.Kind,
					IDs:       ccDef.IDs,
					FormField: ccDef.FormField,
					// An omitted timing resolves to "always" at deploy, the
					// designer's displayed default — the runtime timing match
					// only fires for concrete values.
					Timing: cmp.Or(ccDef.Timing, approval.DefaultCCTiming),
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

	behavior.EventCollectorFromContext(ctx).Add(
		approval.NewFlowDeployedEvent(&flow, version.ID, version.Version),
	)

	return &version, nil
}
