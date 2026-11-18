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

type BusinessKindOption struct {
	approval.BusinessKindDescriptor

	IsVoidable bool `json:"isVoidable"`
}

func (h *ListBusinessKindsHandler) Handle(context.Context, ListBusinessKindsQuery) ([]BusinessKindOption, error) {
	ds := h.registry.Descriptors()

	options := make([]BusinessKindOption, len(ds))
	for i, d := range ds {
		_, ok := h.registry.Voider(d.Kind)
		options[i] = BusinessKindOption{BusinessKindDescriptor: d, IsVoidable: ok}
	}

	return options, nil
}
