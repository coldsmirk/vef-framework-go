package sqlguard

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/ajitpratap0/GoSQLX/pkg/gosqlx"
	"github.com/ajitpratap0/GoSQLX/pkg/sql/ast"

	"github.com/coldsmirk/vef-framework-go/logx"
)

var (
	ErrDangerousSQL   = errors.New("dangerous sql detected")
	ErrSQLParseFailed = errors.New("failed to parse sql")
	ErrNotReadOnly    = errors.New("only read-only sql statements are permitted")
)

// GuardError wraps a sql guard error with additional context.
type GuardError struct {
	Err       error
	Violation *Violation
	SQL       string
}

func (e *GuardError) Error() string {
	if e.Violation != nil {
		return fmt.Sprintf("%v: rule=%s, statement=%s, description=%s",
			e.Err, e.Violation.Rule, e.Violation.Statement, e.Violation.Description)
	}

	return e.Err.Error()
}

func (e *GuardError) Unwrap() error {
	return e.Err
}

// Guard coordinates sql rule checking.
type Guard struct {
	rules  []Rule
	logger logx.Logger
}

// NewGuard creates a new sql guard with the given rules.
// If no rules are provided, the default rules are used.
func NewGuard(logger logx.Logger, rules ...Rule) *Guard {
	if len(rules) == 0 {
		rules = DefaultRules()
	}

	return &Guard{
		rules:  rules,
		logger: logger,
	}
}

// Check validates the sql statement against all rules.
// Returns nil if the sql is safe, or an error if a violation is detected.
func (g *Guard) Check(sql string) error {
	astNode, err := gosqlx.Parse(sql)
	if err != nil {
		g.logger.Debugf("Failed to parse sql for guard check: %v", err)

		return nil
	}

	for _, rule := range g.rules {
		if violation := rule.Check(astNode); violation != nil {
			g.logger.Warnf("Sql guard violation: rule=%s, statement=%s, sql=%s",
				violation.Rule, violation.Statement, sql)

			return &GuardError{
				Err:       ErrDangerousSQL,
				Violation: violation,
				SQL:       sql,
			}
		}
	}

	return nil
}

// dangerousFnCall matches PostgreSQL functions that, although callable inside a
// SELECT, perform side effects (server-side file IO, large-object export, remote
// execution, sequence mutation) or enable resource-exhaustion DoS. This is a
// best-effort, defense-in-depth heuristic only — the authoritative protection
// for a tool that runs caller-supplied SQL is to connect with a least-privilege,
// read-only database role.
var dangerousFnCall = regexp.MustCompile(`(?i)\b(pg_read_file|pg_read_binary_file|pg_ls_dir|pg_stat_file|lo_export|lo_import|lo_get|dblink|dblink_exec|pg_sleep|setval)\s*\(`)

// EnsureReadOnly verifies that sql consists solely of read-only statements
// (SELECT/SHOW/DESCRIBE). Unlike Check it fails closed: a parse error, an empty
// statement list, or any non-read statement returns an error. It also rejects
// data-modifying CTEs (e.g. WITH x AS (DELETE ... RETURNING ...) SELECT ...),
// whose top-level type is SELECT but which execute writes. It is intended for
// surfaces that execute caller-supplied SQL, such as the MCP database query tool.
func EnsureReadOnly(sql string) error {
	astNode, err := gosqlx.Parse(sql)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSQLParseFailed, err)
	}

	if len(astNode.Statements) == 0 {
		return ErrNotReadOnly
	}

	for _, stmt := range astNode.Statements {
		if !isReadOnlyStatement(stmt) {
			return newReadOnlyViolation(sql, fmt.Sprintf("%T", stmt))
		}

		if writer := firstWritingCTE(stmt); writer != nil {
			return newReadOnlyViolation(sql, fmt.Sprintf("CTE:%T", writer))
		}
	}

	if dangerousFnCall.MatchString(sql) {
		return newReadOnlyViolation(sql, "dangerous_function")
	}

	return nil
}

// isReadOnlyStatement reports whether stmt is a read-only statement type.
func isReadOnlyStatement(stmt ast.Statement) bool {
	switch stmt.(type) {
	case *ast.SelectStatement, *ast.Select, *ast.ShowStatement, *ast.DescribeStatement:
		return true
	default:
		return false
	}
}

// firstWritingCTE returns the first CTE body in stmt's tree that is not a
// read-only statement, or nil. Descending the whole tree covers nested WITH
// clauses and subqueries, so a write hidden inside any CTE is caught.
func firstWritingCTE(stmt ast.Statement) ast.Statement {
	var writer ast.Statement

	ast.Inspect(stmt, func(n ast.Node) bool {
		if writer != nil {
			return false
		}

		if cte, ok := n.(*ast.CommonTableExpr); ok && cte.Statement != nil && !isReadOnlyStatement(cte.Statement) {
			writer = cte.Statement

			return false
		}

		return true
	})

	return writer
}

// newReadOnlyViolation builds a fail-closed read-only guard error.
func newReadOnlyViolation(sql, statement string) error {
	return &GuardError{
		Err: ErrNotReadOnly,
		SQL: sql,
		Violation: &Violation{
			Rule:        "read_only",
			Statement:   statement,
			Description: "only read-only (SELECT) statements are permitted",
		},
	}
}
