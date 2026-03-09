package sqlguard

import (
	"errors"
	"fmt"

	"github.com/ajitpratap0/GoSQLX/pkg/gosqlx"

	"github.com/coldsmirk/vef-framework-go/log"
)

var (
	ErrDangerousSQL   = errors.New("dangerous sql detected")
	ErrSQLParseFailed = errors.New("failed to parse sql")
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
	logger log.Logger
}

// NewGuard creates a new sql guard with the given rules.
// If no rules are provided, the default rules are used.
func NewGuard(logger log.Logger, rules ...Rule) *Guard {
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
