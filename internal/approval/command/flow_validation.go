package command

import (
	"errors"
	"strings"
	"text/template"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
)

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
// rejects flows with blank table/pk/status separately). Returns the
// domain-level shared.ErrInvalidBusinessIdentifier so the API surface emits
// a stable error code; the regex itself lives in
// approval.ValidateBusinessIdentifier so the write-back can reuse it for
// defense-in-depth.
func validateBusinessIdentifiers(mode approval.BindingMode, table, pkField, statusField *string) error {
	if mode != approval.BindingBusiness {
		return nil
	}

	for _, v := range []*string{table, pkField, statusField} {
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
