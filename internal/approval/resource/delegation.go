package resource

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/crud"
)

// DelegationParams contains the create/update parameters for delegation.
type DelegationParams struct {
	api.P

	ID             string     `json:"id"`
	DelegatorID    string     `json:"delegatorId" validate:"required"`
	DelegateeID    string     `json:"delegateeId" validate:"required"`
	FlowCategoryID *string    `json:"flowCategoryId"`
	FlowID         *string    `json:"flowId"`
	StartTime      *time.Time `json:"startTime"`
	EndTime        *time.Time `json:"endTime"`
	IsActive       bool       `json:"isActive"`
	Reason         *string    `json:"reason"`
}

// DelegationSearch contains the search parameters for delegation.
type DelegationSearch struct {
	crud.Sortable

	DelegatorID string `json:"delegatorId" search:"eq,column=delegator_id"`
	DelegateeID string `json:"delegateeId" search:"eq,column=delegatee_id"`
	IsActive    *bool  `json:"isActive" search:"eq,column=is_active"`
}

// DelegationResource handles delegation CRUD using standard apis generics.
type DelegationResource struct {
	api.Resource

	crud.FindPage[approval.Delegation, DelegationSearch]
	crud.Create[approval.Delegation, DelegationParams]
	crud.Update[approval.Delegation, DelegationParams]
	crud.Delete[approval.Delegation]
}

// NewDelegationResource creates a new delegation resource with standard CRUD operations.
func NewDelegationResource() api.Resource {
	return &DelegationResource{
		Resource: api.NewRPCResource("approval/delegation"),
		FindPage: crud.NewFindPage[approval.Delegation, DelegationSearch]().PermToken("approval:delegation:query"),
		Create:   crud.NewCreate[approval.Delegation, DelegationParams]().PermToken("approval:delegation:create"),
		Update:   crud.NewUpdate[approval.Delegation, DelegationParams]().PermToken("approval:delegation:update"),
		Delete:   crud.NewDelete[approval.Delegation]().PermToken("approval:delegation:delete"),
	}
}
