package approval

import (
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// OperatorInfo bundles operator identity for action logging.
type OperatorInfo struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	DeptID   *string `json:"deptId,omitempty"`
	DeptName *string `json:"deptName,omitempty"`
}

// NewActionLog creates an ActionLog with the operator fields pre-filled.
func (o OperatorInfo) NewActionLog(instanceID string, action ActionType) *ActionLog {
	return &ActionLog{
		InstanceID:       instanceID,
		Action:           action,
		OperatorID:       o.ID,
		OperatorName:     o.Name,
		OperatorDeptID:   o.DeptID,
		OperatorDeptName: o.DeptName,
	}
}

// ── Flow Definition ─────────────────────────────────────────────────────────

// Flow represents a flow definition.
type Flow struct {
	orm.BaseModel `bun:"table:apv_flow,alias:af"`
	orm.Model

	TenantID               string      `json:"tenantId" bun:"tenant_id"`
	CategoryID             string      `json:"categoryId" bun:"category_id"`
	Code                   string      `json:"code" bun:"code"`
	Name                   string      `json:"name" bun:"name"`
	Icon                   *string     `json:"icon" bun:"icon,nullzero"`
	Description            *string     `json:"description" bun:"description,nullzero"`
	BindingMode            BindingMode `json:"bindingMode" bun:"binding_mode"`
	BusinessTable          *string     `json:"businessTable" bun:"business_table,nullzero"`
	BusinessPkField        *string     `json:"businessPkField" bun:"business_pk_field,nullzero"`
	BusinessTitleField     *string     `json:"businessTitleField" bun:"business_title_field,nullzero"`
	BusinessStatusField    *string     `json:"businessStatusField" bun:"business_status_field,nullzero"`
	AdminUserIDs           []string    `json:"adminUserIds" bun:"admin_user_ids,type:jsonb"`
	IsAllInitiationAllowed bool        `json:"isAllInitiationAllowed" bun:"is_all_initiation_allowed"`
	InstanceTitleTemplate  string      `json:"instanceTitleTemplate" bun:"instance_title_template"`
	IsActive               bool        `json:"isActive" bun:"is_active"`
	CurrentVersion         int         `json:"currentVersion" bun:"current_version"`
}

// FlowCategory represents a category for grouping flows.
type FlowCategory struct {
	orm.BaseModel `bun:"table:apv_flow_category,alias:afc"`
	orm.Model

	TenantID  string         `json:"tenantId" bun:"tenant_id"`
	Code      string         `json:"code" bun:"code"`
	Name      string         `json:"name" bun:"name"`
	Icon      *string        `json:"icon" bun:"icon,nullzero"`
	ParentID  *string        `json:"parentId" bun:"parent_id,nullzero"`
	SortOrder int            `json:"sortOrder" bun:"sort_order"`
	IsActive  bool           `json:"isActive" bun:"is_active"`
	Remark    *string        `json:"remark" bun:"remark,nullzero"`
	Children  []FlowCategory `json:"children,omitempty" bun:"-"`
}

// FlowVersion represents a versioned snapshot of a flow definition.
type FlowVersion struct {
	orm.BaseModel `bun:"table:apv_flow_version,alias:afv"`
	orm.Model

	FlowID      string          `json:"flowId" bun:"flow_id"`
	Version     int             `json:"version" bun:"version"`
	Status      VersionStatus   `json:"status" bun:"status"`
	Description *string         `json:"description" bun:"description,nullzero"`
	StorageMode StorageMode     `json:"storageMode" bun:"storage_mode"`
	FlowSchema  *FlowDefinition `json:"flowSchema" bun:"flow_schema,type:jsonb,nullzero"`
	FormSchema  *FormDefinition `json:"formSchema" bun:"form_schema,type:jsonb,nullzero"`
	PublishedAt *timex.DateTime `json:"publishedAt" bun:"published_at,nullzero"`
	PublishedBy *string         `json:"publishedBy" bun:"published_by,nullzero"`
}

