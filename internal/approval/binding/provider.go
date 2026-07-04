package binding

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// noopRefProvider is the default BusinessRefProvider. The framework cannot
// create a business row generically, so it binds nothing — callers either
// pass businessRef in the start parameters or hosts replace this provider
// via vef.SupplyBusinessRefProvider to allocate the row inside the
// start_instance transaction.
type noopRefProvider struct{}

// NewNoopRefProvider constructs the default no-op provider.
func NewNoopRefProvider() approval.BusinessRefProvider { return new(noopRefProvider) }

// OnInstanceCreated returns an empty ref: nothing to bind.
func (*noopRefProvider) OnInstanceCreated(context.Context, orm.DB, *approval.Flow, *approval.Instance) (string, error) {
	return "", nil
}
