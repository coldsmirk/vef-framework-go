package sqlguard

import (
	"testing"

	"github.com/ajitpratap0/GoSQLX/pkg/gosqlx"
	"github.com/ajitpratap0/GoSQLX/pkg/sql/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseSQL(t *testing.T, sql string) *ast.AST {
	t.Helper()

	astNode, err := gosqlx.Parse(sql)
	require.NoError(t, err, "SQL parser should parse test statement")

	return astNode
}

// TestDropStatementRule tests drop statement rule functionality.
func TestDropStatementRule(t *testing.T) {
	rule := new(DropStatementRule)

	tests := []struct {
		name      string
		sql       string
		wantBlock bool
	}{
		{"DropTable", "DROP TABLE users", true},
		{"DropTableIfExists", "DROP TABLE IF EXISTS users", true},
		{"SelectQuery", "SELECT * FROM users", false},
		{"DeleteWithWhere", "DELETE FROM users WHERE id = 1", false},
		{"InsertQuery", "INSERT INTO users (name) VALUES ('test')", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astNode := parseSQL(t, tt.sql)
			violation := rule.Check(astNode)

			if tt.wantBlock {
				require.NotNil(t, violation, "Drop rule should block DROP statement")
				assert.Equal(t, "no_drop", violation.Rule, "Drop violation should use no_drop rule")
				assert.Equal(t, "DROP", violation.Statement, "Drop violation should record DROP statement")
			} else {
				assert.Nil(t, violation, "Drop rule should allow non-DROP statement")
			}
		})
	}
}

// TestTruncateStatementRule tests truncate statement rule functionality.
func TestTruncateStatementRule(t *testing.T) {
	rule := new(TruncateStatementRule)

	tests := []struct {
		name      string
		sql       string
		wantBlock bool
	}{
		{"TruncateTable", "TRUNCATE TABLE users", true},
		{"SelectQuery", "SELECT * FROM users", false},
		{"DeleteWithWhere", "DELETE FROM users WHERE id = 1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astNode := parseSQL(t, tt.sql)
			violation := rule.Check(astNode)

			if tt.wantBlock {
				require.NotNil(t, violation, "Truncate rule should block TRUNCATE statement")
				assert.Equal(t, "no_truncate", violation.Rule, "Truncate violation should use no_truncate rule")
				assert.Equal(t, "TRUNCATE", violation.Statement, "Truncate violation should record TRUNCATE statement")
			} else {
				assert.Nil(t, violation, "Truncate rule should allow non-TRUNCATE statement")
			}
		})
	}
}

// TestDeleteWithoutWhereRule tests delete without where rule functionality.
func TestDeleteWithoutWhereRule(t *testing.T) {
	rule := new(DeleteWithoutWhereRule)

	tests := []struct {
		name      string
		sql       string
		wantBlock bool
	}{
		{"DeleteWithoutWhere", "DELETE FROM users", true},
		{"DeleteWithWhere", "DELETE FROM users WHERE id = 1", false},
		{"DeleteWithComplexWhere", "DELETE FROM users WHERE created_at < '2023-01-01' AND status = 'inactive'", false},
		{"SelectQuery", "SELECT * FROM users", false},
		{"UpdateWithoutWhere", "UPDATE users SET name = 'test'", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astNode := parseSQL(t, tt.sql)
			violation := rule.Check(astNode)

			if tt.wantBlock {
				require.NotNil(t, violation, "Delete rule should block DELETE without WHERE")
				assert.Equal(t, "delete_requires_where", violation.Rule, "Delete violation should use delete_requires_where rule")
				assert.Equal(t, "DELETE", violation.Statement, "Delete violation should record DELETE statement")
			} else {
				assert.Nil(t, violation, "Delete rule should allow safe statement")
			}
		})
	}
}

// TestDefaultRules tests default rules functionality.
func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()

	assert.Len(t, rules, 3, "DefaultRules should return three rules")

	ruleNames := make([]string, len(rules))
	for i, rule := range rules {
		ruleNames[i] = rule.Name()
	}

	assert.Contains(t, ruleNames, "no_drop", "Default rules should include no_drop")
	assert.Contains(t, ruleNames, "no_truncate", "Default rules should include no_truncate")
	assert.Contains(t, ruleNames, "delete_requires_where", "Default rules should include delete_requires_where")
}
