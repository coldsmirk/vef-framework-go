package service

import (
	"bytes"
	"context"
	"errors"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"slices"
	"strings"
	"unicode/utf8"
)

// AnnotationEditor coordinates writes with the same instance lock used by decisions.
type AnnotationEditor struct {
	tasks     *TaskService
	instances *InstanceService
	business  *business.Registry
}

func NewAnnotationEditor(tasks *TaskService, instances *InstanceService, registry *business.Registry) *AnnotationEditor {
	return &AnnotationEditor{tasks: tasks, instances: instances, business: registry}
}

func (e *AnnotationEditor) Save(ctx context.Context, db orm.DB, in approval.SaveAnnotationInput) (*approval.AnnotationView, error) {
	a := &approval.Annotation{Author: in.Operator, Status: approval.AnnotationDraft}
	if in.ID != "" {
		existing, err := e.editable(ctx, db, in.ID, in.Operator.ID, in.Caller)
		if err != nil {
			return nil, err
		}
		if in.TaskID != "" && (existing.TaskID == nil || *existing.TaskID != in.TaskID) || in.ParentID != "" && (existing.ParentID == nil || *existing.ParentID != in.ParentID) {
			return nil, approval.ErrAnnotationNotEditable
		}
		a = existing
		if a.TaskID != nil {
			in.TaskID = *a.TaskID
		}
		if a.ParentID != nil {
			in.ParentID = *a.ParentID
		}
	}
	if utf8.RuneCountInString(in.Content) > 4000 || (strings.TrimSpace(in.Content) == "" && (in.ParentID == "" || in.Resolution == nil || *in.Resolution != approval.ResolutionAddressed)) {
		return nil, errAnnotationContent
	}
	a.Content = in.Content
	var nodeName string
	var err error
	if in.ParentID != "" {
		nodeName, err = e.prepareResponse(ctx, db, in, a)
	} else {
		nodeName, err = e.prepareRoot(ctx, db, in, a)
	}
	if err != nil {
		return nil, err
	}
	if in.ID == "" {
		_, err = db.NewInsert().Model(a).Exec(ctx)
	} else {
		_, err = db.NewUpdate().Model(a).Select("anchor", "audience", "content", "resolution").WherePK().Exec(ctx)
	}
	if err != nil {
		return nil, err
	}
	view := a.View(nodeName)
	return &view, nil
}
func (e *AnnotationEditor) Delete(ctx context.Context, db orm.DB, in approval.DeleteAnnotationInput) error {
	a, err := e.editable(ctx, db, in.ID, in.Operator.ID, in.Caller)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model(a).WherePK().Exec(ctx)
	return err
}

