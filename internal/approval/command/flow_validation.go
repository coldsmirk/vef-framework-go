package command

import (
	"text/template"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// validateBindingMode rejects an out-of-enum binding mode at save time. A
// typo'd mode would silently behave like "standalone" and disable the business
// write-back.
func validateBindingMode(mode approval.BindingMode) error {
	if !mode.IsValid() {
		return shared.ErrInvalidBindingMode
	}

	return nil
}

// validateInitiatorRules enforces that a flow's initiation setting and its
// initiator rules agree, and that every rule is executable.
//
// The setting and the rules are mutually exclusive because permission checking
// short-circuits on isAllInitiationAllowed and never reads the rules — storing
// rules alongside it would display a restriction that does not hold.
//
// Requiring rules on a restricted flow serves two ends. Such a flow could
// otherwise be started by nobody, since the permission check fails closed on an
// empty rule set. And it makes the stored state a bijection, so an empty
// initiator list means exactly "open to everyone" and a single query answers
// who may start a flow.
//
// A rule itself must name a registered kind and carry the input that kind
// declares. Both come from the boot-registered InitiatorResolver set rather
// than from a closed enum here, so a host kind — "any ward supervisor", say —
// is validated exactly like a built-in one. A kind that needs no selection at
// all is legitimate and carries no IDs; only the kinds whose SelectionMode
// picks from a catalog must select something, because a rule that selects
// nobody matches nobody while looking restricted and functional.
func validateInitiatorRules(
	isAllInitiationAllowed bool,
	initiators []shared.CreateFlowInitiatorCmd,
	selections map[approval.InitiatorKind]approval.SelectionMode,
) error {
	if isAllInitiationAllowed {
		if len(initiators) > 0 {
			return shared.ErrInitiatorsNotAllowed
		}

		return nil
	}

	if len(initiators) == 0 {
		return shared.ErrInitiatorsRequired
	}

	for _, init := range initiators {
		switch shared.CheckKindRule(init.Kind, init.IDs, nil, selections) {
		// An initiator kind can never require a form field — there is no form
		// when initiation is checked, which NewCompositeInitiatorResolver
		// rejects at boot — so that fault can only mean the vocabulary itself
		// is misconfigured, which is what the invalid-kind error says.
		case shared.KindRuleUnknownKind, shared.KindRuleFormFieldRequired:
			return shared.ErrInvalidInitiatorKind
		case shared.KindRuleIDsRequired:
			return shared.ErrInitiatorsRequired
		case shared.KindRuleOK:
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
