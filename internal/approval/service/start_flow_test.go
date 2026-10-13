package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
)

func TestStartTenant(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		caller    approval.CallerContext
		requested string
		want      string
		wantErr   bool
	}{
		{"MemberStartsInOwnTenant", approval.CallerContext{TenantID: "tenant-a"}, "", "tenant-a", false},
		{"MemberIgnoresRequestedTenant", approval.CallerContext{TenantID: "tenant-a"}, "tenant-b", "tenant-a", false},
		{"MemberCannotRequestShared", approval.CallerContext{TenantID: "tenant-a"}, approval.SharedTenantID, "tenant-a", false},
		{"MemberWithoutTenantFailsClosed", approval.CallerContext{}, "tenant-b", "", true},
		{"SharedTenantClaimFailsClosed", approval.CallerContext{TenantID: approval.SharedTenantID}, "", "", true},
		{"SuperAdminStartsInRequestedTenant", approval.CallerContext{TenantID: "tenant-a", IsSuperAdmin: true}, "tenant-b", "tenant-b", false},
		{"SuperAdminDefaultsToOwnTenant", approval.CallerContext{TenantID: "tenant-a", IsSuperAdmin: true}, "", "tenant-a", false},
		{"SuperAdminWithoutTenantDefaults", approval.CallerContext{IsSuperAdmin: true}, "", approval.DefaultTenantID, false},
		{"SuperAdminInSharedTenantDefaults", approval.CallerContext{TenantID: approval.SharedTenantID, IsSuperAdmin: true}, "", approval.DefaultTenantID, false},
		{"SystemDefaults", approval.SystemCaller, "", approval.DefaultTenantID, false},
		{"SystemCannotStartInShared", approval.SystemCaller, approval.SharedTenantID, "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.StartTenant(tc.caller, tc.requested)
			if tc.wantErr {
				require.ErrorIs(t, err, approval.ErrCrossTenantAccess, "Should fail closed for %s", tc.name)
				assert.Empty(t, got, "A refused start tenant must be empty for %s", tc.name)

				return
			}

			require.NoError(t, err, "Should resolve a start tenant for %s", tc.name)
			assert.Equal(t, tc.want, got, "Should resolve the expected start tenant for %s", tc.name)
		})
	}
}
