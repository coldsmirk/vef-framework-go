package service

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/security"
)

// FormGlobalsService decides which host globals an instance's form may read:
// those the registered approval.InstanceGlobalsResolver names through the
// optional approval.FormGlobalsExposer, and none otherwise.
type FormGlobalsService struct {
	resolver approval.InstanceGlobalsResolver
}

// NewFormGlobalsService creates a service over the registered globals resolver.
func NewFormGlobalsService(resolver approval.InstanceGlobalsResolver) *FormGlobalsService {
	return &FormGlobalsService{resolver: resolver}
}

func (s *FormGlobalsService) keys(flowCode string) []string {
	exposer, ok := s.resolver.(approval.FormGlobalsExposer)
	if !ok {
		return nil
	}

	return exposer.FormGlobalKeys(flowCode)
}

// Visible returns the part of globals the form of an instance of flowCode may
// read, or nil when there is none.
func (s *FormGlobalsService) Visible(flowCode string, globals map[string]any) map[string]any {
	keys := s.keys(flowCode)
	if len(keys) == 0 || len(globals) == 0 {
		return nil
	}

	visible := make(map[string]any, len(keys))

	for _, key := range keys {
		if value, ok := globals[key]; ok {
			visible[key] = value
		}
	}

	if len(visible) == 0 {
		return nil
	}

	return visible
}

// Resolve returns the form-visible part of the globals a new instance of
// flowCode started by principal would snapshot — the start form's view of
// what Start will persist. The resolver is not called at all when the flow
// exposes nothing, so hosts that never opt in pay nothing for the start form.
func (s *FormGlobalsService) Resolve(ctx context.Context, principal *security.Principal, flowCode string) (map[string]any, error) {
	if len(s.keys(flowCode)) == 0 {
		return nil, nil
	}

	globals, err := s.resolver.Resolve(ctx, principal, flowCode)
	if err != nil {
		return nil, fmt.Errorf("resolve form globals: %w", err)
	}

	return s.Visible(flowCode, globals), nil
}
