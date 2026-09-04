package command

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// RetryBusinessProjectionCmd immediately retries one eventual business
// projection.
type RetryBusinessProjectionCmd struct {
	cqrs.BaseCommand
	approval.RetryBusinessProjectionInput
}

// RetryBusinessProjectionHandler handles manual projection retries.
type RetryBusinessProjectionHandler struct {
	db     orm.DB
	worker *binding.Worker
}

// NewRetryBusinessProjectionHandler creates a manual retry handler.
func NewRetryBusinessProjectionHandler(db orm.DB, worker *binding.Worker) *RetryBusinessProjectionHandler {
	return &RetryBusinessProjectionHandler{db: db, worker: worker}
}

func (h *RetryBusinessProjectionHandler) Handle(ctx context.Context, cmd RetryBusinessProjectionCmd) (cqrs.Unit, error) {
	db := contextx.DB(ctx, h.db)

	projection := new(approval.BusinessProjection)

	projection.ID = cmd.ProjectionID
	if err := db.NewSelect().Model(projection).WherePK().ForUpdate().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return cqrs.Unit{}, approval.ErrBindingProjectionNotFound
		}

		return cqrs.Unit{}, fmt.Errorf("load business projection: %w", err)
	}

	if err := cmd.Caller.Authorize(projection.TenantID); err != nil {
		return cqrs.Unit{}, approval.ErrBindingProjectionNotFound
	}

	if err := h.worker.Retry(ctx, db, projection); err != nil {
		return cqrs.Unit{}, fmt.Errorf("retry business projection: %w", err)
	}

	return cqrs.Unit{}, nil
}