// FlowNode represents a node within a flow version.
type FlowNode struct {
	orm.BaseModel `bun:"table:apv_flow_node,alias:afn"`
	orm.Model

	FlowVersionID             string                    `json:"flowVersionId" bun:"flow_version_id"`
	Key                       string                    `json:"key" bun:"key"`
	Kind                      NodeKind                  `json:"kind" bun:"kind"`
	Name                      string                    `json:"name" bun:"name"`
	Description               *string                   `json:"description" bun:"description,nullzero"`
	ExecutionType             ExecutionType             `json:"executionType" bun:"execution_type"`
	ApprovalMethod            ApprovalMethod            `json:"approvalMethod" bun:"approval_method"`
	PassRule                  PassRule                  `json:"passRule" bun:"pass_rule"`
	PassRatio                 decimal.Decimal           `json:"passRatio" bun:"pass_ratio"`
	EmptyAssigneeAction       EmptyAssigneeAction       `json:"emptyAssigneeAction" bun:"empty_assignee_action"`
	FallbackUserIDs           []string                  `json:"fallbackUserIds" bun:"fallback_user_ids,type:jsonb"`
	AdminUserIDs              []string                  `json:"adminUserIds" bun:"admin_user_ids,type:jsonb"`
	SameApplicantAction       SameApplicantAction       `json:"sameApplicantAction" bun:"same_applicant_action"`
	IsRollbackAllowed         bool                      `json:"isRollbackAllowed" bun:"is_rollback_allowed"`
	RollbackType              RollbackType              `json:"rollbackType" bun:"rollback_type"`
	RollbackDataStrategy      RollbackDataStrategy      `json:"rollbackDataStrategy" bun:"rollback_data_strategy"`
	RollbackTargetKeys        []string                  `json:"rollbackTargetKeys" bun:"rollback_target_keys,type:jsonb,nullzero"`
	IsAddAssigneeAllowed      bool                      `json:"isAddAssigneeAllowed" bun:"is_add_assignee_allowed"`
	AddAssigneeTypes          []AddAssigneeType         `json:"addAssigneeTypes" bun:"add_assignee_types,type:jsonb"`
	IsRemoveAssigneeAllowed   bool                      `json:"isRemoveAssigneeAllowed" bun:"is_remove_assignee_allowed"`
	FieldPermissions          map[string]Permission     `json:"fieldPermissions" bun:"field_permissions,type:jsonb"`
	IsManualCCAllowed         bool                      `json:"isManualCcAllowed" bun:"is_manual_cc_allowed"`
	IsTransferAllowed         bool                      `json:"isTransferAllowed" bun:"is_transfer_allowed"`
	IsOpinionRequired         bool                      `json:"isOpinionRequired" bun:"is_opinion_required"`
	TimeoutHours              int                       `json:"timeoutHours" bun:"timeout_hours"`
	TimeoutAction             TimeoutAction             `json:"timeoutAction" bun:"timeout_action"`
	TimeoutNotifyBeforeHours  int                       `json:"timeoutNotifyBeforeHours" bun:"timeout_notify_before_hours"`
	UrgeCooldownMinutes       int                       `json:"urgeCooldownMinutes" bun:"urge_cooldown_minutes"`
	ConsecutiveApproverAction ConsecutiveApproverAction `json:"consecutiveApproverAction" bun:"consecutive_approver_action"`
	IsReadConfirmRequired     bool                      `json:"isReadConfirmRequired" bun:"is_read_confirm_required"`
	Branches                  []ConditionBranch         `json:"branches" bun:"branches,type:jsonb,nullzero"`
}

// FlowEdge represents a directed edge between two flow nodes.
type FlowEdge struct {
	orm.BaseModel `bun:"table:apv_flow_edge,alias:afe"`
	orm.IDModel

	FlowVersionID string  `json:"flowVersionId" bun:"flow_version_id"`
	Key           string  `json:"key" bun:"key,nullzero"`
	SourceNodeID  string  `json:"sourceNodeId" bun:"source_node_id"`
	SourceNodeKey string  `json:"sourceNodeKey" bun:"source_node_key"`
	TargetNodeID  string  `json:"targetNodeId" bun:"target_node_id"`
	TargetNodeKey string  `json:"targetNodeKey" bun:"target_node_key"`
	SourceHandle  *string `json:"sourceHandle" bun:"source_handle,nullzero"`
}

