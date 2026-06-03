package approval_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// TestValidateBusinessIdentifier covers the SQL-injection guard that whitelists
// table/column names before they are interpolated into raw SQL templates.
func TestValidateBusinessIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		// Valid identifiers
		{"SimpleAlpha", "users", false},
		{"WithUnderscore", "user_name", false},
		{"MixedCase", "UserTable", false},
		{"LeadingUnderscore", "_internal", false},
		{"SingleChar", "a", false},
		{"WithDigitInMiddle", "user2_name", false},

		// Boundary: length
		{"MaxLength62Trailing", "a" + strings.Repeat("b", 62), false}, // 63 chars total: passes
		{"TooLong64Chars", "a" + strings.Repeat("b", 63), true},       // 64 chars total: fails
		{"ExactlyOneChar", "z", false},

		// Empty / whitespace: pass (absence is not ValidateBusinessIdentifier's concern)
		{"Empty", "", false},
		{"Whitespace", "   ", false},
		{"Tab", "\t", false},

		// Leading digit: invalid SQL identifier
		{"LeadingDigit", "1table", true},
		{"OnlyDigits", "123", true},

		// Injection-style inputs
		{"WithSpace", "user name", true},
		{"SingleQuote", "user'name", true},
		{"DoubleQuote", `user"name`, true},
		{"Semicolon", "users;drop", true},
		{"DashDash", "user--comment", true},
		{"Slash", "user/name", true},
		{"Parenthesis", "name()", true},
		{"Bracket", "name[0]", true},
		{"DotQualified", "schema.table", true},
		{"Asterisk", "user*", true},
		{"EqualSign", "col=val", true},
		{"Backtick", "`users`", true},
		{"Hash", "user#comment", true},
		{"NullByte", "user\x00name", true},

		// Unicode
		{"ChineseChars", "用户表", true},
		{"AccentedLetter", "tàble", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := approval.ValidateBusinessIdentifier(tt.id)
			if tt.wantErr {
				require.Error(t, err, "%s: expected rejection but got nil", tt.name)
				assert.True(t, errors.Is(err, approval.ErrInvalidBusinessIdentifier), "%s: error should be ErrInvalidBusinessIdentifier", tt.name)
			} else {
				assert.NoError(t, err, "%s: expected acceptance but got error", tt.name)
			}
		})
	}
}
