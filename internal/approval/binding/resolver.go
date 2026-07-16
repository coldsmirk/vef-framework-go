package binding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// identityResolver is the default BusinessRefResolver. Single-column refs are
// taken verbatim; composite refs are decoded from a JSON object.
type identityResolver struct{}

// NewIdentityResolver constructs the default identity resolver.
func NewIdentityResolver() approval.BusinessRefResolver { return new(identityResolver) }

func (*identityResolver) ResolveRecordKey(_ context.Context, flow *approval.Flow, businessRef string) (approval.BusinessRecordKey, error) {
	if flow.BusinessBinding == nil || len(flow.BusinessBinding.KeyColumns) == 0 {
		return nil, fmt.Errorf("%w: flow has no key columns", ErrBindingMisconfigured)
	}

	if len(flow.BusinessBinding.KeyColumns) == 1 {
		return approval.BusinessRecordKey{flow.BusinessBinding.KeyColumns[0]: businessRef}, nil
	}

	decoder := json.NewDecoder(strings.NewReader(businessRef))
	decoder.UseNumber()

	var key approval.BusinessRecordKey
	if err := decoder.Decode(&key); err != nil {
		return nil, fmt.Errorf("%w: decode composite key: %w", ErrInvalidBusinessRef, err)
	}

	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("%w: composite key must contain one JSON object", ErrInvalidBusinessRef)
	}

	return key, nil
}
