package query

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// ListKindOptionsQuery asks for the assignee / CC / initiator kinds this
// application accepts — what the flow designer offers in its rule editors.
type ListKindOptionsQuery struct {
	cqrs.BaseQuery
}

// ListKindOptionsHandler handles the ListKindOptionsQuery.
type ListKindOptionsHandler struct {
	registry *strategy.StrategyRegistry
}

// NewListKindOptionsHandler creates a new ListKindOptionsHandler.
func NewListKindOptionsHandler(registry *strategy.StrategyRegistry) *ListKindOptionsHandler {
	return &ListKindOptionsHandler{registry: registry}
}

// Handle reads the catalog straight off the boot-registered resolvers, so it
// reflects host registrations and overrides with no second list to maintain.
// Labels are resolved per call rather than cached, so they follow the
// configured language.
func (h *ListKindOptionsHandler) Handle(context.Context, ListKindOptionsQuery) (*approval.KindOptions, error) {
	return &approval.KindOptions{
		Assignees:  h.registry.Assignees().Descriptors(),
		CCs:        h.registry.CCs().Descriptors(),
		Initiators: h.registry.Initiators().Descriptors(),
	}, nil
}
