package resource

import (
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/crud"
	"github.com/coldsmirk/vef-framework-go/integration"
)

// LogSearch contains the search parameters for invocation logs.
type LogSearch struct {
	crud.Sortable

	SystemCode   string `json:"systemCode" search:"eq,column=system_code"`
	ContractCode string `json:"contractCode" search:"eq,column=contract_code"`
	FailureKind  string `json:"failureKind" search:"eq,column=failure_kind"`
	RequestID    string `json:"requestId" search:"eq,column=request_id"`
}

// LogResource exposes the invocation log read-only: the page view for
// browsing and the single-record view for the full captures.
type LogResource struct {
	api.Resource

	crud.FindPage[integration.InvocationLog, LogSearch]
	crud.FindOne[integration.InvocationLog, LogSearch]
}

// NewLogResource creates the invocation log resource.
func NewLogResource() api.Resource {
	return &LogResource{
		Resource: api.NewRPCResource("integration/log"),
		FindPage: crud.NewFindPage[integration.InvocationLog, LogSearch]().
			RequiredPermission("integration.log.query"),
		FindOne: crud.NewFindOne[integration.InvocationLog, LogSearch]().
			RequiredPermission("integration.log.query"),
	}
}
