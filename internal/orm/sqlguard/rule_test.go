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
	require.NoError(t, err, "Should not return error")

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
				require.NotNil(t, violation, "Should block DROP statement")
				assert.Equal(t, "no_drop", violation.Rule, "Should equal expected value")
				assert.Equal(t, "DROP", violation.Statement, "Should equal expected value")
			} else {
				assert.Nil(t, violation, "Should allow non-DROP statement")
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
				require.NotNil(t, violation, "Should block TRUNCATE statement")
				assert.Equal(t, "no_truncate", violation.Rule, "Should equal expected value")
				assert.Equal(t, "TRUNCATE", violation.Statement, "Should equal expected value")
			} else {
				assert.Nil(t, violation, "Should allow non-TRUNCATE statement")
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
				require.NotNil(t, violation, "Should block DELETE without WHERE")
				assert.Equal(t, "delete_requires_where", violation.Rule, "Should equal expected value")
				assert.Equal(t, "DELETE", violation.Statement, "Should equal expected value")
			} else {
				assert.Nil(t, violation, "Should allow statement")
			}
		})
	}
}

// TestDefaultRules tests default rules functionality.
func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()

	assert.Len(t, rules, 3, "Should have 3 default rules")

	ruleNames := make([]string, len(rules))
	for i, rule := range rules {
		ruleNames[i] = rule.Name()
	}

	assert.Contains(t, ruleNames, "no_drop", "Should contain expected value")
	assert.Contains(t, ruleNames, "no_truncate", "Should contain expected value")
	assert.Contains(t, ruleNames, "delete_requires_where", "Should contain expected value")
}
