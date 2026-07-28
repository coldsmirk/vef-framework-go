package query

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// at returns a deterministic timestamp m minutes into the test hour.
func at(m int) timex.DateTime {
	return timex.DateTime(time.Date(2026, 7, 2, 10, m, 0, 0, time.UTC))
}

// visitEntered builds a NodeVisit entered at the given time.
func visitEntered(id, nodeID string, sequence int, status approval.NodeVisitStatus, entered timex.DateTime, finished *timex.DateTime) approval.NodeVisit {
	v := visit(id, nodeID, sequence, status)
	v.CreatedAt = entered
	v.FinishedAt = finished

	return v
}

// logAt builds an ActionLog stamped at the given time.
func logAt(id string, action approval.ActionType, operatorID string, created timex.DateTime) approval.ActionLog {
	l := approval.ActionLog{Action: action, OperatorID: operatorID, OperatorName: operatorID}
	l.ID = id
	l.CreatedAt = created

	return l
}

// timelineBundle builds a start → approval → end bundle without runtime state;
// scenarios fill Visits / Tasks / ActionLogs / CCRecords / UrgeRecords.
func timelineBundle() *instanceDetailBundle {
	b := linearBundle()
	b.NodeNameMap = map[string]string{"ns": "kstart", "na": "kappr", "ne": "kend"}

	return b
}

