package orm_test

import (
	"github.com/stretchr/testify/suite"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &DropColumnTestSuite{BaseTestSuite: base}
	})
}

// DropColumnTestSuite tests DropColumn operations across all databases.
type DropColumnTestSuite struct {
	*BaseTestSuite
}

// TestString tests String() output for DropColumn.
func (suite *DropColumnTestSuite) TestString() {
	suite.T().Logf("Testing DropColumn String for %s", suite.ds.Kind)

	sql := suite.db.NewDropColumn().
		Table("test_ddl_model").
		Column("extra").
		String()
	suite.Contains(sql, "ALTER TABLE", "Should render the ALTER TABLE keyword")
	suite.Contains(sql, "DROP COLUMN", "Should render the DROP COLUMN clause")
}

// TestExtended tests DropColumn query building with Model and Table.
func (suite *DropColumnTestSuite) TestExtended() {
	suite.T().Logf("Testing DropColumn for %s", suite.ds.Kind)

	suite.Run("WithModel", func() {
		query := suite.db.NewDropColumn().
			Model((*Tag)(nil)).
			Column("nonexistent_col")

		suite.NotNil(query, "Should return non-nil query with Model")
	})

	suite.Run("WithTable", func() {
		query := suite.db.NewDropColumn().
			Table("test_ddl_model").
			Column("extra")

		suite.NotNil(query, "Should return non-nil query with Table")
	})
}

// TestFluentChaining verifies that DropColumn queries support fluent method chaining.
func (suite *DropColumnTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing DropColumn fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewDropColumn().
		Table("test_ddl_model").
		Column("extra")
	suite.NotNil(q, "Should support fluent method chaining")
}
