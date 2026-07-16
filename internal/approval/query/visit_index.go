package query

import (
	"slices"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// participantFinishers are the actions whose log finishes a task; only these
// fuse into a participant's outcome. Side actions that also reference a task
// (add_assignee, reassign) are activities — letting them in would overwrite
// the decision log of the task they were performed from.
var participantFinishers = collections.NewHashSetFrom(
	approval.ActionApprove,
	approval.ActionHandle,
	approval.ActionReject,
	approval.ActionTransfer,
	approval.ActionRollback,
	approval.ActionRemoveAssignee,
	approval.ActionExecute,
)

// visitIndex is the one-pass assembly of everything the detail projections
// render about node execution: the visit trail grouped by node, each visit's
// tasks, CC recipients, and activities (side-action logs, urge records, and
// the submit/resubmit that opened each start visit), the task-finisher log
// index, and the instance-level milestones. buildInstanceTimeline and
// buildInstanceFlowGraph both read it, so the two projections cannot disagree
// about what happened at a node.
type visitIndex struct {
	nodeByID          map[string]*approval.FlowNode
	visitsByNode      map[string][]*approval.NodeVisit
	tasksByVisit      map[string][]approval.Task
	finisherLogs      map[string]approval.ActionLog
	activitiesByVisit map[string][]approval.Activity
	ccByVisit         map[string][]approval.CCRecipient
	milestones        []approval.TimelineEntry
}

// newVisitIndex assembles the shared projection index from the loaded bundle.
func newVisitIndex(bundle *instanceDetailBundle) *visitIndex {
	idx := &visitIndex{
		nodeByID:          make(map[string]*approval.FlowNode, len(bundle.FlowNodes)),
		visitsByNode:      make(map[string][]*approval.NodeVisit, len(bundle.Visits)),
		tasksByVisit:      make(map[string][]approval.Task, len(bundle.Visits)),
		finisherLogs:      participantLogIndex(bundle.ActionLogs),
		activitiesByVisit: make(map[string][]approval.Activity),
		ccByVisit:         make(map[string][]approval.CCRecipient),
	}

	for i := range bundle.FlowNodes {
		idx.nodeByID[bundle.FlowNodes[i].ID] = &bundle.FlowNodes[i]
	}

	for i := range bundle.Visits {
		visit := &bundle.Visits[i]
		idx.visitsByNode[visit.NodeID] = append(idx.visitsByNode[visit.NodeID], visit)
	}

	taskByID := make(map[string]*approval.Task, len(bundle.Tasks))

	for i := range bundle.Tasks {
		task := &bundle.Tasks[i]
		taskByID[task.ID] = task
		idx.tasksByVisit[task.VisitID] = append(idx.tasksByVisit[task.VisitID], *task)
	}

	// bindActivity attaches an activity to the visit that owns it: through the
	// referenced task's visit when bound, else the node visit open at the
	// activity's time.
	bindActivity := func(taskID, nodeID *string, at timex.DateTime, activity approval.Activity) {
		if taskID != nil {
			if task := taskByID[*taskID]; task != nil {
				idx.activitiesByVisit[task.VisitID] = append(idx.activitiesByVisit[task.VisitID], activity)

				return
			}
		}

		if nodeID == nil {
			return
		}

		if visit := visitAt(idx.visitsByNode, *nodeID, at); visit != nil {
			idx.activitiesByVisit[visit.ID] = append(idx.activitiesByVisit[visit.ID], activity)
		}
	}

	var startLogs []approval.ActionLog

	for _, log := range bundle.ActionLogs {
		switch log.Action {
		case approval.ActionSubmit, approval.ActionResubmit:
			// Instance-level: each one caused a start-node visit; zipped below.
			startLogs = append(startLogs, log)

		case approval.ActionWithdraw, approval.ActionTerminate:
			idx.milestones = append(idx.milestones, milestoneEntry(log))

		case approval.ActionApprove, approval.ActionHandle, approval.ActionReject:
			// Decisions live on the participant that made them.

		default:
			// Node-scoped side actions become activities — except task-scoped
			// execute logs, which are participant finishers.
			if log.Action == approval.ActionExecute && log.TaskID != nil {
				continue
			}

			bindActivity(log.TaskID, log.NodeID, log.CreatedAt, activityFromLog(log, bundle.NodeNameMap))
		}
	}

	// The n-th start visit was caused by the n-th submit/resubmit.
	startsSeen := 0

	for i := range bundle.Visits {
		visit := &bundle.Visits[i]
		if node := idx.nodeByID[visit.NodeID]; node == nil || node.Kind != approval.NodeStart {
			continue
		}

		if startsSeen < len(startLogs) {
			idx.activitiesByVisit[visit.ID] = append(idx.activitiesByVisit[visit.ID], activityFromLog(startLogs[startsSeen], bundle.NodeNameMap))
		}

		startsSeen++
	}

	for i := range bundle.CCRecords {
		record := &bundle.CCRecords[i]
		if record.VisitID == nil {
			// Instance-level records are not anchored to a node traversal.
			continue
		}

		idx.ccByVisit[*record.VisitID] = append(idx.ccByVisit[*record.VisitID], record.Recipient())
	}

	for i := range bundle.UrgeRecords {
		record := &bundle.UrgeRecords[i]
		bindActivity(record.TaskID, new(record.NodeID), record.CreatedAt, activityFromUrge(record))
	}

	// Activities accumulate from independently-ordered sources (action logs,
	// urge records, the start zip); order each visit's list chronologically.
	for _, activities := range idx.activitiesByVisit {
		slices.SortStableFunc(activities, func(a, b approval.Activity) int {
			switch {
			case a.CreatedAt.Before(b.CreatedAt):
				return -1
			case b.CreatedAt.Before(a.CreatedAt):
				return 1
			default:
				return 0
			}
		})
	}

	return idx
}

// participantLogIndex maps each task to the log that finished it, so a
// participant's opinion, attachments, and action time come from the exact
// action that ended the task — repeated visits (rollback → redo) keep their
// own logs because tasks, not nodes, key the index.
func participantLogIndex(logs []approval.ActionLog) map[string]approval.ActionLog {
	index := make(map[string]approval.ActionLog, len(logs))

	for _, log := range logs {
		if log.TaskID == nil || !participantFinishers.Contains(log.Action) {
			continue
		}

		index[*log.TaskID] = log
	}

	return index
}

// buildParticipants assembles the per-assignee involvement list for an
// approval or handle node from its tasks (identity, status, order) fused with
// the log that finished each task (opinion, attachments, transfer target,
// action time). Tasks arrive in sort order, which the list preserves.
func buildParticipants(tasks []approval.Task, finisherLogs map[string]approval.ActionLog) []approval.NodeParticipant {
	if len(tasks) == 0 {
		return nil
	}

	participants := make([]approval.NodeParticipant, len(tasks))

	for i, task := range tasks {
		participant := approval.NodeParticipant{
			TaskID:    task.ID,
			User:      task.Assignee(),
			Delegator: task.Delegator(),
			Status:    string(task.Status),
			Deadline:  task.Deadline,
			IsTimeout: task.IsTimeout,
		}

		if log, ok := finisherLogs[task.ID]; ok {
			participant.Opinion = log.Opinion
			participant.Attachments = log.Attachments
			participant.ActionTime = new(log.CreatedAt)
			participant.TransferTo = log.TransferTo()
		} else if task.FinishedAt != nil {
			participant.ActionTime = task.FinishedAt
		}

		participants[i] = participant
	}

	return participants
}

// activityFromLog projects an action log into an activity, resolving the
// rollback target's display name from the node-name map when present.
func activityFromLog(log approval.ActionLog, nodeNames map[string]string) approval.Activity {
	activity := approval.Activity{
		Action:           string(log.Action),
		Operator:         log.Operator(),
		Opinion:          log.Opinion,
		Attachments:      log.Attachments,
		TransferTo:       log.TransferTo(),
		RollbackToNodeID: log.RollbackToNodeID,
		AddedAssignees:   log.AddedAssignees,
		RemovedAssignees: log.RemovedAssignees,
		CCUsers:          log.CCUsers,
		CreatedAt:        log.CreatedAt,
	}

	if log.RollbackToNodeID != nil {
		if name, ok := nodeNames[*log.RollbackToNodeID]; ok {
			activity.RollbackToNodeName = &name
		}
	}

	return activity
}

// activityFromUrge projects an urge record into an activity: the urger acts on
// the target assignee, with the urge message as the free text.
func activityFromUrge(record *approval.UrgeRecord) approval.Activity {
	activity := approval.Activity{
		Action:    approval.ActivityUrge,
		Operator:  record.Urger(),
		Target:    new(record.Target()),
		CreatedAt: record.CreatedAt,
	}

	if record.Message != "" {
		activity.Opinion = &record.Message
	}

	return activity
}

// visitAt returns the visit of the node that was open at the given time — the
// latest visit entered at or before it — falling back to the node's first
// visit when the time precedes all of them (sub-second truncation on some
// dialects can order a record marginally before the visit that produced it).
// Returns nil when the node has no visits.
func visitAt(visitsByNode map[string][]*approval.NodeVisit, nodeID string, at timex.DateTime) *approval.NodeVisit {
	visits := visitsByNode[nodeID]

	for _, visit := range slices.Backward(visits) {
		if !visit.CreatedAt.After(at) {
			return visit
		}
	}

	if len(visits) > 0 {
		return visits[0]
	}

	return nil
}
