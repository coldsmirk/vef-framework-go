package approval

import "github.com/coldsmirk/vef-framework-go/timex"

// InstanceEventBase is the shared envelope of every instance-scoped domain
// event. It is sized so a subscriber can act without querying the approval
// tables: route on TenantID / FlowCode, locate the host's own record through
// BusinessRef, and render a notification from Title / InstanceNo / Applicant.
// Fields are snapshots taken when the event fires — they describe the world
// at that moment, not the current row. Unbounded payloads (form data) stay
// out deliberately; subscribers that need them fetch by InstanceID.
type InstanceEventBase struct {
	InstanceID   string         `json:"instanceId"`
	InstanceNo   string         `json:"instanceNo"`
	TenantID     string         `json:"tenantId"`
	Title        string         `json:"title"`
	FlowID       string         `json:"flowId"`
	FlowCode     string         `json:"flowCode"`
	BusinessRef  *string        `json:"businessRef,omitempty"`
	Applicant    UserInfo       `json:"applicant"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

// instanceEventBase exposes the embedded envelope to the closed
// InstanceEvent interface: every event type embedding InstanceEventBase
// implements it automatically, and no type outside this package can.
func (b InstanceEventBase) instanceEventBase() InstanceEventBase { return b }

// NewInstanceEventBase snapshots the instance into the shared event envelope,
// stamping OccurredTime with the current time.
func NewInstanceEventBase(instance *Instance) InstanceEventBase {
	return InstanceEventBase{
		InstanceID:   instance.ID,
		InstanceNo:   instance.InstanceNo,
		TenantID:     instance.TenantID,
		Title:        instance.Title,
		FlowID:       instance.FlowID,
		FlowCode:     instance.FlowCode,
		BusinessRef:  instance.BusinessRef,
		Applicant:    instance.Applicant(),
		OccurredTime: timex.Now(),
	}
}

// TaskEventBase extends the instance envelope with the task coordinates a
// to-do style subscriber needs: which task, on which node, under which name.
type TaskEventBase struct {
	InstanceEventBase

	TaskID   string `json:"taskId"`
	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName"`
}

// NewTaskEventBase snapshots the instance, task, and node coordinates into
// the shared task event envelope.
func NewTaskEventBase(instance *Instance, task *Task, node *FlowNode) TaskEventBase {
	return TaskEventBase{
		InstanceEventBase: NewInstanceEventBase(instance),
		TaskID:            task.ID,
		NodeID:            node.ID,
		NodeName:          node.Name,
	}
}

// FlowEventBase is the shared envelope of flow-definition events. Code and
// Name ride along so subscribers can route and render without loading the
// flow row.
type FlowEventBase struct {
	FlowID       string         `json:"flowId"`
	TenantID     string         `json:"tenantId"`
	Code         string         `json:"code"`
	Name         string         `json:"name"`
	OccurredTime timex.DateTime `json:"occurredTime"`
}

// NewFlowEventBase snapshots the flow into the shared event envelope.
func NewFlowEventBase(flow *Flow) FlowEventBase {
	return FlowEventBase{
		FlowID:       flow.ID,
		TenantID:     flow.TenantID,
		Code:         flow.Code,
		Name:         flow.Name,
		OccurredTime: timex.Now(),
	}
}