// FlowNodeAssignee represents a node assignee configuration.
type FlowNodeAssignee struct {
	orm.BaseModel `bun:"table:apv_flow_node_assignee,alias:afna"`
	orm.IDModel

	NodeID    string       `json:"nodeId" bun:"node_id"`
	Kind      AssigneeKind `json:"kind" bun:"kind"`
	IDs       []string     `json:"ids" bun:"ids,type:jsonb"`
	FormField *string      `json:"formField" bun:"form_field,nullzero"`
	SortOrder int          `json:"sortOrder" bun:"sort_order"`
}

// FlowNodeCC represents a node CC configuration.
type FlowNodeCC struct {
	orm.BaseModel `bun:"table:apv_flow_node_cc,alias:afnc"`
	orm.IDModel

	NodeID    string   `json:"nodeId" bun:"node_id"`
	Kind      CCKind   `json:"kind" bun:"kind"`
	IDs       []string `json:"ids" bun:"ids,type:jsonb"`
	FormField *string  `json:"formField" bun:"form_field,nullzero"`
	Timing    CCTiming `json:"timing" bun:"timing"`
}

// FlowFormField represents a flow form field definition.
type FlowFormField struct {
	orm.BaseModel `bun:"table:apv_flow_form_field,alias:afff"`
	orm.IDModel

	FlowVersionID string         `json:"flowVersionId" bun:"flow_version_id"`
	Name          string         `json:"name" bun:"name"`
	Kind          FieldKind      `json:"kind" bun:"kind"`
	Label         string         `json:"label" bun:"label"`
	Placeholder   *string        `json:"placeholder" bun:"placeholder,nullzero"`
	DefaultValue  *string        `json:"defaultValue" bun:"default_value,nullzero"`
	IsRequired    *bool          `json:"isRequired" bun:"is_required,nullzero"`
	IsReadonly    *bool          `json:"isReadonly" bun:"is_readonly,nullzero"`
	Validation    map[string]any `json:"validation" bun:"validation,type:jsonb,nullzero"`
	SortOrder     int            `json:"sortOrder" bun:"sort_order"`
	Meta          map[string]any `json:"meta" bun:"meta,type:jsonb,nullzero"`
}

// FlowInitiator represents a flow initiator configuration.
type FlowInitiator struct {
	orm.BaseModel `bun:"table:apv_flow_initiator,alias:afi"`
	orm.IDModel

	FlowID string        `json:"flowId" bun:"flow_id"`
	Kind   InitiatorKind `json:"kind" bun:"kind"`
	IDs    []string      `json:"ids" bun:"ids,type:jsonb"`
}

// ── Instance & Task ─────────────────────────────────────────────────────────

// Instance represents a flow instance.
type Instance struct {
	orm.BaseModel `bun:"table:apv_instance,alias:ai"`
	orm.Model

	TenantID          string          `json:"tenantId" bun:"tenant_id"`
	FlowID            string          `json:"flowId" bun:"flow_id"`
	FlowVersionID     string          `json:"flowVersionId" bun:"flow_version_id"`
	Title             string          `json:"title" bun:"title"`
	InstanceNo        string          `json:"instanceNo" bun:"instance_no"`
	ApplicantID       string          `json:"applicantId" bun:"applicant_id"`
	ApplicantName     string          `json:"applicantName" bun:"applicant_name"`
	ApplicantDeptID   *string         `json:"applicantDeptId" bun:"applicant_dept_id,nullzero"`
	ApplicantDeptName *string         `json:"applicantDeptName" bun:"applicant_dept_name,nullzero"`
	Status            InstanceStatus  `json:"status" bun:"status"`
	CurrentNodeID     *string         `json:"currentNodeId" bun:"current_node_id,nullzero"`
	FinishedAt        *timex.DateTime `json:"finishedAt" bun:"finished_at,nullzero"`
	BusinessRecordID  *string         `json:"businessRecordId" bun:"business_record_id,nullzero"`
	FormData          map[string]any  `json:"formData" bun:"form_data,type:jsonb,nullzero"`
}

