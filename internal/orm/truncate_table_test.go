package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &TruncateTableTestSuite{BaseTestSuite: base}
	})
}

// TruncateTableTestSuite tests TruncateTable operations across all databases.
type TruncateTableTestSuite struct {
	*BaseTestSuite
}

func (suite *TruncateTableTestSuite) SetupSuite() {
	suite.db.RegisterModel((*DDLModel)(nil))
}

// TestString tests String() output for TruncateTable.
func (suite *TruncateTableTestSuite) TestString() {
	suite.T().Logf("Testing TruncateTable String for %s", suite.ds.Kind)

	sql := suite.db.NewTruncateTable().
		Model((*DDLModel)(nil)).
		String()
	suite.Contains(sql, "test_ddl_model", "Should render the target table name")
	// SQLite has no TRUNCATE; bun renders DELETE FROM instead.
	if suite.ds.Kind == config.SQLite {
		suite.Contains(sql, "DELETE FROM", "Should fall back to DELETE FROM on SQLite")
	} else {
		suite.Contains(sql, "TRUNCATE TABLE", "Should render the TRUNCATE TABLE keyword")
	}
}

// TestTruncate tests truncating a table via the orm.DB interface.
func (suite *TruncateTableTestSuite) TestTruncate() {
	suite.T().Logf("Testing TruncateTable for %s", suite.ds.Kind)

	_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	suite.Require().NoError(err, "Should drop table if it exists")
	_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.Require().NoError(err, "Should create table for truncate test")

	defer func() {
		_, _ = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	}()

	models := []DDLModel{
		{Label: "a", Score: 1},
		{Label: "b", Score: 2},
		{Label: "c", Score: 3},
	}
	_, err = suite.db.NewInsert().Model(&models).Exec(suite.ctx)
	suite.Require().NoError(err, "Should insert test data")

	count, err := suite.db.NewSelect().Model((*DDLModel)(nil)).Count(suite.ctx)
	suite.Require().NoError(err, "Should count rows before truncate")
	suite.Equal(int64(3), count, "Should have 3 rows before truncate")

	_, err = suite.db.NewTruncateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.NoError(err, "Should truncate table successfully")

	count, err = suite.db.NewSelect().Model((*DDLModel)(nil)).Count(suite.ctx)
	suite.NoError(err, "Should count rows after truncate")
	suite.Equal(int64(0), count, "Should have 0 rows after truncate")
}

// TestExtended tests extended TruncateTable methods.
func (suite *TruncateTableTestSuite) TestExtended() {
	suite.T().Logf("Testing TruncateTable extended methods for %s", suite.ds.Kind)

	query := suite.db.NewTruncateTable().
		Model((*Tag)(nil)).
		ContinueIdentity()

	suite.NotNil(query, "Should return non-nil query with ContinueIdentity")
}

// TestCascadeAndRestrict tests TruncateTable Cascade and Restrict methods.
func (suite *TruncateTableTestSuite) TestCascadeAndRestrict() {
	suite.T().Logf("Testing TruncateTable Cascade/Restrict for %s", suite.ds.Kind)

	suite.Run("Cascade", func() {
		query := suite.db.NewTruncateTable().
			Model((*Tag)(nil)).
			Table("test_tag").
			Cascade()

		suite.NotNil(query, "Should return non-nil query with Cascade")
	})

	suite.Run("Restrict", func() {
		query := suite.db.NewTruncateTable().
			Model((*Tag)(nil)).
			Restrict()

		suite.NotNil(query, "Should return non-nil query with Restrict")
	})
}

// TestFluentChaining verifies that TruncateTable queries support fluent method chaining.
func (suite *TruncateTableTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing TruncateTable fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewTruncateTable().
		Model((*DDLModel)(nil))
	suite.NotNil(q, "Should support fluent method chaining")
}
