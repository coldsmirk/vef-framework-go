package sqlguard

import (
	"github.com/ajitpratap0/GoSQLX/pkg/sql/ast"
)

// Violation represents a SQL rule violation.
type Violation struct {
	Rule        string
	Statement   string
	Description string
}

// Rule defines the interface for SQL checking rules.
type Rule interface {
	// Name returns the rule identifier (e.g., "no_drop", "no_truncate").
	Name() string
	// Check inspects the SQL AST and returns a Violation if the rule is violated, or nil if it passes.
	Check(astNode *ast.AST) *Violation
}

// DropStatementRule blocks DROP statements.
type DropStatementRule struct{}

func (*DropStatementRule) Name() string {
	return "no_drop"
}

func (r *DropStatementRule) Check(astNode *ast.AST) *Violation {
	for _, stmt := range astNode.Statements {
		if _, ok := stmt.(*ast.DropStatement); ok {
			return &Violation{
				Rule:        r.Name(),
				Statement:   "DROP",
				Description: "DROP statements are prohibited",
			}
		}
	}

	return nil
}

// TruncateStatementRule blocks TRUNCATE statements.
type TruncateStatementRule struct{}

func (*TruncateStatementRule) Name() string {
	return "no_truncate"
}

func (r *TruncateStatementRule) Check(astNode *ast.AST) *Violation {
	for _, stmt := range astNode.Statements {
		if _, ok := stmt.(*ast.TruncateStatement); ok {
			return &Violation{
				Rule:        r.Name(),
				Statement:   "TRUNCATE",
				Description: "TRUNCATE statements are prohibited",
			}
		}
	}

	return nil
}

// DeleteWithoutWhereRule blocks DELETE statements without WHERE clause.
type DeleteWithoutWhereRule struct{}

func (*DeleteWithoutWhereRule) Name() string {
	return "delete_requires_where"
}

func (r *DeleteWithoutWhereRule) Check(astNode *ast.AST) *Violation {
	for _, stmt := range astNode.Statements {
		switch s := stmt.(type) {
		case *ast.DeleteStatement:
			if s.Where == nil {
				return &Violation{
					Rule:        r.Name(),
					Statement:   "DELETE",
					Description: "DELETE statements without WHERE clause are prohibited",
				}
			}

		case *ast.Delete:
			if s.Where == nil {
				return &Violation{
					Rule:        r.Name(),
					Statement:   "DELETE",
					Description: "DELETE statements without WHERE clause are prohibited",
				}
			}
		}
	}

	return nil
}

// DefaultRules returns the default set of SQL checking rules.
func DefaultRules() []Rule {
	return []Rule{
		new(DropStatementRule),
		new(TruncateStatementRule),
		new(DeleteWithoutWhereRule),
	}
}
