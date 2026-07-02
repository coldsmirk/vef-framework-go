package approval

import (
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// TimelineEntryKind classifies one entry of an instance timeline. Node-visit
// entries reuse the node-kind vocabulary (start / approval / handle / cc);
// instance-level milestones (withdraw / terminate) carry their action name.
// Structural kinds (condition / end) never appear — they route or close the
// flow without anything to narrate.
type TimelineEntryKind string

const (
	TimelineEntryStart     TimelineEntryKind = "start"
	TimelineEntryApproval  TimelineEntryKind = "approval"
	TimelineEntryHandle    TimelineEntryKind = "handle"
	TimelineEntryCC        TimelineEntryKind = "cc"
	TimelineEntryWithdraw  TimelineEntryKind = "withdraw"
	TimelineEntryTerminate TimelineEntryKind = "terminate"
)

// TimelineEntry is one step of the instance timeline — the chronological,
// node-by-node account of the path an instance actually took, ready to render
// as a transit-record list without client-side reshaping. Because condition
// branches are exclusive, the traversed path is always a single line; a node
// re-entered after a rollback produces a second entry. Entries end at the node
// currently in progress — unreached nodes are not predicted.
//
// Node entries carry the node's display config and the people involved:
// Participants for approval/handle entries (one per task, fused with the log
// that finished it), CCRecipients for delivered carbon copies (both cc nodes
// and timing-based CC configured on approval/handle nodes), and Activities for
// side actions that happened at the node (transfer, rollback, add/remove
// assignee, manual CC, urge, system execution — and submit/resubmit on start
// entries). Milestone entries (withdraw / terminate) hold a single activity
// describing who closed the instance and why.
type TimelineEntry struct {
	Kind           TimelineEntryKind `json:"kind"`
	NodeID         *string           `json:"nodeId,omitempty"`
	Name           string            `json:"name,omitempty"`
	Status         NodeVisitStatus   `json:"status,omitempty"`
	ExecutionType  string            `json:"executionType,omitempty"`
	ApprovalMethod string            `json:"approvalMethod,omitempty"`
	PassRule       string            `json:"passRule,omitempty"`
	PassRatio      *decimal.Decimal  `json:"passRatio,omitempty"`
	Participants   []NodeParticipant `json:"participants,omitempty"`
	CCRecipients   []CCRecipient     `json:"ccRecipients,omitempty"`
	Activities     []Activity        `json:"activities,omitempty"`
	StartedAt      timex.DateTime    `json:"startedAt"`
	FinishedAt     *timex.DateTime   `json:"finishedAt,omitempty"`
}