// Task represents an approval task.
type Task struct {
	orm.BaseModel `bun:"table:apv_task,alias:at"`
	orm.Model

	TenantID         string           `json:"tenantId" bun:"tenant_id"`
	InstanceID       string           `json:"instanceId" bun:"instance_id"`
	NodeID           string           `json:"nodeId" bun:"node_id"`
	AssigneeID       string           `json:"assigneeId" bun:"assignee_id"`
	AssigneeName     string           `json:"assigneeName" bun:"assignee_name"`
	DelegatorID      *string          `json:"delegatorId" bun:"delegator_id,nullzero"`
	DelegatorName    *string          `json:"delegatorName" bun:"delegator_name,nullzero"`
	SortOrder        int              `json:"sortOrder" bun:"sort_order"`
	Status           TaskStatus       `json:"status" bun:"status"`
	ReadAt           *timex.DateTime  `json:"readAt" bun:"read_at,nullzero"`
	ParentTaskID     *string          `json:"parentTaskId" bun:"parent_task_id,nullzero"`
	AddAssigneeType  *AddAssigneeType `json:"addAssigneeType" bun:"add_assignee_type,nullzero"`
	Deadline         *timex.DateTime  `json:"deadline" bun:"deadline,nullzero"`
	IsTimeout        bool             `json:"isTimeout" bun:"is_timeout"`
	IsPreWarningSent bool             `json:"isPreWarningSent" bun:"is_pre_warning_sent"`
	FinishedAt       *timex.DateTime  `json:"finishedAt" bun:"finished_at,nullzero"`
}

// FormSnapshot represents a form snapshot for rollback strategies.
type FormSnapshot struct {
	orm.BaseModel `bun:"table:apv_form_snapshot,alias:afs"`
	orm.IDModel
	orm.CreatedModel

	InstanceID string         `json:"instanceId" bun:"instance_id"`
	NodeID     string         `json:"nodeId" bun:"node_id"`
	FormData   map[string]any `json:"formData" bun:"form_data,type:jsonb"`
}

// ── Records & Logs ──────────────────────────────────────────────────────────

// ActionLog represents an action log entry.
type ActionLog struct {
	orm.BaseModel `bun:"table:apv_action_log,alias:aal"`
	orm.IDModel
	orm.CreatedModel

	InstanceID         string           `json:"instanceId" bun:"instance_id"`
	NodeID             *string          `json:"nodeId" bun:"node_id,nullzero"`
	TaskID             *string          `json:"taskId" bun:"task_id,nullzero"`
	Action             ActionType       `json:"action" bun:"action"`
	OperatorID         string           `json:"operatorId" bun:"operator_id"`
	OperatorName       string           `json:"operatorName" bun:"operator_name"`
	OperatorDeptID     *string          `json:"operatorDeptId" bun:"operator_dept_id,nullzero"`
	OperatorDeptName   *string          `json:"operatorDeptName" bun:"operator_dept_name,nullzero"`
	IPAddress          *string          `json:"ipAddress" bun:"ip_address,nullzero"`
	UserAgent          *string          `json:"userAgent" bun:"user_agent,nullzero"`
	Opinion            *string          `json:"opinion" bun:"opinion,nullzero"`
	TransferToID       *string          `json:"transferToId" bun:"transfer_to_id,nullzero"`
	TransferToName     *string          `json:"transferToName" bun:"transfer_to_name,nullzero"`
	RollbackToNodeID   *string          `json:"rollbackToNodeId" bun:"rollback_to_node_id,nullzero"`
	AddAssigneeType    *AddAssigneeType `json:"addAssigneeType" bun:"add_assignee_type,nullzero"`
	AddedAssigneeIDs   []string         `json:"addedAssigneeIds" bun:"added_assignee_ids,type:jsonb"`
	RemovedAssigneeIDs []string         `json:"removedAssigneeIds" bun:"removed_assignee_ids,type:jsonb"`
	CCUserIDs          []string         `json:"ccUserIds" bun:"cc_user_ids,type:jsonb"`
	Attachments        []string         `json:"attachments" bun:"attachments,type:jsonb,nullzero"`
	Meta               map[string]any   `json:"meta" bun:"meta,type:jsonb,nullzero"`
}

