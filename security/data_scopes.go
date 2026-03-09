package security

import (
	"cmp"

	"github.com/coldsmirk/vef-framework-go/orm"
)

const (
	// PrioritySelf indicates access only to data created by the user themselves.
	// This is the most restrictive data scope.
	PrioritySelf = 10

	// PriorityDepartment indicates access to data within the user's department.
	PriorityDepartment = 20

	// PriorityDepartmentAndSub indicates access to data within the user's department and all sub-departments.
	PriorityDepartmentAndSub = 30

	// PriorityOrganization indicates access to data within the user's organization.
	PriorityOrganization = 40

	// PriorityOrganizationAndSub indicates access to data within the user's organization and all sub-organizations.
	PriorityOrganizationAndSub = 50

	// PriorityCustom indicates access to data within the user's custom data scope.
	PriorityCustom = 60

	// PriorityAll indicates unrestricted access to all data.
	// This is the broadest data scope, typically used for system administrators.
	PriorityAll = 10000
)

// AllDataScope grants access to all data without any restrictions.
// This is typically used for system administrators or users with full data access.
type AllDataScope struct{}

// NewAllDataScope creates a new AllDataScope instance.
func NewAllDataScope() DataScope {
	return &AllDataScope{}
}

func (*AllDataScope) Key() string {
	return "all"
}

func (*AllDataScope) Priority() int {
	return PriorityAll
}

func (*AllDataScope) Supports(*Principal, *orm.Table) bool {
	return true
}

func (*AllDataScope) Apply(*Principal, orm.SelectQuery) error {
	return nil
}

// SelfDataScope restricts access to data created by the user themselves.
// This is commonly used for personal data access where users can only see their own records.
type SelfDataScope struct {
	// Database column name for the creator, defaults to "created_by"
	createdByColumn string
}

// NewSelfDataScope creates a new SelfDataScope instance.
// The createdByColumn parameter specifies the database column name for the creator.
// If empty, it defaults to "created_by".
func NewSelfDataScope(createdByColumn string) DataScope {
	return &SelfDataScope{
		createdByColumn: cmp.Or(createdByColumn, orm.ColumnCreatedBy),
	}
}

func (*SelfDataScope) Key() string {
	return "self"
}

func (*SelfDataScope) Priority() int {
	return PrioritySelf
}

func (s *SelfDataScope) Supports(_ *Principal, table *orm.Table) bool {
	field, _ := table.Field(s.createdByColumn)

	return field != nil
}

func (s *SelfDataScope) Apply(principal *Principal, query orm.SelectQuery) error {
	query.Where(func(cb orm.ConditionBuilder) {
		cb.Equals(s.createdByColumn, principal.ID)
	})

	return nil
}