func TestBuildInstanceTimeline(t *testing.T) {
	t.Run("LinearRunningTimeline", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitActive, at(0), nil),
		}

		pending := task("t1", "na", "u1", approval.TaskPending)
		pending.VisitID = "v2"
		done := task("t2", "na", "u2", approval.TaskApproved)
		done.VisitID = "v2"
		b.Tasks = []approval.Task{done, pending}

		submit := logAt("l0", approval.ActionSubmit, "applicant", at(0))
		finisher := logAt("l1", approval.ActionApprove, "u2", at(5))
		finisher.NodeID = new("na")
		finisher.TaskID = new("t2")
		finisher.Opinion = new("fine")
		finisher.Attachments = []string{"doc.pdf"}
		b.ActionLogs = []approval.ActionLog{submit, finisher}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 2, "Timeline should carry start and approval entries (end unvisited)")

		start := entries[0]
		assert.Equal(t, approval.TimelineEntryStart, start.Kind, "First entry is the start node")
		assert.Equal(t, approval.NodeVisitPassed, start.Status, "Start entry reports its visit outcome")
		require.Len(t, start.Activities, 1, "Start entry carries the submit activity")
		assert.Equal(t, string(approval.ActionSubmit), start.Activities[0].Action, "Submission is the start activity")
		assert.Equal(t, "applicant", start.Activities[0].Operator.ID, "Submitter is the activity operator")

		entry := entries[1]
		assert.Equal(t, approval.TimelineEntryApproval, entry.Kind, "Second entry is the approval node")
		assert.Equal(t, approval.NodeVisitActive, entry.Status, "Open visit renders active")
		require.Len(t, entry.Participants, 2, "Both tasks of the visit surface as participants")
		require.NotNil(t, entry.Participants[0].Opinion, "Finished participant fuses the finisher log")
		assert.Equal(t, "fine", *entry.Participants[0].Opinion, "Opinion comes from the finishing log")
		assert.Equal(t, []string{"doc.pdf"}, entry.Participants[0].Attachments, "Attachments come from the finishing log")
		assert.Nil(t, entry.Participants[1].Opinion, "Pending participant has no outcome yet")
	})

	t.Run("SkipsConditionVisitsButKeepsEnd", func(t *testing.T) {
		b := &instanceDetailBundle{
			FlowNodes: []approval.FlowNode{
				flowNode("ns", "kstart", approval.NodeStart),
				flowNode("nc", "kcond", approval.NodeCondition),
				flowNode("ne", "kend", approval.NodeEnd),
			},
		}
		b.Instance.Status = approval.InstanceApproved
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "nc", 2, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v3", "ne", 3, approval.NodeVisitPassed, at(1), new(at(1))),
		}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 2, "A condition visit is pure routing and is skipped; the end visit is kept")
		assert.Equal(t, approval.TimelineEntryStart, entries[0].Kind, "First entry is the start node")

		end := entries[1]
		assert.Equal(t, approval.TimelineEntryEnd, end.Kind, "The end visit closes the timeline")
		assert.Equal(t, approval.NodeVisitPassed, end.Status, "End entry reports its visit outcome")
		require.NotNil(t, end.NodeID, "End entry identifies its node")
		assert.Equal(t, "ne", *end.NodeID, "End entry points at the end node")
		require.NotNil(t, end.FinishedAt, "A reached end node is finished")
		assert.Empty(t, end.Participants, "An end node has no participants")
		assert.Empty(t, end.ExecutionType, "Approval-only display config stays unset on an end entry")
	})

	// An approved instance is the only final state with no milestone log and no
	// rejected visit, so the end entry is what makes it distinguishable from an
	// instance still moving between nodes.
	t.Run("ApprovedInstanceClosesWithEndEntry", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceApproved
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitPassed, at(0), new(at(5))),
			visitEntered("v3", "ne", 3, approval.NodeVisitPassed, at(5), new(at(5))),
		}

		done := task("t1", "na", "u1", approval.TaskApproved)
		done.VisitID = "v2"
		b.Tasks = []approval.Task{done}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 3, "Start, approval and end all render")
		assert.Equal(t, approval.TimelineEntryEnd, entries[2].Kind, "The timeline ends on the end entry")
	})

	// A rejection stops on the approval node and never traverses to the end
	// node, so its closing signal is the rejected visit — not an end entry.
	t.Run("RejectedInstanceHasNoEndEntry", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceRejected
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitRejected, at(0), new(at(5))),
		}

		done := task("t1", "na", "u1", approval.TaskRejected)
		done.VisitID = "v2"
		b.Tasks = []approval.Task{done}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 2, "A rejection never reaches the end node")
		assert.Equal(t, approval.TimelineEntryApproval, entries[1].Kind, "The timeline ends on the approval node")
		assert.Equal(t, approval.NodeVisitRejected, entries[1].Status, "The rejected visit is the closing signal")
	})

	t.Run("WithdrawMilestoneInterleavesAndResubmitReopens", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitCanceled, at(0), new(at(10))),
			visitEntered("v3", "ns", 3, approval.NodeVisitPassed, at(20), new(at(20))),
			visitEntered("v4", "na", 4, approval.NodeVisitActive, at(20), nil),
		}

		submit := logAt("l0", approval.ActionSubmit, "applicant", at(0))
		withdraw := logAt("l1", approval.ActionWithdraw, "applicant", at(10))
		withdraw.Opinion = new("hold on")
		resubmit := logAt("l2", approval.ActionResubmit, "applicant", at(20))
		b.ActionLogs = []approval.ActionLog{submit, withdraw, resubmit}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 5, "Trail entries plus the withdraw milestone")
		assert.Equal(t, approval.TimelineEntryStart, entries[0].Kind, "First submission opens the timeline")
		assert.Equal(t, approval.NodeVisitCanceled, entries[1].Status, "Withdrawn visit renders canceled")

		milestone := entries[2]
		assert.Equal(t, approval.TimelineEntryWithdraw, milestone.Kind, "Withdraw milestone sits between the two rounds")
		require.Len(t, milestone.Activities, 1, "Milestone carries the closing activity")
		assert.Equal(t, "applicant", milestone.Activities[0].Operator.ID, "Milestone names who withdrew")
		require.NotNil(t, milestone.Activities[0].Opinion, "Milestone carries the withdraw reason")

		secondStart := entries[3]
		assert.Equal(t, approval.TimelineEntryStart, secondStart.Kind, "Resubmit re-opens from the start node")
		require.Len(t, secondStart.Activities, 1, "Second start entry zips the resubmit log")
		assert.Equal(t, string(approval.ActionResubmit), secondStart.Activities[0].Action, "Second start activity is the resubmission")
		assert.Equal(t, approval.NodeVisitActive, entries[4].Status, "Re-opened approval visit is active")
	})

	t.Run("RollbackRoundsKeepPerVisitState", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitReturned, at(0), new(at(10))),
			visitEntered("v3", "na", 3, approval.NodeVisitActive, at(10), nil),
		}

		first := task("t1", "na", "u1", approval.TaskRolledBack)
		first.VisitID = "v2"
		second := task("t2", "na", "u1", approval.TaskPending)
		second.VisitID = "v3"
		b.Tasks = []approval.Task{first, second}

		rollback := logAt("l1", approval.ActionRollback, "u1", at(10))
		rollback.NodeID = new("na")
		rollback.TaskID = new("t1")
		rollback.RollbackToNodeID = new("na")
		b.ActionLogs = []approval.ActionLog{rollback}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 3, "Each traversal of the node is its own entry")

		returned := entries[1]
		assert.Equal(t, approval.NodeVisitReturned, returned.Status, "First round concluded by sending the flow back")
		require.Len(t, returned.Participants, 1, "First round keeps only its own participant")
		assert.Equal(t, "t1", returned.Participants[0].TaskID, "First round participant is the rolled-back task")
		require.Len(t, returned.Activities, 1, "Rollback activity binds to the round it happened in")
		require.NotNil(t, returned.Activities[0].RollbackToNodeName, "Rollback target name resolves from the node map")

		redo := entries[2]
		assert.Equal(t, approval.NodeVisitActive, redo.Status, "Second round is open")
		require.Len(t, redo.Participants, 1, "Second round keeps only its own participant")
		assert.Equal(t, "t2", redo.Participants[0].TaskID, "Second round participant is the fresh task")
		assert.Empty(t, redo.Activities, "Second round has no activities yet")
	})

	t.Run("BindsTasklessRecordsByVisitWindow", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceRunning
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitActive, at(0), nil),
		}

		pending := task("t1", "na", "u1", approval.TaskPending)
		pending.VisitID = "v2"
		b.Tasks = []approval.Task{pending}

		// Manual CC log carries a node but no task; the urge record binds
		// through its task.
		addCC := logAt("l1", approval.ActionAddCC, "u1", at(3))
		addCC.NodeID = new("na")
		addCC.CCUsers = []approval.UserInfo{{ID: "cc-1", Name: "CC One"}}
		b.ActionLogs = []approval.ActionLog{addCC}

		dept := "Legal"
		cc := approval.CCRecord{NodeID: new("na"), VisitID: new("v2"), CCUserID: "cc-1", CCUserName: "CC One", CCUserDepartmentName: &dept}
		cc.ID = "ccr-1"
		cc.CreatedAt = at(3)
		readAt := at(4)
		cc.ReadAt = &readAt
		b.CCRecords = []approval.CCRecord{cc}

		urge := approval.UrgeRecord{NodeID: "na", TaskID: new("t1"), UrgerID: "boss", UrgerName: "Boss", TargetUserID: "u1", TargetUserName: "u1", Message: "hurry"}
		urge.ID = "ur-1"
		urge.CreatedAt = at(6)
		b.UrgeRecords = []approval.UrgeRecord{urge}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 2, "Start and approval entries")
		entry := entries[1]

		require.Len(t, entry.CCRecipients, 1, "CC record binds to the open visit of its node")
		assert.Equal(t, "cc-1", entry.CCRecipients[0].User.ID, "CC recipient identity passes through")
		require.NotNil(t, entry.CCRecipients[0].User.DepartmentName, "CC recipient department passes through")
		require.NotNil(t, entry.CCRecipients[0].ReadAt, "Read receipt passes through")

		require.Len(t, entry.Activities, 2, "Manual CC and urge both surface as activities")
		assert.Equal(t, string(approval.ActionAddCC), entry.Activities[0].Action, "Manual CC activity binds by node window")
		assert.Equal(t, approval.ActivityUrge, entry.Activities[1].Action, "Urge activity binds through its task")
		require.NotNil(t, entry.Activities[1].Target, "Urge activity names the urged assignee")
		assert.Equal(t, "u1", entry.Activities[1].Target.ID, "Urge target is the task assignee")
		require.NotNil(t, entry.Activities[1].Opinion, "Urge message travels as the activity text")
	})

	t.Run("TerminalMilestoneAppendsAfterLastEntry", func(t *testing.T) {
		b := timelineBundle()
		b.Instance.Status = approval.InstanceTerminated
		b.Visits = []approval.NodeVisit{
			visitEntered("v1", "ns", 1, approval.NodeVisitPassed, at(0), new(at(0))),
			visitEntered("v2", "na", 2, approval.NodeVisitCanceled, at(0), new(at(30))),
		}
		terminate := logAt("l1", approval.ActionTerminate, "admin", at(30))
		terminate.Opinion = new("force close")
		b.ActionLogs = []approval.ActionLog{terminate}

		entries := buildInstanceTimeline(b)

		require.Len(t, entries, 3, "Trail entries plus the terminate milestone")
		last := entries[2]
		assert.Equal(t, approval.TimelineEntryTerminate, last.Kind, "Terminate milestone closes the timeline")
		require.Len(t, last.Activities, 1, "Milestone carries the closing activity")
		assert.Equal(t, "admin", last.Activities[0].Operator.ID, "Milestone names who terminated")
	})
}