// CCRecord represents a CC notification record.
type CCRecord struct {
	orm.BaseModel `bun:"table:apv_cc_record,alias:acr"`
	orm.IDModel
	orm.CreatedModel

	InstanceID string          `json:"instanceId" bun:"instance_id"`
	NodeID     *string         `json:"nodeId" bun:"node_id,nullzero"`
	TaskID     *string         `json:"taskId" bun:"task_id,nullzero"`
	CCUserID   string          `json:"ccUserId" bun:"cc_user_id"`
	CCUserName string          `json:"ccUserName" bun:"cc_user_name"`
	IsManual   bool            `json:"isManual" bun:"is_manual"`
	ReadAt     *timex.DateTime `json:"readAt" bun:"read_at,nullzero"`
}

// ParallelRecord represents a parallel approval decision record.
type ParallelRecord struct {
	orm.BaseModel `bun:"table:apv_parallel_record,alias:apr"`
	orm.IDModel
	orm.CreatedModel

	InstanceID string  `json:"instanceId" bun:"instance_id"`
	NodeID     string  `json:"nodeId" bun:"node_id"`
	TaskID     string  `json:"taskId" bun:"task_id"`
	AssigneeID string  `json:"assigneeId" bun:"assignee_id"`
	Decision   *string `json:"decision" bun:"decision,nullzero"`
	Opinion    *string `json:"opinion" bun:"opinion,nullzero"`
}

// Delegation represents an approval delegation.
type Delegation struct {
	orm.BaseModel `bun:"table:apv_delegation,alias:ad"`
	orm.Model

	DelegatorID    string         `json:"delegatorId" bun:"delegator_id"`
	DelegateeID    string         `json:"delegateeId" bun:"delegatee_id"`
	FlowCategoryID *string        `json:"flowCategoryId" bun:"flow_category_id,nullzero"`
	FlowID         *string        `json:"flowId" bun:"flow_id,nullzero"`
	StartTime      timex.DateTime `json:"startTime" bun:"start_time"`
	EndTime        timex.DateTime `json:"endTime" bun:"end_time"`
	IsActive       bool           `json:"isActive" bun:"is_active"`
	Reason         *string        `json:"reason" bun:"reason,nullzero"`
}

// EventOutbox represents an event outbox entry for transactional event publishing.
type EventOutbox struct {
	orm.BaseModel `bun:"table:apv_event_outbox,alias:aeo"`
	orm.IDModel
	orm.CreatedModel

	EventID     string            `json:"eventId" bun:"event_id"`
	EventType   string            `json:"eventType" bun:"event_type"`
	Payload     map[string]any    `json:"payload" bun:"payload,type:jsonb"`
	Status      EventOutboxStatus `json:"status" bun:"status"`
	RetryCount  int               `json:"retryCount" bun:"retry_count"`
	LastError   *string           `json:"lastError" bun:"last_error,nullzero"`
	ProcessedAt *timex.DateTime   `json:"processedAt" bun:"processed_at,nullzero"`
	RetryAfter  *timex.DateTime   `json:"retryAfter" bun:"retry_after,nullzero"`
}

// UrgeRecord represents an urge/reminder record.
type UrgeRecord struct {
	orm.BaseModel `bun:"table:apv_urge_record,alias:aur"`
	orm.IDModel
	orm.CreatedModel

	InstanceID     string  `json:"instanceId" bun:"instance_id"`
	NodeID         string  `json:"nodeId" bun:"node_id"`
	TaskID         *string `json:"taskId" bun:"task_id,nullzero"`
	UrgerID        string  `json:"urgerId" bun:"urger_id"`
	UrgerName      string  `json:"urgerName" bun:"urger_name"`
	TargetUserID   string  `json:"targetUserId" bun:"target_user_id"`
	TargetUserName string  `json:"targetUserName" bun:"target_user_name"`
	Message        string  `json:"message" bun:"message"`
}
