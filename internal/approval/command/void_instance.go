package command

import (
	"context"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/voiding"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type VoidInstanceCmd struct {
	cqrs.BaseCommand
	approval.VoidInstanceInput
}
type VoidInstanceHandler struct {
	db   orm.DB
	core *voiding.Core
}

func NewVoidInstanceHandler(db orm.DB, core *voiding.Core) *VoidInstanceHandler {
	return &VoidInstanceHandler{db: db, core: core}
}
func (h *VoidInstanceHandler) Handle(ctx context.Context, cmd VoidInstanceCmd) (cqrs.Unit, error) {
	return cqrs.Unit{}, h.core.Direct(ctx, contextx.DB(ctx, h.db), cmd.VoidInstanceInput)
}
