package service

import (
	"context"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// AnnotationViewer evaluates audiences using the instance roles already loaded for the detail.
type AnnotationViewer struct {
	UserID      string
	applicantID string
	nodes       map[string]bool
}

func NewAnnotationViewer(applicantID, userID string, tasks []approval.Task) *AnnotationViewer {
	v := &AnnotationViewer{UserID: userID, applicantID: applicantID, nodes: make(map[string]bool)}
	for _, t := range tasks {
		if t.AssigneeID == userID || t.DelegatorID != nil && *t.DelegatorID == userID {
			v.nodes[t.NodeID] = true
		}
	}

	return v
}

// CanSee checks an annotation after the calling participant gate.
func (v *AnnotationViewer) CanSee(a *approval.Annotation, parentAuthor string) bool {
	if a.Author.ID == v.UserID {
		return true
	}

	if a.Status != approval.AnnotationPublished {
		return false
	}

	if a.ParentID != nil && v.UserID == parentAuthor {
		return true
	}

	switch a.Audience.Scope {
	case approval.AudienceParticipants:
		return true
	case approval.AudienceApplicant:
		return v.UserID == v.applicantID
	case approval.AudienceNodes:
		return slices.ContainsFunc(a.Audience.NodeIDs, func(id string) bool { return v.nodes[id] })
	case approval.AudienceUsers:
		return slices.Contains(a.Audience.UserIDs, v.UserID)
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
	if err := db.NewSelect().
		Model(&ccs).
		Select("node_id", "cc_user_id").
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID).Equals("cc_user_id", userID) }).
		Scan(ctx); err != nil {
		return nil, nil, err
	}

	return tasks, ccs, nil
}
