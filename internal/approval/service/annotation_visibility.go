package service

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"slices"
)

// AnnotationVisible applies the audience after the instance participant gate.
// Responses inherit their parent's audience and remain visible to the parent author.
func AnnotationVisible(a *approval.Annotation, applicant, viewer, parentAuthor string, tasks []approval.Task) bool {
	if a.Author.ID == viewer {
		return true
	}
	if a.Status != approval.AnnotationPublished {
		return false
	}
	if a.ParentID != nil && viewer == parentAuthor {
		return true
	}
	switch a.Audience.Scope {
	case approval.AudienceParticipants:
		return true
	case approval.AudienceApplicant:
		return viewer == applicant
	case approval.AudienceUsers:
		return slices.Contains(a.Audience.UserIDs, viewer)
	case approval.AudienceNodes:
		return slices.ContainsFunc(tasks, func(t approval.Task) bool {
			return (t.AssigneeID == viewer || t.DelegatorID != nil && *t.DelegatorID == viewer) && slices.Contains(a.Audience.NodeIDs, t.NodeID)
		})
	default:
		return false
	}
}

// viewerRows loads only the contexts a single person contributes to participation and visibility.
func viewerRows(ctx context.Context, db orm.DB, instanceID, userID string) ([]approval.Task, []approval.CCRecord, error) {
	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).Select("id", "node_id", "assignee_id", "delegator_id", "status").Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instanceID).Group(func(cb orm.ConditionBuilder) { cb.Equals("assignee_id", userID).OrEquals("delegator_id", userID) })
	}).Scan(ctx); err != nil {
		return nil, nil, err
	}
	var ccs []approval.CCRecord
	if err := db.NewSelect().Model(&ccs).Select("node_id", "cc_user_id").Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID).Equals("cc_user_id", userID) }).Scan(ctx); err != nil {
		return nil, nil, err
	}
	return tasks, ccs, nil
}
