package command

import (
	"context"
	"fmt"
	"time"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// UrgeTaskCmd sends an urge notification for a pending task.
type UrgeTaskCmd struct {
	cqrs.BaseCommand

	TaskID  string
	UrgerID string
	Message string
}

// UrgeTaskHandler handles the UrgeTaskCmd command.
type UrgeTaskHandler struct {
	db           orm.DB
	taskSvc      *service.TaskService
	publisher    *dispatcher.EventPublisher
	userResolver approval.UserInfoResolver
}

// NewUrgeTaskHandler creates a new UrgeTaskHandler.
func NewUrgeTaskHandler(db orm.DB, taskSvc *service.TaskService, publisher *dispatcher.EventPublisher, userResolver approval.UserInfoResolver) *UrgeTaskHandler {
	return &UrgeTaskHandler{db: db, taskSvc: taskSvc, publisher: publisher, userResolver: userResolver}
}

func (h *UrgeTaskHandler) Handle(ctx context.Context, cmd UrgeTaskCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	var task approval.Task

	task.ID = cmd.TaskID

	if err := db.NewSelect().
		Model(&task).
		Select("status", "node_id", "instance_id", "assignee_id", "assignee_name").
		ForUpdate().
		WherePK().
		Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, shared.ErrTaskNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load task: %w", err)
	}

	if task.Status != approval.TaskPending {
		return cqrs.Unit{}, shared.ErrTaskNotPending
	}

	isParticipant, err := h.taskSvc.IsInstanceParticipant(ctx, db, task.InstanceID, cmd.UrgerID)
	if err != nil {
		return cqrs.Unit{}, err
	}

	if !isParticipant {
		return cqrs.Unit{}, shared.ErrAccessDenied
	}

	var node approval.FlowNode

	node.ID = task.NodeID

	if err := db.NewSelect().
		Model(&node).
		Select("urge_cooldown_minutes").
		WherePK().
		Scan(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("load node: %w", err)
	}

	cooldownMinutes := node.UrgeCooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = 30
	}

	cooldownSince := timex.Now().Add(-time.Duration(cooldownMinutes) * time.Minute)

	existingCount, err := db.NewSelect().
		Model((*approval.UrgeRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("task_id", cmd.TaskID).
				Equals("urger_id", cmd.UrgerID).
				GreaterThan("created_at", cooldownSince)
		}).
		Count(ctx)
	if err != nil {
		return cqrs.Unit{}, fmt.Errorf("check urge cooldown: %w", err)
	}

	if existingCount > 0 {
		return cqrs.Unit{}, result.Err(
			fmt.Sprintf("催办操作过于频繁，请 %d 分钟后再试", cooldownMinutes),
			result.WithCode(shared.ErrCodeUrgeCooldown),
		)
	}

	urgerName := shared.ResolveUserName(ctx, h.userResolver, cmd.UrgerID)

	record := &approval.UrgeRecord{
		InstanceID:     task.InstanceID,
		NodeID:         task.NodeID,
		TaskID:         &cmd.TaskID,
		UrgerID:        cmd.UrgerID,
		UrgerName:      urgerName,
		TargetUserID:   task.AssigneeID,
		TargetUserName: task.AssigneeName,
		Message:        cmd.Message,
	}
	if _, err := db.NewInsert().Model(record).Exec(ctx); err != nil {
		return cqrs.Unit{}, fmt.Errorf("insert urge record: %w", err)
	}

	event := approval.NewTaskUrgedEvent(
		task.InstanceID, task.NodeID, cmd.TaskID,
		cmd.UrgerID, urgerName, task.AssigneeID, task.AssigneeName, cmd.Message,
	)

	if err := h.publisher.PublishAll(ctx, db, []approval.DomainEvent{event}); err != nil {
		return cqrs.Unit{}, err
	}

	return cqrs.Unit{}, nil
}
