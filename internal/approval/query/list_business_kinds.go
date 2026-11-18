package query

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

type (
	ListBusinessKindsQuery   struct{ cqrs.BaseQuery }
	ListBusinessKindsHandler struct{ registry *business.Registry }
)

func NewListBusinessKindsHandler(r *business.Registry) *ListBusinessKindsHandler {
	return &ListBusinessKindsHandler{registry: r}
}

func (h *ListBusinessKindsHandler) Handle(context.Context, ListBusinessKindsQuery) ([]approval.BusinessKindDescriptor, error) {
	return h.registry.Descriptors(), nil
}
