package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// ActivityUrge is the Activity.Action value for urge records — the one
// vocabulary member with no ActionType counterpart, because urges are
// persisted as urge records rather than action logs. Every other Action value
// is an ActionType string verbatim.
const ActivityUrge = "urge"

// NodeParticipant is one assignee's involvement at an approval/handle node
// during a single visit: the task identity (TaskID is what task operations are
// submitted against), the assignee (plus delegator when the task arrived via
// delegation), the task status verbatim, and — once the task is finished — the
// outcome details fused from the action log that finished it. IsTimeout marks
// tasks the timeout scanner decided or escalated. Shared by the timeline and
// the flow-graph projections so the two views can never disagree about who did
// what.
type NodeParticipant struct {
	TaskID      string          `json:"taskId"`
	User        UserInfo        `json:"user"`
	Delegator   *UserInfo       `json:"delegator,omitempty"`
	Status      string          `json:"status"`
	Deadline    *timex.DateTime `json:"deadline,omitempty"`
	IsTimeout   bool            `json:"isTimeout,omitempty"`
	Opinion     *string         `json:"opinion,omitempty"`
	Attachments []string        `json:"attachments,omitempty"`
	ActionTime  *timex.DateTime `json:"actionTime,omitempty"`
	TransferTo  *UserInfo       `json:"transferTo,omitempty"`
}

// Activity is a side action recorded at a node: who did what, when, and the
// action-specific details. Action carries the ActionType string (transfer /
// rollback / add_assignee / remove_assignee / add_cc / reassign / execute /
// submit / resubmit / withdraw / terminate) plus ActivityUrge for urge
// records. Opinion holds the action's free text (a transfer reason, a withdraw
// reason, an urge message); Target names the counterpart of a directed action
// (the urged assignee). Decisions themselves (approve / handle / reject) are
// not repeated here — they live on the participant that made them. Shared by
// the timeline and the flow-graph projections.
type Activity struct {
	Action             string         `json:"action"`
	Operator           UserInfo       `json:"operator"`
	Opinion            *string        `json:"opinion,omitempty"`
	Attachments        []string       `json:"attachments,omitempty"`
	TransferTo         *UserInfo      `json:"transferTo,omitempty"`
	Target             *UserInfo      `json:"target,omitempty"`
	RollbackToNodeID   *string        `json:"rollbackToNodeId,omitempty"`
	RollbackToNodeName *string        `json:"rollbackToNodeName,omitempty"`
	AddedAssignees     []UserInfo     `json:"addedAssignees,omitempty"`
	RemovedAssignees   []UserInfo     `json:"removedAssignees,omitempty"`
	CCUsers            []UserInfo     `json:"ccUsers,omitempty"`
	CreatedAt          timex.DateTime `json:"createdAt"`
}

// CCRecipient is one carbon-copy recipient at a node, with the read receipt
// when the recipient has confirmed reading. Shared by the timeline and the
// flow-graph projections.
type CCRecipient struct {
	User   UserInfo        `json:"user"`
	ReadAt *timex.DateTime `json:"readAt,omitempty"`
}
