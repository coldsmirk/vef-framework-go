package service

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// AnnotationService maintains the draft lifecycle under the caller's instance lock.
type AnnotationService struct{}

// PublishDrafts publishes the author's task drafts, or their response drafts on resubmission.
func (AnnotationService) PublishDrafts(ctx context.Context, db orm.DB, instance *approval.Instance, taskID *string, author approval.UserInfo) error {
	var drafts []approval.Annotation
	if err := db.NewSelect().Model(&drafts).Select("id", "node_id").Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instance.ID).Equals("author_id", author.ID).Equals("status", approval.AnnotationDraft)
		if taskID == nil {
			cb.IsNotNull("parent_id")
		} else {
			cb.Equals("task_id", *taskID)
		}
	}).OrderBy("id").ForUpdate().Scan(ctx); err != nil {
		return err
	}
	if len(drafts) == 0 {
		return nil
	}
	ids := make([]string, len(drafts))
	for i := range drafts {
		ids[i] = drafts[i].ID
	}
	if _, err := db.NewUpdate().Model((*approval.Annotation)(nil)).Set("status", approval.AnnotationPublished).Set("published_at", timex.Now()).Where(func(cb orm.ConditionBuilder) { cb.In("id", ids).Equals("status", approval.AnnotationDraft) }).Exec(ctx); err != nil {
		return err
	}
	return behavior.EmitEvents(ctx, nil, db, &approval.AnnotationPublishedEvent{InstanceEventBase: approval.NewInstanceEventBase(instance), TaskID: taskID, NodeID: drafts[0].NodeID, Author: author, AnnotationIDs: ids})
}

// DiscardTaskDrafts retires unpublished drafts when their original seat is lost.
func (AnnotationService) DiscardTaskDrafts(ctx context.Context, db orm.DB, taskIDs ...string) error {
	if len(taskIDs) == 0 {
		return nil
	}
	_, err := db.NewUpdate().Model((*approval.Annotation)(nil)).Set("status", approval.AnnotationDiscarded).Where(func(cb orm.ConditionBuilder) { cb.In("task_id", taskIDs).Equals("status", approval.AnnotationDraft) }).Exec(ctx)
	return err
}
func (AnnotationService) discardResponses(ctx context.Context, db orm.DB, instanceID string) error {
	_, err := db.NewUpdate().Model((*approval.Annotation)(nil)).Set("status", approval.AnnotationDiscarded).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instanceID).IsNotNull("parent_id").Equals("status", approval.AnnotationDraft)
	}).Exec(ctx)
	return err
}
