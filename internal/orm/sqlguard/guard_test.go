package sqlguard

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

// TestGuardCheck tests guard check functionality.
func TestGuardCheck(t *testing.T) {
	logger := logx.Named("test")
	guard := NewGuard(logger)

	tests := []struct {
		name      string
		sql       string
		wantBlock bool
		errType   error
	}{
		{"SafeSelect", "SELECT * FROM users WHERE id = 1", false, nil},
		{"SafeDeleteWithWhere", "DELETE FROM users WHERE id = 1", false, nil},
		{"SafeInsert", "INSERT INTO users (name) VALUES ('test')", false, nil},
		{"SafeUpdateWithWhere", "UPDATE users SET name = 'test' WHERE id = 1", false, nil},
		{"DangerousDrop", "DROP TABLE users", true, ErrDangerousSQL},
		{"DangerousTruncate", "TRUNCATE TABLE users", true, ErrDangerousSQL},
		{"DangerousDeleteWithoutWhere", "DELETE FROM users", true, ErrDangerousSQL},
		{"InvalidSqlShouldPass", "INVALID SQL SYNTAX HERE", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guard.Check(tt.sql)

			if tt.wantBlock {
				require.Error(t, err, "Guard should block dangerous SQL")

				var guardErr *GuardError
				require.True(t, errors.As(err, &guardErr), "Guard error should unwrap as GuardError")
				assert.True(t, errors.Is(guardErr.Err, tt.errType), "Guard error should wrap expected type")
			} else {
				assert.NoError(t, err, "Guard should allow SQL that is not blocked")
			}
		})
	}
}

// TestGuardCustomRules tests guard custom rules functionality.
func TestGuardCustomRules(t *testing.T) {
	logger := logx.Named("test")
	guard := NewGuard(logger, new(DropStatementRule))

	// DROP should be blocked
	err := guard.Check("DROP TABLE users")
	require.Error(t, err, "Custom rule should block DROP")

	// DELETE without WHERE should pass (rule not included)
	err = guard.Check("DELETE FROM users")
	assert.NoError(t, err, "Custom rule set should allow DELETE without WHERE")

	// TRUNCATE should pass (rule not included)
	err = guard.Check("TRUNCATE TABLE users")
	assert.NoError(t, err, "Custom rule set should allow TRUNCATE")
}

// TestGuardEmptyRulesUsesDefaults tests guard empty rules uses defaults functionality.
func TestGuardEmptyRulesUsesDefaults(t *testing.T) {
	logger := logx.Named("test")
	guard := NewGuard(logger)

	assert.Len(t, guard.rules, 3, "Guard should use three default rules when none provided")
}

// TestGuardError tests guard error functionality.
func TestGuardError(t *testing.T) {
	t.Run("WithViolation", func(t *testing.T) {
		err := &GuardError{
			Err: ErrDangerousSQL,
			Violation: &Violation{
				Rule:        "no_drop",
				Statement:   "DROP",
				Description: "DROP statements are prohibited",
			},
			SQL: "DROP TABLE users",
		}

		assert.Contains(t, err.Error(), "dangerous sql detected", "GuardError should include dangerous SQL prefix")
		assert.Contains(t, err.Error(), "no_drop", "GuardError should include violation rule")
		assert.Contains(t, err.Error(), "DROP", "GuardError should include blocked statement")
		assert.True(t, errors.Is(err, ErrDangerousSQL), "GuardError should match ErrDangerousSQL")
	})

	t.Run("WithoutViolation", func(t *testing.T) {
		err := &GuardError{
			Err: ErrSQLParseFailed,
			SQL: "INVALID SQL",
		}

		assert.Equal(t, ErrSQLParseFailed.Error(), err.Error(), "GuardError without violation should use wrapped error message")
		assert.True(t, errors.Is(err, ErrSQLParseFailed), "GuardError should match ErrSQLParseFailed")
	})
}
