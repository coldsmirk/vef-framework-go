package service

import (
	"context"
	"slices"
	"strings"

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

func annotationAccessible(ctx context.Context, db orm.DB, a *approval.Annotation, inst *approval.Instance, userID string) (bool, error) {
	if a.Status != approval.AnnotationPublished {
		return false, nil
	}

	tasks, ccs, err := viewerRows(ctx, db, inst.ID, userID)
	if err != nil {
		return false, err
	}

	p := ResolveParticipation(inst, tasks, len(ccs) > 0, userID)
	if !p.IsParticipant() {
		return false, nil
	}

	parentAuthor := ""
	if a.ParentID != nil {
		parent := approval.Annotation{ID: *a.ParentID}
		if err := db.NewSelect().Model(&parent).Select("author_id").WherePK().Scan(ctx); err != nil {
			return false, annotationReadError(err)
		}

		parentAuthor = parent.Author.ID
	}

	return NewAnnotationViewer(inst.ApplicantID, userID, tasks).CanSee(a, parentAuthor), nil
}

func annotationAudience(ctx context.Context, db orm.DB, versionID string, input *approval.AnnotationAudience) (approval.AnnotationAudience, error) {
	a := approval.AnnotationAudience{Scope: approval.AudienceParticipants}
	if input == nil {
		return a, nil
	}

	a = *input
	if a.Scope == "" {
		a.Scope = approval.AudienceParticipants
	}

	switch a.Scope {
	case approval.AudienceParticipants, approval.AudienceApplicant:
		if len(a.NodeIDs)+len(a.UserIDs) != 0 {
			return a, approval.ErrAnnotationAudienceInvalid
		}
	case approval.AudienceNodes:
		if len(a.NodeIDs) == 0 || len(a.UserIDs) != 0 {
			return a, approval.ErrAnnotationAudienceInvalid
		}

		a.NodeIDs = slices.Compact(slices.Sorted(slices.Values(a.NodeIDs)))

		n, err := db.NewSelect().Model((*approval.FlowNode)(nil)).Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", versionID).In("id", a.NodeIDs) }).Count(ctx)
		if err != nil {
			return a, err
		}

		if int64(len(a.NodeIDs)) != n {
			return a, approval.ErrAnnotationAudienceInvalid
		}

	case approval.AudienceUsers:
		if len(a.UserIDs) == 0 || len(a.NodeIDs) != 0 || slices.ContainsFunc(a.UserIDs, func(s string) bool { return strings.TrimSpace(s) == "" }) {
			return a, approval.ErrAnnotationAudienceInvalid
		}

		a.UserIDs = slices.Compact(slices.Sorted(slices.Values(a.UserIDs)))

	default:
		return a, approval.ErrAnnotationAudienceInvalid
	}

	return a, nil
}
