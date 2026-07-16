package command

import (
	"regexp"
	"text/template"
	"unicode/utf8"

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

// flowLabelKeyPattern is the JSON-path-safe charset for label keys. The label
// equality filter feeds keys into the cross-dialect JSON path builder, where a
// dot is a nesting separator — a dotted key would be stored fine but never
// match the filter. Restricting keys at save time keeps every stored label
// filterable. 63 chars mirrors the Kubernetes label bound.
var flowLabelKeyPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_-]*[A-Za-z0-9])?$`)

// maxFlowLabelKeyLength and maxFlowLabelValueLength bound a single label
// entry. Values carry no charset restriction because they are only ever
// compared as bind parameters, so their bound counts runes — a Chinese value
// gets the same budget as an ASCII one (keys are ASCII by pattern, where
// bytes and runes coincide).
const (
	maxFlowLabelKeyLength   = 63
	maxFlowLabelValueLength = 256
)

// validateFlowLabels rejects label entries whose key would silently escape
// the label filter (see flowLabelKeyPattern) or whose value exceeds the size
// bound. Empty values are valid — presence-style flags ("mobile": "") are a
// legitimate labeling scheme.
func validateFlowLabels(labels map[string]string) error {
	for key, value := range labels {
		if len(key) > maxFlowLabelKeyLength || !flowLabelKeyPattern.MatchString(key) {
			return shared.ErrInvalidFlowLabel
		}

		if utf8.RuneCountInString(value) > maxFlowLabelValueLength {
			return shared.ErrInvalidFlowLabel
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
