package crud_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/guregu/null/v6"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// Operator is the audit user model referenced by created_by/updated_by.
type Operator struct {
	bun.BaseModel `bun:"table:test_operator,alias:op"`
	orm.IDModel

	Name string `json:"name" bun:",notnull"`
}

// Employee is the primary test model for CRUD tests.
type Employee struct {
	bun.BaseModel `bun:"table:test_employee,alias:te"`
	orm.Model

	Name         string `json:"name"         bun:",notnull"`
	Email        string `json:"email"        bun:",unique,notnull"`
	Description  string `json:"description"`
	Age          int    `json:"age"          bun:",notnull"`
	Position     string `json:"position"     bun:",notnull"`
	DepartmentID string `json:"departmentId" bun:",notnull"`
	Status       string `json:"status"       bun:",notnull,default:'active'"`
}

// EmployeeSearch is the search parameters for Employee.
type EmployeeSearch struct {
	api.P

	ID           null.String `json:"id"           search:"eq"`
	Keyword      null.String `json:"keyword"      search:"contains,column=name|description"`
	Email        null.String `json:"email"        search:"eq"`
	Status       null.String `json:"status"       search:"eq"`
	Age          []int       `json:"age"          search:"between"`
	Position     null.String `json:"position"     search:"eq"`
	DepartmentID null.String `json:"departmentId" search:"eq"`
}

// Department is the tree-based test model.
type Department struct {
	bun.BaseModel `bun:"table:test_department,alias:td"`
	orm.Model

	Name        string  `json:"name"               bun:",notnull"`
	Code        string  `json:"code"               bun:",unique,notnull"`
	Description string  `json:"description"`
	ParentID    *string `json:"parentId"`
	Sort        int     `json:"sort"               bun:",notnull,default:0"`
	Children    any     `json:"children,omitempty" bun:"-"`
}

// DepartmentSearch is the search parameters for Department.
type DepartmentSearch struct {
	api.P

	ID       null.String `json:"id"       search:"eq"`
	Keyword  null.String `json:"keyword"  search:"contains,column=name|description"`
	Code     null.String `json:"code"     search:"eq"`
	ParentID null.String `json:"parentId" search:"eq"`
}

// ProjectAssignment is the composite-PK test model.
type ProjectAssignment struct {
	bun.BaseModel `bun:"table:test_project_assignment,alias:tpa"`

	ProjectCode  string `json:"projectCode"  bun:",pk,notnull"`
	EmployeeID   string `json:"employeeId"   bun:",pk,notnull"`
	Role         string `json:"role"         bun:",notnull"`
	HoursPerWeek int    `json:"hoursPerWeek" bun:",notnull,default:0"`
	Status       string `json:"status"       bun:",notnull,default:'active'"`
	CreatedAt    string `json:"createdAt"    bun:",notnull"`
	CreatedBy    string `json:"createdBy"    bun:",notnull"`
}

// fixtureEndDate is the upper bound for fixture data timestamps.
// All fixture data has created_at in 2025; test-inserted data will have 2026+ timestamps.
var fixtureEndDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// fixtureScope limits queries to fixture data only (created_at < 2026).
// Applied to all query resource definitions so write tests never affect read test results.
func fixtureScope(cb orm.ConditionBuilder) {
	cb.LessThan("created_at", fixtureEndDate)
}

type BaseTestSuite struct {
	apptest.Suite

	ctx   context.Context
	db    orm.DB
	bunDB *bun.DB
	ds    *config.DataSourceConfig
}

// cleanupTestRecords deletes all records created after fixture data (created_at >= 2026).
// This follows the orm module pattern: fixture data stays untouched, tests only create/modify
// their own data, and this method cleans up after each test.
func (suite *BaseTestSuite) cleanupTestRecords() {
	// Order matters: delete child records first to respect FK constraints.
	_, _ = suite.db.NewDelete().Model((*ProjectAssignment)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.GreaterThanOrEqual("created_at", fixtureEndDate)
	}).Exec(suite.ctx)

	_, _ = suite.db.NewDelete().Model((*Employee)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.GreaterThanOrEqual("created_at", fixtureEndDate)
	}).Exec(suite.ctx)
}

func (suite *BaseTestSuite) setupBaseSuite(resourceCtors ...any) {
	opts := make([]fx.Option, 0, len(resourceCtors)+2)
	for _, ctor := range resourceCtors {
		opts = append(opts, vef.ProvideAPIResource(ctor))
	}

	opts = append(opts, fx.Replace(suite.ds))

	suite.SetupAppWithDB(suite.bunDB, opts...)
}

func (suite *BaseTestSuite) tearDownBaseSuite() {
	suite.TearDownApp()
}

// NoPKModel is a test model without primary key fields.
type NoPKModel struct {
	bun.BaseModel `bun:"table:no_pk_test"`

	Name string `json:"name"`
}

// CompositePKModel is a test model with composite primary key.
type CompositePKModel struct {
	bun.BaseModel `bun:"table:composite_pk_test"`

	Key1 string `json:"key1" bun:",pk"`
	Key2 string `json:"key2" bun:",pk"`
	Name string `json:"name"`
}

// callHandlerFactory invokes the handler factory function via reflection with the given DB.
// Returns the error from the factory (second return value).
func callHandlerFactory(t *testing.T, handler any, db orm.DB) error {
	t.Helper()

	fn := reflect.ValueOf(handler)
	results := fn.Call([]reflect.Value{reflect.ValueOf(db)})

	// Factory returns (handlerFunc, error)
	if len(results) == 2 && !results[1].IsNil() {
		return results[1].Interface().(error)
	}

	return nil
}
