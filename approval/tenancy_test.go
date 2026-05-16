package approval_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/security"
)

func TestIsSuperAdmin(t *testing.T) {
	t.Parallel()

	t.Run("NilPrincipal", func(t *testing.T) {
		t.Parallel()
		assert.False(t, approval.IsSuperAdmin(nil), "Nil principal should not be super admin")
	})

	t.Run("WithoutRole", func(t *testing.T) {
		t.Parallel()

		p := &security.Principal{Roles: []string{"approval:admin"}}
		assert.False(t, approval.IsSuperAdmin(p), "Principal without SuperAdminRole should not pass")
	})

	t.Run("WithRole", func(t *testing.T) {
		t.Parallel()

		p := &security.Principal{Roles: []string{"some:role", approval.SuperAdminRole}}
		assert.True(t, approval.IsSuperAdmin(p), "Principal carrying SuperAdminRole should pass")
	})
}

func TestCallerContextAuthorize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		caller         approval.CallerContext
		entityTenantID string
		wantErr        bool
	}{
		{
			name:           "SuperAdminCrossTenant",
			caller:         approval.CallerContext{IsSuperAdmin: true, TenantID: ""},
			entityTenantID: "tenant-b",
			wantErr:        false,
		},
		{
			name:           "ZeroValuePermissive",
			caller:         approval.CallerContext{},
			entityTenantID: "tenant-a",
			wantErr:        false,
		},
		{
			name:           "MatchingTenant",
			caller:         approval.CallerContext{TenantID: "tenant-a"},
			entityTenantID: "tenant-a",
			wantErr:        false,
		},
		{
			name:           "CrossTenantDenied",
			caller:         approval.CallerContext{TenantID: "tenant-a"},
			entityTenantID: "tenant-b",
			wantErr:        true,
		},
		{
			name:           "EmptyEntityTenantDenied",
			caller:         approval.CallerContext{TenantID: "tenant-a"},
			entityTenantID: "",
			wantErr:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.caller.Authorize(tc.entityTenantID)
			if tc.wantErr {
				assert.True(t, errors.Is(err, approval.ErrCrossTenantAccess), "Should surface ErrCrossTenantAccess for %s", tc.name)
			} else {
				assert.NoError(t, err, "Should allow %s", tc.name)
			}
		})
	}
}
