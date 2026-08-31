package shared

import (
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// KindRuleFault names what is wrong with one assignee / CC / initiator rule,
// leaving the error wording to the caller: the same three checks are made at
// flow deploy (which reports internal design errors) and at flow save (which
// reports API errors), and the checks must not drift between them.
type KindRuleFault int

const (
	// KindRuleOK means the rule is executable.
	KindRuleOK KindRuleFault = iota
	// KindRuleUnknownKind means no resolver is registered for the kind, so
	// nothing could execute the rule.
	KindRuleUnknownKind
	// KindRuleIDsRequired means the kind selects from a catalog but the rule
	// picked nothing. A rule that selects nobody matches nobody.
	KindRuleIDsRequired
	// KindRuleFormFieldRequired means the kind reads its value from the form
	// but the rule names no field.
	KindRuleFormFieldRequired
)

// SelectionIndex maps each descriptor's kind to the input it requires — the
// shape validation consumes, derived from the registered resolver set.
func SelectionIndex[K ~string](descriptors []approval.KindDescriptor[K]) map[K]approval.SelectionMode {
	index := make(map[K]approval.SelectionMode, len(descriptors))
	for _, descriptor := range descriptors {
		index[descriptor.Kind] = descriptor.Selection
	}

	return index
}

// CheckKindRule validates one rule against the registered kind vocabulary: the
// kind must resolve, and whatever input its selection mode declares must be
// present. It is the single place that turns a SelectionMode into a save-time
// requirement, so registering a kind is all a host has to do for its rules to
// be validated like a built-in's.
func CheckKindRule[K ~string](kind K, ids []string, formField *string, selections map[K]approval.SelectionMode) KindRuleFault {
	selection, ok := selections[kind]
	if !ok {
		return KindRuleUnknownKind
	}

	if selection.RequiresIDs() && len(NormalizeUniqueIDs(ids)) == 0 {
		return KindRuleIDsRequired
	}

	if selection.RequiresFormField() && !HasText(formField) {
		return KindRuleFormFieldRequired
	}

	return KindRuleOK
}

// HasText reports whether an optional string carries a non-blank value.
func HasText(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}
