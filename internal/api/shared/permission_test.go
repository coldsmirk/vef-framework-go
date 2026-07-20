package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidPermissionToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{"SingleSegment", "query", true},
		{"TwoSegments", "cron.run", true},
		{"ThreeSegments", "approval.flow.create", true},
		{"UnderscoreInSegment", "integration.ops.dry_run_inbound", true},
		{"Digits", "sys.v2.query", true},
		// Case is not policed: the convention being enforced is the separator.
		{"MixedCase", "Approval.Flow.Create", true},

		{"ColonSeparator", "user:write", false},
		{"MixedSeparators", "approval.flow:create", false},
		{"SlashSeparator", "approval/flow", false},
		{"HyphenSeparator", "approval-flow", false},
		{"Empty", "", false},
		{"LeadingDot", ".approval", false},
		{"TrailingDot", "approval.", false},
		{"EmptySegment", "approval..create", false},
		{"Whitespace", "approval flow", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidPermissionToken(tt.token),
				"Permission token validity should match the expectation")
		})
	}
}
