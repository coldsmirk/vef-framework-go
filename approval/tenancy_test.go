package approval_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			name:           "SystemInternalCrossTenant",
			caller:         approval.SystemCaller,
			entityTenantID: "tenant-b",
			wantErr:        false,
		},
		{
			name:           "ZeroValueDenied",
			caller:         approval.CallerContext{},
			entityTenantID: "tenant-a",
			wantErr:        true,
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
				assert.ErrorIs(t, err, approval.ErrCrossTenantAccess, "Should surface ErrCrossTenantAccess for %s", tc.name)
			} else {
				assert.NoError(t, err, "Should allow %s", tc.name)
			}
		})
	}
}

func TestCallerContextAllows(t *testing.T) {
	t.Parallel()

	assert.True(t, approval.SystemCaller.Allows("any"), "System caller should allow any tenant")
	assert.True(t, approval.CallerContext{IsSuperAdmin: true}.Allows("any"), "Super admin caller should allow any tenant")
	assert.True(t, approval.CallerContext{TenantID: "t1"}.Allows("t1"), "Matching tenant should allow")
	assert.False(t, approval.CallerContext{TenantID: "t1"}.Allows("t2"), "Non-matching tenant should deny")
	assert.False(t, approval.CallerContext{}.Allows("t1"), "Zero caller should deny")
}

func TestCallerContextTenantScopeFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		caller   approval.CallerContext
		override string
		want     *string // nil = unfiltered cross-tenant
		wantErr  bool
	}{
		{"SuperAdminOverrideScopes", approval.CallerContext{IsSuperAdmin: true}, "tenant-x", new("tenant-x"), false},
		{"SuperAdminEmptyOverrideCrossTenant", approval.CallerContext{IsSuperAdmin: true}, "", nil, false},
		{"SystemOverrideScopes", approval.SystemCaller, "tenant-x", new("tenant-x"), false},
		{"SystemEmptyOverrideCrossTenant", approval.SystemCaller, "", nil, false},
		{"NonSuperPinsToOwnTenant", approval.CallerContext{TenantID: "tenant-a"}, "tenant-b", new("tenant-a"), false},
		{"NonSuperEmptyOverrideKeepsOwn", approval.CallerContext{TenantID: "tenant-a"}, "", new("tenant-a"), false},
		// The fail-closed cornerstone: an ordinary caller with no tenant must be
		// denied, never collapsed to an unfiltered cross-tenant query.
		{"ZeroCallerFailsClosed", approval.CallerContext{}, "tenant-x", nil, true},
		{"EmptyTenantNonSuperFailsClosed", approval.CallerContext{}, "", nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.caller.TenantScopeFilter(tc.override)
			if tc.wantErr {
				require.ErrorIs(t, err, approval.ErrCrossTenantAccess, "Empty-tenant ordinary caller must fail closed for %s", tc.name)
				assert.Nil(t, got, "Failed scope must not return a filter for %s", tc.name)

				return
			}

			require.NoError(t, err, "Should not error for %s", tc.name)
			assert.Equal(t, tc.want, got, "Should return expected tenant filter for %s", tc.name)
		})
	}
}

func TestCallerContextResolveWriteTenant(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		caller       approval.CallerContext
		clientTenant string
		want         string
		wantErr      bool
	}{
		{"SuperAdminHonorsClient", approval.CallerContext{IsSuperAdmin: true}, "tenant-x", "tenant-x", false},
		{"SystemHonorsClient", approval.SystemCaller, "tenant-x", "tenant-x", false},
		{"NonSuperPinsToOwnTenant", approval.CallerContext{TenantID: "tenant-a"}, "tenant-b", "tenant-a", false},
		{"EmptyTenantNonSuperFailsClosed", approval.CallerContext{}, "tenant-b", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.caller.ResolveWriteTenant(tc.clientTenant)
			if tc.wantErr {
				require.ErrorIs(t, err, approval.ErrCrossTenantAccess, "Empty-tenant ordinary caller must fail closed for %s", tc.name)
				assert.Empty(t, got, "Failed write-tenant must be empty for %s", tc.name)

				return
			}

			require.NoError(t, err, "Should not error for %s", tc.name)
			assert.Equal(t, tc.want, got, "Should return expected write tenant for %s", tc.name)
		})
	}
}
