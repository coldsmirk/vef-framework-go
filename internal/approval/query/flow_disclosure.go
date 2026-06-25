package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// authorizeFlowDisclosure loads a flow's tenant and applies the
// authorize-before-disclose gate shared by the tenant-scoped flow sub-resource
// queries (versions, initiators). It returns ok=false with a nil error when the
// flow is missing, belongs to another tenant, or fails the optional tenant
// pre-filter — a single indistinguishable "no data" outcome so the API never
// reveals cross-tenant existence to a probing caller. A non-nil error is a real
// infrastructure failure, not an authorization result.
func authorizeFlowDisclosure(
	ctx context.Context,
	db orm.DB,
	flowID string,
	tenantID *string,
	caller approval.CallerContext,
) (bool, error) {
	var flow approval.Flow

	flow.ID = flowID

	err := db.NewSelect().
		Model(&flow).
		Select("tenant_id").
		WherePK().
		Scan(ctx)
	switch {
	case result.IsRecordNotFound(err):
		return false, nil

	case err != nil:
		return false, fmt.Errorf("load flow for tenant check: %w", err)
	}

	if authErr := caller.Authorize(flow.TenantID); authErr != nil {
		// Intentionally swallowed: an out-of-tenant flow is indistinguishable
		// from a missing one so a probing caller cannot detect cross-tenant
		// existence.
		return false, nil //nolint:nilerr // tenant isolation requires opaque response
	}

	if tenantID != nil && *tenantID != flow.TenantID {
		return false, nil
	}

	return true, nil
}
