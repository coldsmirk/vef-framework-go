package auth

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/security"
)

// DefaultInstanceGlobalsResolver resolves no globals: flows route on form
// data and the built-in applicant subjects only. Hosts whose flows reference
// global variables (tenant attributes, applicant roles, business limits, …)
// replace it via fx.Replace with a resolver that derives them server-side
// from their own principal / business context.
type DefaultInstanceGlobalsResolver struct{}

// NewDefaultInstanceGlobalsResolver constructs the default resolver.
func NewDefaultInstanceGlobalsResolver() approval.InstanceGlobalsResolver {
	return new(DefaultInstanceGlobalsResolver)
}

// Resolve returns no globals.
func (*DefaultInstanceGlobalsResolver) Resolve(context.Context, *security.Principal, string) (map[string]any, error) {
	return nil, nil
}