// locked locates the immutable instance coordinate before acquiring the instance
// lock. The locking reread sees a draft published, discarded or deleted while waiting.
func (e *AnnotationEditor) locked(ctx context.Context, db orm.DB, id, userID string, caller approval.CallerContext, applicantAction bool) (*approval.Annotation, *approval.Instance, error) {
	a := approval.Annotation{ID: id}
	if err := db.NewSelect().Model(&a).Select("instance_id", "parent_id").WherePK().Scan(ctx); err != nil {
		return nil, nil, annotationReadError(err)
	}
	if applicantAction || a.ParentID != nil {
		userID = ""
	}
	inst, err := e.instances.LoadForUpdateAs(ctx, db, a.InstanceID, caller, userID)
	if err != nil {
		return nil, nil, annotationReadError(err)
	}
	if err := db.NewSelect().Model(&a).WherePK().ForUpdate().Scan(ctx); err != nil {
		return nil, nil, annotationReadError(err)
	}
	return &a, inst, nil
}
func annotationReadError(err error) error {
	if result.IsRecordNotFound(err) || errors.Is(err, approval.ErrInstanceNotFound) || errors.Is(err, approval.ErrTaskNotFound) {
		return approval.ErrAnnotationNotFound
	}
	return err
}
func (e *AnnotationEditor) editable(ctx context.Context, db orm.DB, id, userID string, caller approval.CallerContext) (*approval.Annotation, error) {
	a, inst, err := e.locked(ctx, db, id, userID, caller, false)
	if err != nil {
		return nil, err
	}
	if a.Author.ID != userID {
		visible, err := annotationAccessible(ctx, db, a, inst, userID)
		if err != nil {
			return nil, err
		}
		if !visible {
			return nil, approval.ErrAnnotationNotFound
		}
		return nil, approval.ErrAnnotationNotEditable
	}
	if a.Status != approval.AnnotationDraft {
		return nil, approval.ErrAnnotationNotEditable
	}
	if a.TaskID != nil {
		if _, err := e.taskContext(ctx, db, *a.TaskID, userID, caller, approval.ErrAnnotationNotEditable); err != nil {
			return nil, err
		}
	} else if inst.Status != approval.InstanceReturned || inst.ApplicantID != userID {
		return nil, approval.ErrAnnotationNotEditable
	}
	return a, nil
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
	return AnnotationVisible(a, inst.ApplicantID, userID, parentAuthor, tasks), nil
}
func (e *AnnotationEditor) taskContext(ctx context.Context, db orm.DB, taskID, userID string, caller approval.CallerContext, denied error) (*TaskContext, error) {
	tc, err := e.tasks.LoadTaskContextForNodeOperation(ctx, db, taskID, decisionOptions(userID, caller))
	if errors.Is(err, approval.ErrNotAssignee) || errors.Is(err, approval.ErrInstanceCompleted) || errors.Is(err, approval.ErrTaskNotPending) {
		return nil, denied
	}
	return tc, annotationReadError(err)
}
func (e *AnnotationEditor) prepareRoot(ctx context.Context, db orm.DB, in approval.SaveAnnotationInput, a *approval.Annotation) (string, error) {
	if in.TaskID == "" || in.Resolution != nil {
		return "", approval.ErrAnnotationNotAllowed
	}
	tc, err := e.taskContext(ctx, db, in.TaskID, in.Operator.ID, in.Caller, approval.ErrAnnotationNotAllowed)
	if err != nil {
		return "", err
	}
	aud, err := annotationAudience(ctx, db, tc.Instance.FlowVersionID, in.Audience)
	if err != nil {
		return "", err
	}
	anchor := bytes.TrimSpace(in.Anchor)
	if bytes.Equal(anchor, []byte("null")) {
		anchor = nil
	}
	if err := e.validateAnchor(ctx, db, &approval.AnchorContext{Instance: tc.Instance, Node: tc.Node, Task: tc.Task, Author: in.Operator, Anchor: anchor}); err != nil {
		return "", err
	}
	a.TenantID = tc.Instance.TenantID
	a.InstanceID = tc.Instance.ID
	a.NodeID = tc.Node.ID
	a.TaskID = &tc.Task.ID
	a.VisitID = &tc.Task.VisitID
	a.Audience = aud
	a.Anchor = anchor
	return tc.Node.Name, nil
}
func (e *AnnotationEditor) prepareResponse(ctx context.Context, db orm.DB, in approval.SaveAnnotationInput, a *approval.Annotation) (string, error) {
	parent, inst, err := e.locked(ctx, db, in.ParentID, in.Operator.ID, in.Caller, true)
	if err != nil {
		return "", err
	}
	if inst.ApplicantID != in.Operator.ID {
		return "", approval.ErrAnnotationNotAllowed
	}
	visible, err := annotationAccessible(ctx, db, parent, inst, in.Operator.ID)
	if err != nil {
		return "", err
	}
	if !visible {
		return "", approval.ErrAnnotationNotFound
	}
	if inst.Status != approval.InstanceReturned || in.TaskID != "" || parent.ParentID != nil || in.Resolution == nil || (*in.Resolution != approval.ResolutionAddressed && *in.Resolution != approval.ResolutionDeclined) {
		return "", approval.ErrAnnotationNotAllowed
	}
	if in.Audience != nil {
		return "", approval.ErrAnnotationAudienceInvalid
	}
	if len(in.Anchor) > 0 && !bytes.Equal(bytes.TrimSpace(in.Anchor), []byte("null")) {
		return "", approval.ErrAnnotationAnchorInvalid
	}
	var node approval.FlowNode
	if err := db.NewSelect().Model(&node).Select("id", "name").Where(func(cb orm.ConditionBuilder) {
		cb.Equals("flow_version_id", inst.FlowVersionID).Equals("kind", approval.NodeStart)
	}).Scan(ctx); err != nil {
		return "", err
	}
	a.TenantID = inst.TenantID
	a.InstanceID = inst.ID
	a.NodeID = node.ID
	a.ParentID = &parent.ID
	a.Resolution = in.Resolution
	a.Anchor = parent.Anchor
	a.Audience = parent.Audience
	return node.Name, nil
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
