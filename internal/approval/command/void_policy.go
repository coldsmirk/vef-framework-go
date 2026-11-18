package command

import (
	"context"
	"errors"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/business"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/orm"
	"strings"
)

func normalizeVoidPolicy(ctx context.Context, db orm.DB, registry *business.Registry, tenant string, kind *approval.BusinessKind, policy approval.VoidPolicy, flowCode *string) (approval.VoidPolicy, error) {
	if policy == "" {
		policy = approval.VoidDisabled
	}
	if policy == approval.VoidDisabled && flowCode == nil {
		return policy, nil
	}
	switch policy {
	case approval.VoidDirect:
		if flowCode != nil {
			return "", approval.ErrVoidPolicyInvalid
		}
	case approval.VoidApproval:
		if flowCode == nil || strings.TrimSpace(*flowCode) == "" {
			return "", approval.ErrVoidPolicyInvalid
		}
		flow, err := service.FindStartFlow(ctx, db, tenant, *flowCode)
		if errors.Is(err, approval.ErrFlowNotFound) {
			return "", approval.ErrVoidPolicyInvalid
		}
		if err != nil {
			return "", err
		}
		if flow.BusinessKind == nil || *flow.BusinessKind != approval.BusinessKindVoidRequest {
			return "", approval.ErrVoidPolicyInvalid
		}
	default:
		return "", approval.ErrVoidPolicyInvalid
	}
	if kind != nil {
		handler, _ := registry.Lookup(*kind)
		if _, ok := handler.(approval.BusinessVoider); !ok || *kind == approval.BusinessKindVoidRequest {
			return "", approval.ErrVoidPolicyInvalid
		}
	}
	return policy, nil
}
