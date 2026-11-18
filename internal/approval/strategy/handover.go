package strategy

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
)

type noHandovers struct{}

func (noHandovers) ResolveHandovers(context.Context, *approval.HandoverContext, []string) (map[string]approval.Handover, error) {
	return nil, nil
}

func newHandoverResolver() approval.HandoverResolver { return noHandovers{} }
