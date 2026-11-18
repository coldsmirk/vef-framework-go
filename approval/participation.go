package approval

import (
	"context"
	"slices"

	"github.com/coldsmirk/vef-framework-go/orm"
)

type ParticipantRole string

const (
	ParticipantApplicant   ParticipantRole = "applicant"
	ParticipantAssignee    ParticipantRole = "assignee" // any task's assignee, any status
	ParticipantDelegator   ParticipantRole = "delegator"
	ParticipantCCRecipient ParticipantRole = "cc_recipient"
)

type Participation struct {
	Roles []ParticipantRole `json:"roles"`
	// ActionableTaskIDs are the tasks the person can decide now: pending, on
	// the current node, assigned to them.
	ActionableTaskIDs []string `json:"actionableTaskIds"`
}

func (p *Participation) Has(role ParticipantRole) bool {
	return p != nil && slices.Contains(p.Roles, role)
}
func (p *Participation) IsParticipant() bool { return p != nil && len(p.Roles) > 0 }
func (p *Participation) CanAct(taskID string) bool {
	return p != nil && slices.Contains(p.ActionableTaskIDs, taskID)
}

type ParticipationReader interface {
	// Participation reports every way in.UserID takes part in the instance.
	// A missing instance, or one outside the caller's tenant, is ErrInstanceNotFound.
	Participation(ctx context.Context, db orm.DB, in ParticipationInput) (*Participation, error)
}
type ParticipationInput struct {
	InstanceID, UserID string
	Caller             CallerContext
}
