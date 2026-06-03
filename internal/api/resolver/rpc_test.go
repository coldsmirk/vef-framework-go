package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/api"
)

// TestRPCResolve tests the RPC resolver.
func TestRPCResolve(t *testing.T) {
	resolver := NewRPC()

	t.Run("NonRPCResourceReturnsNil", func(t *testing.T) {
		resource := api.NewRESTResource("test")
		spec := api.OperationSpec{Action: "get", Handler: func() error { return nil }}

		result, err := resolver.Resolve(resource, spec)

		assert.NoError(t, err, "Non-RPC resource should be skipped without error")
		assert.Nil(t, result, "Non-RPC resource should yield nil handler")
	})

	t.Run("RPCWithExplicitHandler", func(t *testing.T) {
		resource := api.NewRPCResource("test")
		spec := api.OperationSpec{Action: "get", Handler: func() error { return nil }}

		result, err := resolver.Resolve(resource, spec)

		assert.NoError(t, err, "Explicit RPC handler should resolve without error")
		assert.NotNil(t, result, "Resolved handler should not be nil")
	})

	t.Run("RPCWithMethodLookup", func(t *testing.T) {
		resource := &MockResource{
			Resource: api.NewRPCResource("test"),
		}
		spec := api.OperationSpec{Action: "get_user"}

		result, err := resolver.Resolve(resource, spec)

		assert.NoError(t, err, "RPC handler resolved via PascalCase method lookup should succeed")
		assert.NotNil(t, result, "Resolved handler should not be nil")
	})

	t.Run("RPCMethodNotFound", func(t *testing.T) {
		resource := &MockResource{
			Resource: api.NewRPCResource("test"),
		}
		spec := api.OperationSpec{Action: "non_existent_method"}

		_, err := resolver.Resolve(resource, spec)

		assert.Error(t, err, "Unknown action with no matching method should return an error")
	})

	t.Run("RPCWithFactoryMethod", func(t *testing.T) {
		resource := &MockResource{
			Resource: api.NewRPCResource("test"),
		}
		spec := api.OperationSpec{Action: "create_user_factory"}

		result, err := resolver.Resolve(resource, spec)

		assert.NoError(t, err, "Factory method should resolve without error")
		assert.NotNil(t, result, "Resolved factory handler should not be nil")
	})
}
