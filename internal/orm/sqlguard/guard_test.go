package sqlguard

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/config"
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
				assert.ErrorIs(t, guardErr.Err, tt.errType, "Guard error should wrap expected type")
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

	assert.Len(t, guard.rules, 4, "Guard should use four default rules when none provided")
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
		assert.ErrorIs(t, err, ErrDangerousSQL, "GuardError should match ErrDangerousSQL")
	})

	t.Run("WithoutViolation", func(t *testing.T) {
		err := &GuardError{
			Err: ErrSQLParseFailed,
			SQL: "INVALID SQL",
		}

		assert.Equal(t, ErrSQLParseFailed.Error(), err.Error(), "GuardError without violation should use wrapped error message")
		assert.ErrorIs(t, err, ErrSQLParseFailed, "GuardError should match ErrSQLParseFailed")
	})
}

// TestEnsureReadOnly verifies the fail-closed read-only gate used by the MCP
// database query tool: rejection of data-modifying CTEs whose top-level
// statement is a SELECT (including nested CTEs), and the dialect-aware AST-based
// dangerous function denylist — which catches comment/quote-obfuscated calls yet
// does not trip on a function name appearing inside a string literal, and which
// blocks each dialect's own side-effecting primitives.
func TestEnsureReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		kind    config.DBKind
		sql     string
		wantErr bool
	}{
		{"PlainSelect", config.Postgres, "SELECT * FROM users WHERE id = 1", false},
		{"ReadOnlyCTE", config.Postgres, "WITH t AS (SELECT id FROM users) SELECT * FROM t", false},
		{"AggregateFunctionAllowed", config.Postgres, "SELECT count(*) FROM users", false},
		{"Insert", config.Postgres, "INSERT INTO users (name) VALUES ('x')", true},
		{"Update", config.Postgres, "UPDATE users SET name = 'x' WHERE id = 1", true},
		{"Delete", config.Postgres, "DELETE FROM users WHERE id = 1", true},
		{"Drop", config.Postgres, "DROP TABLE users", true},
		{"Truncate", config.Postgres, "TRUNCATE TABLE users", true},
		{"DataModifyingDeleteCTE", config.Postgres, "WITH t AS (DELETE FROM users WHERE id = 1 RETURNING *) SELECT * FROM t", true},
		{"DataModifyingInsertCTE", config.Postgres, "WITH t AS (INSERT INTO users (id) VALUES (1) RETURNING id) SELECT * FROM t", true},
		{"DataModifyingUpdateCTE", config.Postgres, "WITH t AS (UPDATE users SET name = 'x' RETURNING *) SELECT count(*) FROM t", true},
		{"NestedDataModifyingCTE", config.Postgres, "WITH a AS (WITH b AS (DELETE FROM users RETURNING *) SELECT * FROM b) SELECT * FROM a", true},
		{"MultiStatement", config.Postgres, "SELECT 1; DROP TABLE users", true},
		{"DangerousReadFileFunction", config.Postgres, "SELECT pg_read_file('/etc/passwd')", true},
		{"DangerousSleepFunction", config.Postgres, "SELECT pg_sleep(10)", true},
		{"DangerousSequenceMutation", config.Postgres, "SELECT nextval('seq')", true},
		{"DangerousFunctionViaComment", config.Postgres, "SELECT pg_sleep/**/(10)", true},
		{"DangerousFunctionQuotedIdentifier", config.Postgres, `SELECT "pg_read_file"('/etc/passwd')`, true},
		{"FunctionNameInsideStringLiteralAllowed", config.Postgres, "SELECT 'pg_read_file(' AS note", false},
		{"Empty", config.Postgres, "", true},
		// Dialect-aware denylist: each dialect blocks its own primitives.
		{"MySQLLoadFile", config.MySQL, "SELECT load_file('/etc/passwd')", true},
		{"MySQLSleep", config.MySQL, "SELECT sleep(10)", true},
		{"MySQLBenchmark", config.MySQL, "SELECT benchmark(1000000, md5('x'))", true},
		{"SQLServerXpCmdshell", config.SQLServer, "SELECT xp_cmdshell('dir')", true},
		{"OracleUtlFile", config.Oracle, "SELECT utl_file.fopen('d', 'f', 'r') FROM dual", true},
		// Parser coverage pins for the SQL Server / Oracle read surface (the
		// integration lane's sql.queryList): the portable modern forms parse,
		// while dialect-private syntax is refused by the fail-closed parser —
		// scripts must use the standard alternatives (OFFSET/FETCH for TOP,
		// ANSI joins for (+), quoted identifiers for brackets).
		{"SQLServerOffsetFetch", config.SQLServer, "SELECT id FROM users ORDER BY id OFFSET 0 ROWS FETCH NEXT 10 ROWS ONLY", false},
		{"SQLServerTopUnparsed", config.SQLServer, "SELECT TOP 10 id FROM users ORDER BY id", true},
		{"SQLServerBracketsUnparsed", config.SQLServer, "SELECT [id] FROM [users]", true},
		{"SQLServerNolockHintUnparsed", config.SQLServer, "SELECT id FROM users WITH (NOLOCK)", true},
		{"OracleDual", config.Oracle, "SELECT sysdate FROM dual", false},
		{"OracleRownum", config.Oracle, "SELECT id FROM users WHERE ROWNUM <= 10", false},
		{"OracleFetchFirst", config.Oracle, "SELECT id FROM users ORDER BY id FETCH FIRST 10 ROWS ONLY", false},
		{"OracleScalarFunctions", config.Oracle, "SELECT TO_CHAR(created_at, 'YYYY-MM-DD'), NVL(name, 'x') FROM orders", false},
		{"OracleLegacyOuterJoinUnparsed", config.Oracle, "SELECT a.id FROM a, b WHERE a.id = b.id(+)", true},
		{"OracleConnectByUnparsed", config.Oracle, "SELECT id FROM depts START WITH parent_id IS NULL CONNECT BY PRIOR id = parent_id", true},
		// A dialect's own functions are blocked even though they are absent from
		// other dialects' lists: an unknown kind falls back to the union.
		{"MySQLSleepBlockedUnderUnknownKind", config.DBKind("unknown"), "SELECT sleep(10)", true},
		// A harmless MySQL function under a Postgres guard passes (sleep is not a PG primitive).
		{"MySQLSleepAllowedUnderPostgres", config.Postgres, "SELECT sleep(10)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureReadOnly(tt.kind, tt.sql)

			if tt.wantErr {
				require.Error(t, err, "EnsureReadOnly must reject non-read-only SQL")
				assert.True(t, errors.Is(err, ErrNotReadOnly) || errors.Is(err, ErrSQLParseFailed),
					"rejection should surface a read-only or parse error")
			} else {
				assert.NoError(t, err, "EnsureReadOnly must allow read-only SQL")
			}
		})
	}
}
