package command

import (
	"regexp"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// businessIdentifierPattern restricts business_table / business_pk_field /
// business_status_field / business_title_field to safe SQL identifiers.
// `DefaultHook.OnInstanceCompleted` interpolates these values into a raw
// `UPDATE %s SET %s = ? WHERE %s = ?` template, so anything outside this
// whitelist (spaces, quotes, semicolons, brackets, sub-selects) could open
// a SQL injection vector. PostgreSQL allows up to 63 characters; we follow
// the same bound.
var businessIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// validateBusinessIdentifiers enforces the SQL-identifier whitelist on
// every business binding field whenever BindingMode == BindingBusiness.
// Empty values are tolerated here (the runtime check in DefaultHook rejects
// flows with blank table/pk/status separately).
func validateBusinessIdentifiers(mode approval.BindingMode, table, pkField, statusField, titleField *string) error {
	if mode != approval.BindingBusiness {
		return nil
	}

	for _, v := range []*string{table, pkField, statusField, titleField} {
		if v == nil {
			continue
		}

		trimmed := strings.TrimSpace(*v)
		if trimmed == "" {
			continue
		}

		if !businessIdentifierPattern.MatchString(trimmed) {
			return shared.ErrInvalidBusinessIdentifier
		}
	}

	return nil
}
