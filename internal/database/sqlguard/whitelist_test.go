package sqlguard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestContextKey string

// TestWhitelist tests whitelist functionality.
func TestWhitelist(t *testing.T) {
	tests := []struct {
		name            string
		setupCtx        func() context.Context
		wantWhitelisted bool
	}{
		{
			name:            "BackgroundContextNotWhitelisted",
			setupCtx:        context.Background,
			wantWhitelisted: false,
		},
		{
			name: "WhitelistedContext",
			setupCtx: func() context.Context {
				return WithWhitelist(context.Background())
			},
			wantWhitelisted: true,
		},
		{
			name: "NestedWhitelistedContext",
			setupCtx: func() context.Context {
				ctx := WithWhitelist(context.Background())

				return context.WithValue(ctx, TestContextKey("other_key"), "other_value")
			},
			wantWhitelisted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := IsWhitelisted(ctx)
			assert.Equal(t, tt.wantWhitelisted, result, "Should equal expected value")
		})
	}
}

// TestWhitelistDoesNotAffectOriginalContext tests whitelist does not affect original context functionality.
func TestWhitelistDoesNotAffectOriginalContext(t *testing.T) {
	originalCtx := context.Background()
	whitelistedCtx := WithWhitelist(originalCtx)

	assert.False(t, IsWhitelisted(originalCtx), "Original context should remain non-whitelisted")
	assert.True(t, IsWhitelisted(whitelistedCtx), "New context should be whitelisted")
}
