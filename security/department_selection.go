package security

import (
	"context"
)

// ChallengeTypeDepartmentSelection is the challenge type identifier for department selection.
const ChallengeTypeDepartmentSelection = "department_selection"

// DepartmentOption represents a selectable department.
//
// ID is the only field the framework is party to: Resolve hands it back to
// DepartmentSelector untouched. Name and Meta exist purely so the login UI can
// render the choice.
type DepartmentOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Meta carries whatever the host's login screen needs beyond a label — the
	// owning organization, a parent id for tree rendering, an ancestry path, a
	// sort weight. The framework transports it and nothing more: it is never
	// read, validated, or persisted. Both ends of this channel are host code
	// (the DepartmentLoader that fills it, the challenge renderer that reads
	// it), so the key vocabulary is the host's to define — the framework takes
	// no position on how organizations relate to departments.
	Meta map[string]any `json:"meta,omitempty"`
}

// DepartmentSelectionChallengeData describes the metadata for a department selection challenge.
type DepartmentSelectionChallengeData struct {
	Departments []DepartmentOption `json:"departments"`
	// Meta carries challenge-scoped display data belonging to no single option:
	// the organization tree the options hang off, a default selection, grouping
	// definitions.
	//
	// Prefer it over mixing non-selectable organization nodes into Departments.
	// Every entry there must be a genuinely selectable department — Evaluate
	// treats a non-empty list as "ask the user", and Resolve forwards whichever
	// ID comes back to DepartmentSelector, so a grouping node listed as an
	// option becomes a selectable one that the host then has to reject by hand.
	Meta map[string]any `json:"meta,omitempty"`
}

// DepartmentLoader loads the department choice presented to a user.
type DepartmentLoader interface {
	// LoadDepartments returns the challenge data offered to the user, so the
	// host controls both the selectable options and the challenge-scoped Meta
	// around them. Return nil — or data carrying no departments — to skip the
	// challenge.
	LoadDepartments(ctx context.Context, principal *Principal) (*DepartmentSelectionChallengeData, error)
}

// DepartmentSelector validates the user's department selection and enriches the principal.
type DepartmentSelector interface {
	// SelectDepartment validates the department choice and returns an enriched principal.
	SelectDepartment(ctx context.Context, principal *Principal, departmentID string) (*Principal, error)
}

// DepartmentSelectionChallengeProvider orchestrates department selection evaluation and resolution.
// It implements the ChallengeProvider interface.
type DepartmentSelectionChallengeProvider struct {
	loader   DepartmentLoader
	selector DepartmentSelector
}

// NewDepartmentSelectionChallengeProvider creates a department selection challenge provider.
// Default type "department_selection", order 500.
// Panics if loader or selector is nil.
func NewDepartmentSelectionChallengeProvider(loader DepartmentLoader, selector DepartmentSelector) *DepartmentSelectionChallengeProvider {
	if loader == nil {
		panic("security: DepartmentLoader is required")
	}

	if selector == nil {
		panic("security: DepartmentSelector is required")
	}

	return &DepartmentSelectionChallengeProvider{loader: loader, selector: selector}
}

func (*DepartmentSelectionChallengeProvider) Type() string { return ChallengeTypeDepartmentSelection }
func (*DepartmentSelectionChallengeProvider) Order() int   { return 500 }

func (p *DepartmentSelectionChallengeProvider) Evaluate(ctx context.Context, principal *Principal) (*LoginChallenge, error) {
	data, err := p.loader.LoadDepartments(ctx, principal)
	if err != nil {
		return nil, err
	}

	// No options means nothing to ask, whether the loader signaled that with a
	// nil payload or an empty list.
	if data == nil || len(data.Departments) == 0 {
		return nil, nil
	}

	// The loader's payload is forwarded as-is so both Meta levels reach the
	// client; rebuilding it here is what used to strand them.
	return &LoginChallenge{
		Type:     ChallengeTypeDepartmentSelection,
		Data:     data,
		Required: true,
	}, nil
}

func (p *DepartmentSelectionChallengeProvider) Resolve(ctx context.Context, principal *Principal, response any) (*Principal, error) {
	departmentID, ok := response.(string)
	if !ok || departmentID == "" {
		return nil, ErrDepartmentRequired
	}

	return p.selector.SelectDepartment(ctx, principal, departmentID)
}
