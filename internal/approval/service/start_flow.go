package service

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// FindStartFlow loads the flow code names in tenantID: the tenant's own flow
// with that code, or else the shared flow with that code, whichever tenant
// owns it. Existence decides, not state — an inactive tenant flow still hides
// the shared one — so a tenant replaces a shared flow by defining its own, and
// the available-flow list, the start form and the start agree on which flow a
// code names in a tenant. A shared flow's code is unique among the shared
// flows of every tenant, so at most one shared flow answers.
func FindStartFlow(ctx context.Context, db orm.DB, tenantID, code string) (*approval.Flow, error) {
	var flows []approval.Flow

	if err := db.NewSelect().
		Model(&flows).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("code", code).
				Group(func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", tenantID).
						OrIsTrue("is_shared")
				})
		}).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load flow: %w", err)
	}

	var sharedFlow *approval.Flow

	for i := range flows {
		if flows[i].TenantID == tenantID {
			return &flows[i], nil
		}

		sharedFlow = &flows[i]
	}

	if sharedFlow == nil {
		return nil, approval.ErrFlowNotFound
	}

	return sharedFlow, nil
}
