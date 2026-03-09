package security

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/log"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// RequestScopedDataPermApplier is the default implementation of DataPermissionApplier.
// It applies data permission filtering using a single DataScope instance.
//
// IMPORTANT: This struct is request-scoped and should NOT be stored beyond request lifecycle.
type RequestScopedDataPermApplier struct {
	principal *Principal
	dataScope DataScope
	logger    log.Logger
}

// NewRequestScopedDataPermApplier creates a new request-scoped data permission applier.
// This function is typically called by the data permission middleware for each request.
func NewRequestScopedDataPermApplier(
	principal *Principal,
	dataScope DataScope,
	logger log.Logger,
) DataPermissionApplier {
	return &RequestScopedDataPermApplier{
		principal: principal,
		dataScope: dataScope,
		logger:    logger,
	}
}

// Apply implements security.DataPermissionApplier.Apply.
func (a *RequestScopedDataPermApplier) Apply(query orm.SelectQuery) error {
	if a.dataScope == nil {
		a.logger.Debugf("No data scope configured, skipping data permission")

		return nil
	}

	queryBuilder, ok := query.(orm.QueryBuilder)
	if !ok {
		return ErrQueryNotQueryBuilder
	}

	table := queryBuilder.GetTable()
	if table == nil {
		return ErrQueryModelNotSet
	}

	if !a.dataScope.Supports(a.principal, table) {
		a.logger.Debugf(
			"Data scope %q is not applicable to table %q, skipping data permission",
			a.dataScope.Key(), table.TypeName)

		return nil
	}

	if err := a.dataScope.Apply(a.principal, query); err != nil {
		return fmt.Errorf("failed to apply data scope %q: %w", a.dataScope.Key(), err)
	}

	a.logger.Debugf("Applied data scope %q to table %q", a.dataScope.Key(), table.TypeName)

	return nil
}
