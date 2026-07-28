package command

import (
	"text/template"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
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

// validateInitiatorPolicy enforces that a flow's initiation setting and its
// initiator rules agree: open-to-everyone carries no rules, restricted carries
// at least one. The two are mutually exclusive because permission checking
// short-circuits on isAllInitiationAllowed and never reads the rules — storing
// rules alongside it would display a restriction that does not hold.
//
// Requiring rules on a restricted flow serves two ends. Such a flow could
// otherwise be started by nobody, since the permission check fails closed on an
// empty rule set. And it makes the stored state a bijection, so an empty
// initiator list means exactly "open to everyone" and a single query answers
// who may start a flow.
func validateInitiatorPolicy(isAllInitiationAllowed bool, initiators []shared.CreateFlowInitiatorCmd) error {
	if isAllInitiationAllowed {
		if len(initiators) > 0 {
			return shared.ErrInitiatorsNotAllowed
		}

		return nil
	}

	if len(initiators) == 0 {
		return shared.ErrInitiatorsRequired
	}

	// A rule that selects nobody matches nobody: CheckInitiationPermission
	// scans each rule's IDs for the applicant, so an empty one leaves the flow
	// just as unstartable as no rule at all — while making the stored list look
	// restricted and functional. Rule count alone is not the invariant.
	for _, init := range initiators {
		if len(init.IDs) == 0 {
			return shared.ErrInitiatorsRequired
		}
	}

	return nil
}

// validateFlowLabels rejects label entries the shared orm.ValidateLabels
// rules refuse (keys that would silently escape the label filter, oversize
// keys or values).
func validateFlowLabels(labels map[string]string) error {
	if err := orm.ValidateLabels(labels); err != nil {
		return shared.ErrInvalidFlowLabel
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
