package approval

import (
	"errors"
	"fmt"
)

var (
	errKindEmpty            = errors.New("kind descriptor has an empty kind")
	errKindLabelEmpty       = errors.New("kind descriptor has an empty label")
	errInvalidSelectionMode = errors.New("invalid SelectionMode")
)

// SelectionMode declares what a flow designer must supply alongside a kind.
// It is the single answer to two questions that used to be answered by
// hard-coded switches: which input the designer renders for a rule of this
// kind, and which companion field save-time validation requires. A host that
// registers a custom assignee / CC / initiator kind therefore describes its
// input once and both ends follow.
type SelectionMode string

const (
	// SelectionNone takes no designer input at all: the kind carries its own
	// meaning and resolves entirely from the runtime context (the applicant,
	// their department, the instance globals). The built-in `self`,
	// `superior` and `department_leader` assignee kinds work this way, and so
	// does a host kind such as "the applicant's head nurse".
	SelectionNone SelectionMode = "none"
	// SelectionUser picks concrete user IDs.
	SelectionUser SelectionMode = "user"
	// SelectionRole picks role IDs.
	SelectionRole SelectionMode = "role"
	// SelectionDepartment picks department IDs.
	SelectionDepartment SelectionMode = "department"
	// SelectionFormField names one form field whose value carries the IDs.
	SelectionFormField SelectionMode = "form_field"
	// SelectionCustom picks IDs from a host-supplied catalog (an expert
	// panel, a review board). The designer resolves the picker by kind, so
	// several custom kinds coexist without colliding.
	SelectionCustom SelectionMode = "custom"
)

// IsValid reports whether the selection mode is one of the defined values.
func (m SelectionMode) IsValid() bool {
	switch m {
	case SelectionNone, SelectionUser, SelectionRole, SelectionDepartment, SelectionFormField, SelectionCustom:
		return true
	default:
		return false
	}
}

// RequiresIDs reports whether a rule of this selection mode must carry at
// least one ID. A rule that selects nobody matches nobody, so an empty ID list
// is a design error the save must reject rather than a rule that silently
// never fires.
func (m SelectionMode) RequiresIDs() bool {
	switch m {
	case SelectionUser, SelectionRole, SelectionDepartment, SelectionCustom:
		return true
	default:
		return false
	}
}

// RequiresFormField reports whether a rule of this selection mode must name a
// form field.
func (m SelectionMode) RequiresFormField() bool {
	return m == SelectionFormField
}

// KindDescriptor is the designer-facing metadata for one assignee, CC, or
// initiator kind: the kind's identifier, its display label, and the input the
// designer must collect for it. Resolvers return it from Describe, which makes
// the registered resolver set the single source of truth for three things that
// previously drifted independently — the enum the engine accepts, the rules
// save-time validation enforces, and the options the flow designer offers.
type KindDescriptor[K ~string] struct {
	Kind      K             `json:"kind"`
	Label     string        `json:"label"`
	Selection SelectionMode `json:"selection"`
}

// Validate checks the descriptor is usable: a resolver that describes itself
// with a blank kind could never be addressed from a flow definition, one with
// a blank label would render as an empty option in the designer, and an
// out-of-enum selection mode would leave the designer with no input to draw.
// The registry runs this at boot so a faulty host registration fails there
// rather than at the first deploy that tries to use it.
func (d KindDescriptor[K]) Validate() error {
	if d.Kind == "" {
		return errKindEmpty
	}

	if d.Label == "" {
		return fmt.Errorf("%w: %s", errKindLabelEmpty, d.Kind)
	}

	if !d.Selection.IsValid() {
		return fmt.Errorf("%w: %q for kind %s", errInvalidSelectionMode, d.Selection, d.Kind)
	}

	return nil
}

// KindOptions is the flow designer's catalog: every assignee, CC, and
// initiator kind the running application actually accepts, in the order the
// designer should offer them.
//
// It exists because the designer's options and the server's validation used to
// be two hand-maintained lists that could disagree — a kind offered but not
// deployable, or deployable but never offered. Both now read the registered
// resolver sets, so the catalog a client renders is by construction the set a
// deploy will accept.
type KindOptions struct {
	Assignees  []KindDescriptor[AssigneeKind]  `json:"assignees"`
	CCs        []KindDescriptor[CCKind]        `json:"ccs"`
	Initiators []KindDescriptor[InitiatorKind] `json:"initiators"`
}
