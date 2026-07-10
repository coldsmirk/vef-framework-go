package command

import (
	"text/template"

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
