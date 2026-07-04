package binding

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// identityResolver is the default BusinessRefResolver: the ref is taken to
// be the business primary key itself. Hosts whose refs are composite (e.g.
// JSON) replace it via vef.SupplyBusinessRefResolver.
type identityResolver struct{}

// NewIdentityResolver constructs the default identity resolver.
func NewIdentityResolver() approval.BusinessRefResolver { return new(identityResolver) }

// ResolveRecordID returns businessRef verbatim.
func (*identityResolver) ResolveRecordID(_ context.Context, _ *approval.Flow, businessRef string) (string, error) {
	return businessRef, nil
}
