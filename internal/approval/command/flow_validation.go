package command

import (
	"errors"
	"strings"
	"text/template"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

// validateFlowEnums rejects out-of-enum binding modes and initiator kinds at
// save time. Both gate load-bearing branches — a typo'd binding mode would
// silently behave like "standalone" and disable the business write-back, and
// an unknown initiator kind would silently never match any user.
func validateFlowEnums(mode approval.BindingMode, initiators []shared.CreateFlowInitiatorCmd) error {
	if !mode.IsValid() {
		return shared.ErrInvalidBindingMode
	}

	for _, init := range initiators {
		if !init.Kind.IsValid() {
			return shared.ErrInvalidInitiatorKind
		}
	}

	return nil
}

// validateBusinessBindingComplete requires the table / pk / status binding
// columns to be present and non-blank whenever BindingMode == BindingBusiness,
// so a half-configured business flow is rejected when the admin saves it rather
// than silently no-op'ing the status write-back on the first completed instance.
func validateBusinessBindingComplete(mode approval.BindingMode, table, pkField, statusField *string) error {
	if mode != approval.BindingBusiness {
		return nil
	}

	for _, v := range []*string{table, pkField, statusField} {
		if v == nil || strings.TrimSpace(*v) == "" {
			return shared.ErrBindingIncomplete
		}
	}

	return nil
}

// validateBusinessIdentifiers enforces the SQL-identifier whitelist on
// every business binding field whenever BindingMode == BindingBusiness.
// Empty values are tolerated here (the runtime check in the binding Writer
// rejects flows with blank table/pk/status separately; optional columns may
// legitimately stay unset). Returns the domain-level
// shared.ErrInvalidBusinessIdentifier so the API surface emits a stable
// error code; the regex itself lives in approval.ValidateBusinessIdentifier
// so the write-back can reuse it for defense-in-depth.
func validateBusinessIdentifiers(mode approval.BindingMode, fields ...*string) error {
	if mode != approval.BindingBusiness {
		return nil
	}

	for _, v := range fields {
		if v == nil {
			continue
		}

		if err := approval.ValidateBusinessIdentifier(*v); err != nil {
			if errors.Is(err, approval.ErrInvalidBusinessIdentifier) {
				return shared.ErrInvalidBusinessIdentifier
			}

			return err
		}
	}

	return nil
}

// validateBusinessColumnsDistinct rejects two binding fields naming the same
// business column. The write-back emits one UPDATE per trigger; a duplicated
// column would render as `SET col = ?, col = ?`, which databases reject at
// runtime — during the applicant's start transaction, where the failure is
// hardest to diagnose. Catching it when the admin saves the flow keeps the
// fault at its source. Only the SET-side columns are checked; the pk field
// lives in the WHERE clause and may legally coincide with a written column.
func validateBusinessColumnsDistinct(mode approval.BindingMode, columns ...*string) error {
	if mode != approval.BindingBusiness {
		return nil
	}

	seen := collections.NewHashSetWithCapacity[string](len(columns))

	for _, v := range columns {
		if v == nil {
			continue
		}

		col := strings.TrimSpace(*v)
		if col == "" {
			continue
		}

		if !seen.Add(col) {
			return shared.ErrBindingColumnsConflict
		}
	}

	return nil
}

// validateInstanceTitleTemplate parses the configured title template so a
// syntax error is rejected when the admin saves the flow, not when the first
// applicant tries to start an instance (where a broken template would fail
// every submission while the admin sees nothing). An empty template is valid
// — start_instance falls back to "flowName-instanceNo".
func validateInstanceTitleTemplate(titleTemplate string) error {
	if titleTemplate == "" {
		return nil
	}

	if _, err := template.New("title").Parse(titleTemplate); err != nil {
		return shared.ErrInvalidTitleTemplate
	}

	return nil
}
