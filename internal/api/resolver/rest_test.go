package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/api"
)

// TestRESTResolve tests the REST resolver.
func TestRESTResolve(t *testing.T) {
	resolver := NewRest()

	t.Run("NonRESTResourceReturnsNil", func(t *testing.T) {
		resource := api.NewRPCResource("test")
		spec := api.OperationSpec{Action: "get", Handler: func() error { return nil }}

		result, err := resolver.Resolve(resource, spec)

		assert.NoError(t, err, "Non-REST resource should be skipped without error")
		assert.Nil(t, result, "Non-REST resource should yield nil handler")
	})

	t.Run("RESTWithoutHandler", func(t *testing.T) {
		resource := api.NewRESTResource("test")
		spec := api.OperationSpec{Action: "get", Handler: nil}

		_, err := resolver.Resolve(resource, spec)

		assert.Error(t, err, "Nil handler on a REST resource should be rejected")
		assert.Contains(t, err.Error(), "handler is required", "error should indicate that a handler is required")
	})

	t.Run("RESTWithValidHandler", func(t *testing.T) {
		resource := api.NewRESTResource("test")
		spec := api.OperationSpec{Action: "get", Handler: func() error { return nil }}

		result, err := resolver.Resolve(resource, spec)

		assert.NoError(t, err, "Valid REST handler should resolve without error")
		assert.NotNil(t, result, "Resolved handler should not be nil")
	})
}
