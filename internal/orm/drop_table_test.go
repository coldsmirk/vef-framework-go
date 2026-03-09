package orm_test

import (
	"github.com/stretchr/testify/suite"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &DropTableTestSuite{BaseTestSuite: base}
	})
}

// DropTableTestSuite tests DropTable operations across all databases.
type DropTableTestSuite struct {
	*BaseTestSuite
}

func (suite *DropTableTestSuite) SetupSuite() {
	suite.db.RegisterModel((*DDLModel)(nil))
}

// TestString tests String() output for DropTable.
func (suite *DropTableTestSuite) TestString() {
	suite.T().Logf("Testing DropTable String for %s", suite.ds.Kind)

	sql := suite.db.NewDropTable().
		Model((*DDLModel)(nil)).
		IfExists().
		String()
	suite.Contains(sql, "DROP TABLE", "Should contain DROP TABLE keyword")
	suite.Contains(sql, "IF EXISTS", "Should contain IF EXISTS clause")
}

// TestExtended tests extended DropTable methods.
func (suite *DropTableTestSuite) TestExtended() {
	suite.T().Logf("Testing DropTable extended methods for %s", suite.ds.Kind)

	query := suite.db.NewDropTable().
		Model((*Tag)(nil)).
		IfExists()

	suite.NotNil(query, "Should return non-nil query with IfExists")
}

// TestCascadeAndRestrict tests DropTable Cascade and Restrict methods.
func (suite *DropTableTestSuite) TestCascadeAndRestrict() {
	suite.T().Logf("Testing DropTable Cascade/Restrict for %s", suite.ds.Kind)

	suite.Run("Cascade", func() {
		query := suite.db.NewDropTable().
			Model((*Tag)(nil)).
			Table("nonexistent_table").
			IfExists().
			Cascade()

		suite.NotNil(query, "Should return non-nil query with Cascade")
	})

	suite.Run("Restrict", func() {
		query := suite.db.NewDropTable().
			Model((*Tag)(nil)).
			IfExists().
			Restrict()

		suite.NotNil(query, "Should return non-nil query with Restrict")
	})
}

// TestFluentChaining verifies that DropTable queries support fluent method chaining.
func (suite *DropTableTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing DropTable fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewDropTable().
		Model((*DDLModel)(nil)).
		IfExists()
	suite.NotNil(q, "Should support fluent method chaining")
}
