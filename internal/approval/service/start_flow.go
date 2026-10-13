package service

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// StartTenant resolves the tenant an instance the caller starts runs in, which
// is also the tenant whose flows — its own plus the shared ones — the caller is
// offered: the tenant CallerContext.ResolveWriteTenant stamps a new instance
// with, except that the shared tenant is refused with
// approval.ErrCrossTenantAccess — it holds definitions, never instances.
func StartTenant(caller approval.CallerContext, requested string) (string, error) {
	tenantID, err := caller.ResolveWriteTenant(requested)
	if err != nil {
		return "", err
	}

	if tenantID == approval.SharedTenantID {
		return "", approval.ErrCrossTenantAccess
	}

	return tenantID, nil
}

// FindStartFlow loads the flow code names in tenantID: the tenant's own flow
// with that code, or else the shared flow with that code. Existence decides,
// not state — an inactive tenant flow still hides the shared one — so a tenant
// replaces a shared flow by defining its own, and the available-flow list, the
// start form and the start agree on which flow a code names in a tenant.
func FindStartFlow(ctx context.Context, db orm.DB, tenantID, code string) (*approval.Flow, error) {
	var flows []approval.Flow

	if err := db.NewSelect().
		Model(&flows).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("code", code).
				In("tenant_id", []string{tenantID, approval.SharedTenantID})
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
