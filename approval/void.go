package approval

import (
	"context"
	"encoding/json"

	"github.com/coldsmirk/vef-framework-go/orm"
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
