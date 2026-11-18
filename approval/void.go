package approval

import (
	"context"
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type VoidPolicy string

const (
	VoidDisabled            VoidPolicy   = "disabled"
	VoidDirect              VoidPolicy   = "direct"
	VoidApproval            VoidPolicy   = "approval"
	BusinessKindVoidRequest BusinessKind = "approval.void_request"
)

// BusinessVoider reverses the effects of an approved business instance.
type BusinessVoider interface {
	// PlanVoid describes blockers, reversals and manual follow-ups without writing.
	PlanVoid(ctx context.Context, db orm.DB, vc *VoidContext) (*VoidPlan, error)
	// Void reverses the effects in the transaction before the instance status changes.
	Void(ctx context.Context, db orm.DB, vc *VoidContext) error
}

type VoidContext struct {
	Instance *Instance
	Flow     *Flow
	Operator UserInfo
	Reason   string
	Options  map[string]any
	Request  *Instance
}

type VoidPlan struct {
	Blockers  []string `json:"blockers"`
	Reversals []string `json:"reversals"`
	FollowUps []string `json:"followUps"`
}

func (p *VoidPlan) IsBlocked() bool { return len(p.Blockers) > 0 }

// MarshalJSON represents an empty list of effects as an array on every wire path.
func (p VoidPlan) MarshalJSON() ([]byte, error) {
	type wire VoidPlan
	if p.Blockers == nil {
		p.Blockers = []string{}
	}
	if p.Reversals == nil {
		p.Reversals = []string{}
	}
	if p.FollowUps == nil {
		p.FollowUps = []string{}
	}
	return json.Marshal(wire(p))
}

type VoidStatus string

const (
	VoidRequested VoidStatus = "requested"
	VoidVoided    VoidStatus = "voided"
	VoidDismissed VoidStatus = "dismissed"
)

type VoidRecord struct {
	orm.BaseModel `bun:"table:apv_void_record,alias:avr"`
	orm.FullAuditedModel

	TenantID          string          `json:"tenantId"`
	InstanceID        string          `json:"instanceId"`
	Status            VoidStatus      `json:"status"`
	RequestInstanceID *string         `json:"requestInstanceId,omitempty"`
	Operator          UserInfo        `json:"operator" bun:"embed:operator_"`
	Reason            string          `json:"reason"`
	Attachments       []string        `json:"attachments,omitempty" bun:"type:jsonb,nullzero"`
	Options           map[string]any  `json:"options,omitempty" bun:"type:jsonb,nullzero"`
	Reversals         []string        `json:"reversals" bun:"type:jsonb"`
	FollowUps         []string        `json:"followUps" bun:"type:jsonb"`
	VoidedAt          *timex.DateTime `json:"voidedAt,omitempty"`
}

// VoidRecordView is the executed reversal shown on the target instance.
type VoidRecordView = VoidRecord

// VoidRequestView identifies the pending request and its current workflow state.
type VoidRequestView struct {
	RequestInstanceID string         `json:"requestInstanceId"`
	RequestInstanceNo string         `json:"requestInstanceNo"`
	RequestStatus     InstanceStatus `json:"requestStatus"`
	Applicant         UserInfo       `json:"applicant"`
	CreatedAt         timex.DateTime `json:"createdAt"`
}

type VoidInstanceInput struct {
	InstanceID  string
	Operator    UserInfo
	Reason      string
	Attachments []string
	Options     map[string]any
	Caller      CallerContext
}

// VoidPreview describes a direct void or a request under the flow's live policy.
type VoidPreview struct {
	Policy       VoidPolicy `json:"policy"`
	VoidFlowCode *string    `json:"voidFlowCode,omitempty"`
	Plan         *VoidPlan  `json:"plan"`
}

type InstanceVoidedEvent struct {
	InstanceEventBase
	Operator          UserInfo `json:"operator"`
	Reason            string   `json:"reason"`
	RequestInstanceID *string  `json:"requestInstanceId,omitempty"`
	Reversals         []string `json:"reversals"`
	FollowUps         []string `json:"followUps"`
}

func (*InstanceVoidedEvent) EventType() string { return EventTypeInstanceVoided }
