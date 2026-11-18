package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
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

	var (
		inst *approval.Instance
		err  error
	)
	if in.ID != "" {
		a, inst, err = e.lockDraft(ctx, db, in.ID, in.Operator.ID, in.Caller)
		if err != nil {
			return nil, err
		}

		if in.TaskID != "" && (a.TaskID == nil || *a.TaskID != in.TaskID) || in.ParentID != "" && (a.ParentID == nil || *a.ParentID != in.ParentID) {
			return nil, approval.ErrAnnotationNotEditable
		}

		if a.TaskID != nil {
			in.TaskID = *a.TaskID
		}

		if a.ParentID != nil {
			in.ParentID = *a.ParentID
		}
	}

	var nodeName string
	if in.ParentID != "" {
		nodeName, err = e.prepareResponse(ctx, db, in, a, inst)
	} else {
		nodeName, err = e.prepareRoot(ctx, db, in, a, inst)
	}

	if err != nil {
		return nil, err
	}

	a.Content = in.Content

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
	a, inst, err := e.lockDraft(ctx, db, in.ID, in.Operator.ID, in.Caller)
	if err != nil {
		return err
	}

	if a.TaskID != nil {
		if _, err := e.taskContext(ctx, db, inst, *a.TaskID, in.Operator.ID, in.Caller); err != nil {
			return err
		}
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

func (e *AnnotationEditor) lockDraft(ctx context.Context, db orm.DB, id, userID string, caller approval.CallerContext) (*approval.Annotation, *approval.Instance, error) {
	a, inst, err := e.locked(ctx, db, id, userID, caller, false)
	if err != nil {
		return nil, nil, err
	}

	if a.Author.ID != userID {
		visible, err := annotationAccessible(ctx, db, a, inst, userID)
		if err != nil {
			return nil, nil, err
		}

		if !visible {
			return nil, nil, approval.ErrAnnotationNotFound
		}

		return nil, nil, approval.ErrAnnotationNotEditable
	}

	if a.Status != approval.AnnotationDraft {
		return nil, nil, approval.ErrAnnotationNotEditable
	}

	if a.TaskID == nil && (inst.Status != approval.InstanceReturned || inst.ApplicantID != userID) {
		return nil, nil, approval.ErrAnnotationNotEditable
	}

	return a, inst, nil
}

func (e *AnnotationEditor) taskContext(ctx context.Context, db orm.DB, locked *approval.Instance, taskID, userID string, caller approval.CallerContext) (*TaskContext, error) {
	opts := decisionOptions(userID, caller)

	var (
		tc  *TaskContext
		err error
	)

	denied := approval.ErrAnnotationNotAllowed
	if locked == nil {
		tc, err = e.tasks.LoadTaskContextForNodeOperation(ctx, db, taskID, opts)
	} else {
		tc, err = lockTaskContext(ctx, db, locked, taskID, opts)
		denied = approval.ErrAnnotationNotEditable
	}

	if errors.Is(err, approval.ErrNotAssignee) || errors.Is(err, approval.ErrInstanceCompleted) || errors.Is(err, approval.ErrTaskNotPending) {
		return nil, denied
	}

	return tc, annotationReadError(err)
}

func (e *AnnotationEditor) prepareRoot(ctx context.Context, db orm.DB, in approval.SaveAnnotationInput, a *approval.Annotation, inst *approval.Instance) (string, error) {
	if in.TaskID == "" {
		return "", approval.ErrAnnotationNotAllowed
	}

	tc, err := e.taskContext(ctx, db, inst, in.TaskID, in.Operator.ID, in.Caller)
	if err != nil {
		return "", err
	}

	if in.Resolution != nil {
		return "", approval.ErrAnnotationNotAllowed
	}

	if err := validateAnnotationContent(in.Content, false); err != nil {
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

func (e *AnnotationEditor) prepareResponse(ctx context.Context, db orm.DB, in approval.SaveAnnotationInput, a *approval.Annotation, inst *approval.Instance) (string, error) {
	parent := &approval.Annotation{ID: in.ParentID}
	if inst == nil {
		var err error

		parent, inst, err = e.locked(ctx, db, in.ParentID, in.Operator.ID, in.Caller, true)
		if err != nil {
			return "", err
		}
	} else if err := db.NewSelect().Model(parent).WherePK().ForUpdate().Scan(ctx); err != nil {
		return "", annotationReadError(err)
	}

	visible, err := annotationAccessible(ctx, db, parent, inst, in.Operator.ID)
	if err != nil {
		return "", err
	}

	if !visible {
		return "", approval.ErrAnnotationNotFound
	}

	if inst.ApplicantID != in.Operator.ID || inst.Status != approval.InstanceReturned || in.TaskID != "" || parent.ParentID != nil || in.Resolution == nil ||
		(*in.Resolution != approval.ResolutionAddressed && *in.Resolution != approval.ResolutionDeclined) {
		return "", approval.ErrAnnotationNotAllowed
	}

	if err := validateAnnotationContent(in.Content, *in.Resolution == approval.ResolutionAddressed); err != nil {
		return "", err
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

func validateAnnotationContent(content string, allowEmpty bool) error {
	if utf8.RuneCountInString(content) > 4000 || (!allowEmpty && strings.TrimSpace(content) == "") {
		return errAnnotationContent
	}

	return nil
}
