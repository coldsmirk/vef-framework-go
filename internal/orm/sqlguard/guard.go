package sqlguard

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ajitpratap0/GoSQLX/pkg/gosqlx"
	"github.com/ajitpratap0/GoSQLX/pkg/sql/ast"

	collections "github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/config"
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
// Unlike EnsureReadOnly it fails OPEN: SQL that the parser cannot understand is
// allowed through, because Check is a best-effort developer write-guardrail (the
// opt-in query hook), not a security sandbox. The authoritative protection for
// untrusted SQL is a least-privilege database role.
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

// dangerousFunctionsByKind maps each supported dialect to the functions that,
// although callable inside a SELECT, perform side effects (server-side file IO,
// large-object transfer, remote execution, sequence mutation) or enable
// resource-exhaustion DoS. They are matched against parsed AST function-call
// nodes (see firstDangerousFunction), so the check cannot be evaded by casing,
// comments, or quoting and never trips on a name that merely appears inside a
// string literal.
//
// This is a best-effort, non-exhaustive defense-in-depth heuristic, and AST
// denylist parity across dialects is necessarily incomplete: some dangerous
// primitives are not plain function-call nodes (e.g. SQLServer WAITFOR DELAY,
// or extensions loaded at runtime) and cannot be caught here. The authoritative
// protection for a surface that runs caller-supplied SQL is to connect with a
// least-privilege, read-only database role.
var dangerousFunctionsByKind = map[config.DBKind]collections.Set[string]{
	config.Postgres: collections.NewHashSetFrom(
		"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file",
		"lo_export", "lo_import", "lo_get",
		"dblink", "dblink_exec",
		"pg_sleep", "pg_sleep_for", "pg_sleep_until",
		"setval", "nextval",
	),
	config.MySQL: collections.NewHashSetFrom(
		"load_file",
		"sleep", "benchmark",
		"sys_exec", "sys_eval",
	),
	config.SQLServer: collections.NewHashSetFrom(
		"xp_cmdshell", "xp_dirtree", "xp_fileexist", "xp_regread",
		"openrowset", "openquery", "opendatasource",
	),
	config.Oracle: collections.NewHashSetFrom(
		"utl_file", "utl_http", "utl_tcp", "utl_smtp",
		"dbms_lock", "dbms_scheduler", "dbms_pipe",
	),
	config.SQLite: collections.NewHashSetFrom(
		"load_extension", "readfile", "writefile", "edit", "fts3_tokenizer",
	),
}

// dangerousFunctionsFor returns the dangerous-function denylist for kind. Unknown
// kinds fall back to the union of every dialect's denylist so an unexpected
// dialect still fails closed against the broadest set of primitives.
func dangerousFunctionsFor(kind config.DBKind) collections.Set[string] {
	if set, ok := dangerousFunctionsByKind[kind]; ok {
		return set
	}

	union := collections.NewHashSet[string]()
	for _, set := range dangerousFunctionsByKind {
		union.AddAll(set.ToSlice()...)
	}

	return union
}

// EnsureReadOnly verifies that sql consists solely of read-only statements
// (SELECT/SHOW/DESCRIBE) for the given dialect. Unlike Check it fails closed: a
// parse error, an empty statement list, or any non-read statement returns an
// error. It also rejects data-modifying CTEs (e.g. WITH x AS (DELETE ... RETURNING
// ...) SELECT ...), whose top-level type is SELECT but which execute writes, and
// dialect-specific side-effecting functions (see dangerousFunctionsByKind). It is
// intended for surfaces that execute caller-supplied SQL, such as the MCP database
// query tool. kind selects which dangerous-function denylist applies.
func EnsureReadOnly(kind config.DBKind, sql string) error {
	astNode, err := gosqlx.Parse(sql)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSQLParseFailed, err)
	}

	if len(astNode.Statements) == 0 {
		return ErrNotReadOnly
	}

	dangerous := dangerousFunctionsFor(kind)

	for _, stmt := range astNode.Statements {
		if !isReadOnlyStatement(stmt) {
			return newReadOnlyViolation(sql, fmt.Sprintf("%T", stmt))
		}

		// A single tree walk detects both a data-modifying CTE and a dangerous
		// function call; a writing CTE is reported first to keep the violation
		// detail stable regardless of DFS order.
		writer, fn := firstSubtreeViolation(stmt, dangerous)
		if writer != nil {
			return newReadOnlyViolation(sql, fmt.Sprintf("CTE:%T", writer))
		}

		if fn != "" {
			return newReadOnlyViolation(sql, "dangerous_function:"+fn)
		}
	}

	return nil
}

// isReadOnlyStatement reports whether stmt is a read-only statement type.
// It matches the gosqlx.Parse output shape, which only produces the *ast.…Statement
// node types (never the simplified *ast.Select/*ast.Delete forms).
func isReadOnlyStatement(stmt ast.Statement) bool {
	switch stmt.(type) {
	case *ast.SelectStatement, *ast.ShowStatement, *ast.DescribeStatement:
		return true
	default:
		return false
	}
}

// firstSubtreeViolation walks stmt's tree once and returns the first
// data-modifying CTE body (a CTE whose statement is not read-only) and the first
// side-effecting function call whose lower-cased name is in dangerous. Descending
// the whole tree covers nested WITH clauses and subqueries. Inspecting parsed call
// nodes (rather than scanning raw text) ignores names inside string literals and
// cannot be evaded by casing, comments, or quoting. The walk stops once both have
// been found.
func firstSubtreeViolation(stmt ast.Statement, dangerous collections.Set[string]) (ast.Statement, string) {
	var (
		writer ast.Statement
		fn     string
	)

	ast.Inspect(stmt, func(n ast.Node) bool {
		if writer != nil && fn != "" {
			return false
		}

		switch node := n.(type) {
		case *ast.CommonTableExpr:
			if writer == nil && node.Statement != nil && !isReadOnlyStatement(node.Statement) {
				writer = node.Statement
			}
		case *ast.FunctionCall:
			if fn == "" && dangerous.Contains(strings.ToLower(node.Name)) {
				fn = node.Name
			}
		}

		return true
	})

	return writer, fn
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
