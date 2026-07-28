package query

import "github.com/coldsmirk/vef-framework-go/approval"

// buildInstanceTimeline projects the visit trail into the chronological,
// node-by-node account of the path the instance actually took: one entry per
// node visit (in traversal order, ending at the node currently in progress
// while the instance is executing) interleaved with instance-level milestones
// (withdraw / terminate) at their time positions. Everything a transit-record
// view renders is assembled by the shared visit index — participants fused
// from tasks and their finishing logs, CC recipients with read receipts, and
// side-action activities — so the client displays the list verbatim. Condition
// visits are pure routing and are skipped; the end visit is kept as the
// timeline's closing marker, because it is the only record that distinguishes
// an instance that finished by passing from one still moving between nodes.
func buildInstanceTimeline(bundle *instanceDetailBundle) []approval.TimelineEntry {
	idx := newVisitIndex(bundle)

	entries := make([]approval.TimelineEntry, 0, len(bundle.Visits)+len(idx.milestones))

	for i := range bundle.Visits {
		visit := &bundle.Visits[i]

		node := idx.nodeByID[visit.NodeID]
		if node == nil || node.Kind == approval.NodeCondition {
			continue
		}

		entry := approval.TimelineEntry{
			Kind:         approval.TimelineEntryKind(node.Kind),
			NodeID:       new(visit.NodeID),
			Name:         node.Name,
			Status:       visit.Status,
			StartedAt:    visit.CreatedAt,
			FinishedAt:   visit.FinishedAt,
			CCRecipients: idx.ccByVisit[visit.ID],
			Activities:   idx.activitiesByVisit[visit.ID],
		}

		switch node.Kind {
		case approval.NodeApproval:
			entry.ExecutionType = string(node.ExecutionType)
			entry.ApprovalMethod = string(node.ApprovalMethod)
			entry.PassRule = string(node.PassRule)

			if node.PassRule == approval.PassRatio {
				entry.PassRatio = new(node.PassRatio)
			}

			entry.Participants = buildParticipants(idx.tasksByVisit[visit.ID], idx.finisherLogs)

		case approval.NodeHandle:
			// Handle nodes claim-and-do; approvalMethod / passRule do not apply.
			entry.ExecutionType = string(node.ExecutionType)
			entry.Participants = buildParticipants(idx.tasksByVisit[visit.ID], idx.finisherLogs)
		}

		entries = append(entries, entry)
	}

	return mergeMilestones(entries, idx.milestones)
}

// milestoneEntry wraps an instance-level closing action (withdraw / terminate)
// as a standalone timeline entry holding a single activity that names who
// closed the instance and why.
func milestoneEntry(log approval.ActionLog) approval.TimelineEntry {
	return approval.TimelineEntry{
		Kind:       approval.TimelineEntryKind(log.Action),
		StartedAt:  log.CreatedAt,
		Activities: []approval.Activity{activityFromLog(log, nil)},
	}
}

// mergeMilestones interleaves instance-level milestones into the node-entry
// sequence by start time. On a timestamp tie the milestone goes first — the
// realistic tie is "withdraw, then resubmit re-opens the start node within the
// same clock tick", and the withdrawal happened first.
func mergeMilestones(entries, milestones []approval.TimelineEntry) []approval.TimelineEntry {
	if len(milestones) == 0 {
		return entries
	}

	merged := make([]approval.TimelineEntry, 0, len(entries)+len(milestones))
	next := 0

	for _, entry := range entries {
		for next < len(milestones) && !milestones[next].StartedAt.After(entry.StartedAt) {
			merged = append(merged, milestones[next])
			next++
		}

		merged = append(merged, entry)
	}

	return append(merged, milestones[next:]...)
}
