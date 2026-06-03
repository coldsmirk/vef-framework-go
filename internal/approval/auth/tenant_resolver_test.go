package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/security"
)

func TestDefaultPrincipalTenantResolver(t *testing.T) {
	resolver := NewDefaultPrincipalTenantResolver()
	ctx := context.Background()

	t.Run("NilPrincipal", func(t *testing.T) {
		tenant, err := resolver.Resolve(ctx, nil)
		require.NoError(t, err, "Nil principal (anonymous/system) should return empty without error")
		assert.Empty(t, tenant, "Nil principal should yield empty tenant")
	})

	t.Run("NilDetails", func(t *testing.T) {
		p := &security.Principal{Details: nil}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Principal with nil Details (anonymous/system) should return empty without error")
		assert.Empty(t, tenant, "Nil Details should yield empty tenant")
	})

	t.Run("MapTenantID", func(t *testing.T) {
		p := &security.Principal{Details: map[string]any{"tenant_id": "t-001"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Map with tenant_id key should resolve successfully")
		assert.Equal(t, "t-001", tenant, "Should return the tenant_id value from the map")
	})

	t.Run("MapTenantIdCamelCase", func(t *testing.T) {
		p := &security.Principal{Details: map[string]any{"tenantId": "t-002"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Map with tenantId key should resolve successfully")
		assert.Equal(t, "t-002", tenant, "Should return the tenantId value from the map")
	})

	t.Run("MapTenantIDUpperCase", func(t *testing.T) {
		p := &security.Principal{Details: map[string]any{"TenantID": "t-003"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Map with TenantID key should resolve successfully")
		assert.Equal(t, "t-003", tenant, "Should return the TenantID value from the map")
	})

	t.Run("MapTenant", func(t *testing.T) {
		p := &security.Principal{Details: map[string]any{"Tenant": "t-004"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Map with Tenant key should resolve successfully")
		assert.Equal(t, "t-004", tenant, "Should return the Tenant value from the map")
	})

	t.Run("MapFirstCandidateWins", func(t *testing.T) {
		// tenant_id is checked first; both keys present → tenant_id wins.
		p := &security.Principal{Details: map[string]any{
			"tenant_id": "first",
			"Tenant":    "second",
		}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "First matching candidate key should win")
		assert.Equal(t, "first", tenant, "Should prefer tenant_id over later candidates")
	})

	t.Run("MapMissingTenantField", func(t *testing.T) {
		p := &security.Principal{Details: map[string]any{"user_id": "u-001"}}
		_, err := resolver.Resolve(ctx, p)
		assert.ErrorIs(t, err, ErrTenantNotResolved,
			"Authenticated principal whose map Details has no tenant key should return ErrTenantNotResolved")
	})

	t.Run("MapEmptyTenantValue", func(t *testing.T) {
		// An empty string value must not match — the resolver continues to the
		// next candidate and eventually returns ErrTenantNotResolved.
		p := &security.Principal{Details: map[string]any{"tenant_id": ""}}
		_, err := resolver.Resolve(ctx, p)
		assert.ErrorIs(t, err, ErrTenantNotResolved,
			"Empty string value for a candidate key should not count as resolved")
	})

	// Struct-based principal shapes.

	type principalWithTenantID struct {
		TenantID string
	}

	type principalWithJSONTag struct {
		ID string `json:"tenant_id"` //nolint:tagliatelle // intentional snake_case tag to exercise the resolver's tag matching
	}

	type principalWithCamelJSONTag struct {
		Org string `json:"tenantId"`
	}

	type principalWithCommaTag struct {
		// The json tag has omitempty — extractJSONName must strip the option.
		Tenant string `json:"Tenant,omitempty"` //nolint:tagliatelle // intentional non-camel tag to exercise extractJSONName option stripping
	}

	type principalWithoutTenant struct {
		UserID string
		Name   string
	}

	t.Run("StructFieldByName", func(t *testing.T) {
		p := &security.Principal{Details: principalWithTenantID{TenantID: "t-struct-01"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Struct with TenantID field should resolve by field name")
		assert.Equal(t, "t-struct-01", tenant, "Should return the TenantID struct field value")
	})

	t.Run("StructFieldByJSONTag", func(t *testing.T) {
		p := &security.Principal{Details: principalWithJSONTag{ID: "t-struct-02"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Struct with json:\"tenant_id\" tag should resolve via json tag matching")
		assert.Equal(t, "t-struct-02", tenant, "Should return the field value matched by json tag")
	})

	t.Run("StructFieldByCamelJSONTag", func(t *testing.T) {
		p := &security.Principal{Details: principalWithCamelJSONTag{Org: "t-struct-03"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Struct with json:\"tenantId\" tag should resolve via json tag matching")
		assert.Equal(t, "t-struct-03", tenant, "Should return the field value matched by tenantId json tag")
	})

	t.Run("StructFieldJSONTagWithOption", func(t *testing.T) {
		// Ensures extractJSONName correctly strips the ,omitempty option via strings.Cut.
		p := &security.Principal{Details: principalWithCommaTag{Tenant: "t-struct-04"}}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Struct with json tag containing options should still resolve by base tag name")
		assert.Equal(t, "t-struct-04", tenant, "Should return the field value despite comma option in json tag")
	})

	t.Run("PointerToStruct", func(t *testing.T) {
		inner := principalWithTenantID{TenantID: "t-ptr-01"}
		p := &security.Principal{Details: &inner}
		tenant, err := resolver.Resolve(ctx, p)
		require.NoError(t, err, "Pointer-to-struct Details should be dereferenced by reflection")
		assert.Equal(t, "t-ptr-01", tenant, "Should resolve tenant from a pointer-to-struct Details")
	})

	t.Run("StructMissingTenantField", func(t *testing.T) {
		p := &security.Principal{Details: principalWithoutTenant{UserID: "u-001", Name: "Alice"}}
		_, err := resolver.Resolve(ctx, p)
		assert.ErrorIs(t, err, ErrTenantNotResolved,
			"Authenticated principal whose struct Details has no tenant field should return ErrTenantNotResolved (fail-closed)")
	})

	t.Run("NonStructNonMap", func(t *testing.T) {
		// A scalar Details value (unlikely in practice but must not panic).
		p := &security.Principal{Details: "just-a-string"}
		_, err := resolver.Resolve(ctx, p)
		assert.ErrorIs(t, err, ErrTenantNotResolved,
			"Details that is neither map nor struct should return ErrTenantNotResolved")
	})
}
