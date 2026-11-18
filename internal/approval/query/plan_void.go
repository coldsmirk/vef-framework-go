package query

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/voiding"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type PlanVoidQuery struct {
	cqrs.BaseQuery
	InstanceID string
	Operator   approval.UserInfo
	Options    map[string]any
	Caller     approval.CallerContext
	Policy     approval.VoidPolicy
}
type PlanVoidHandler struct {
	db         orm.DB
	core       *voiding.Core
	validation *service.ValidationService
}

func NewPlanVoidHandler(db orm.DB, c *voiding.Core, v *service.ValidationService) *PlanVoidHandler {
	return &PlanVoidHandler{db: db, core: c, validation: v}
}
func (h *PlanVoidHandler) Handle(ctx context.Context, q PlanVoidQuery) (*approval.VoidPreview, error) {
	return h.core.Preview(ctx, contextx.DB(ctx, h.db), approval.VoidInstanceInput{InstanceID: q.InstanceID, Options: q.Options, Operator: q.Operator, Caller: q.Caller}, q.Policy, h.validation)
}
