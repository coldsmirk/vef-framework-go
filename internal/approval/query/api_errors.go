package query

import (
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/result"
)

var errConflictingFlowFilters = result.Err(i18n.T("approval_flow_filters_conflict"), result.WithCode(result.ErrCodeBadRequest))
