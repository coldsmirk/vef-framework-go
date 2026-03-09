package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/result"
)

// ChallengeTypeDepartmentSelection is the challenge type identifier for department selection.
const ChallengeTypeDepartmentSelection = "department_selection"

// DepartmentOption represents a selectable department.
type DepartmentOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DepartmentSelectionChallengeData describes the metadata for a department selection challenge.
type DepartmentSelectionChallengeData struct {
	Departments []DepartmentOption `json:"departments"`
	Meta        map[string]any     `json:"meta,omitempty"`
}

// DepartmentLoader loads the list of departments available to a user.
type DepartmentLoader interface {
	// LoadDepartments returns the departments available to the user.
	// Return nil or an empty slice to skip the challenge.
	LoadDepartments(ctx context.Context, principal *Principal) ([]DepartmentOption, error)
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
	departments, err := p.loader.LoadDepartments(ctx, principal)
	if err != nil {
		return nil, err
	}

	if len(departments) == 0 {
		return nil, nil
	}

	return &LoginChallenge{
		Type: ChallengeTypeDepartmentSelection,
		Data: &DepartmentSelectionChallengeData{
			Departments: departments,
		},
		Required: true,
	}, nil
}

func (p *DepartmentSelectionChallengeProvider) Resolve(ctx context.Context, principal *Principal, response any) (*Principal, error) {
	departmentID, ok := response.(string)
	if !ok || departmentID == "" {
		return nil, result.ErrDepartmentRequired
	}

	return p.selector.SelectDepartment(ctx, principal, departmentID)
}
