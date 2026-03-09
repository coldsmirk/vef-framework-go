package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &AddColumnTestSuite{BaseTestSuite: base}
	})
}

// AddColumnTestSuite tests AddColumn operations across all databases.
type AddColumnTestSuite struct {
	*BaseTestSuite
}

func (suite *AddColumnTestSuite) SetupSuite() {
	suite.db.RegisterModel((*DDLModel)(nil))
}

// TestAddAndDrop tests adding and dropping columns via the orm.DB interface.
func (suite *AddColumnTestSuite) TestAddAndDrop() {
	suite.T().Logf("Testing AddColumn for %s", suite.ds.Kind)

	_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	suite.Require().NoError(err, "Should drop table if it exists")
	_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.Require().NoError(err, "Should create table for AddColumn tests")

	defer func() {
		_, _ = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	}()

	suite.Run("AddColumn", func() {
		_, err := suite.db.NewAddColumn().
			Table("test_ddl_model").
			Column("extra_field", orm.DataType.VarChar(100)).
			Exec(suite.ctx)
		suite.NoError(err, "Should add column to table")
	})

	suite.Run("DropColumn", func() {
		_, err := suite.db.NewDropColumn().
			Table("test_ddl_model").
			Column("extra_field").
			Exec(suite.ctx)
		suite.NoError(err, "Should drop column from table")
	})
}

// TestExtended tests AddColumn with Model, IfNotExists, and constraints.
func (suite *AddColumnTestSuite) TestExtended() {
	suite.T().Logf("Testing AddColumn extended methods for %s", suite.ds.Kind)

	suite.Run("ModelAndIfNotExists", func() {
		query := suite.db.NewAddColumn().
			Model((*Tag)(nil)).
			IfNotExists().
			Column("extra_col", orm.DataType.Text())

		suite.NotNil(query, "Should return non-nil query")
	})

	suite.Run("ColumnWithConstraints", func() {
		query := suite.db.NewAddColumn().
			Table("test_tag").
			Column("score", orm.DataType.Integer(), orm.NotNull(), orm.Default(0))

		suite.NotNil(query, "Should return non-nil query with constraints")
	})

	suite.Run("NullableColumn", func() {
		query := suite.db.NewAddColumn().
			Table("test_tag").
			Column("avatar_url", orm.DataType.Text(), orm.Nullable())

		suite.NotNil(query, "Should return non-nil query with Nullable")
	})
}

// TestTypeSafe tests the type-safe AddColumn API with real execution.
func (suite *AddColumnTestSuite) TestTypeSafe() {
	suite.T().Logf("Testing AddColumn type-safe API for %s", suite.ds.Kind)

	_, _ = suite.db.NewDropTable().Table("test_ddl_add_col").IfExists().Exec(suite.ctx)
	_, err := suite.db.NewCreateTable().
		Table("test_ddl_add_col").
		Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
		IfNotExists().
		Exec(suite.ctx)
	suite.Require().NoError(err, "Should create table for type-safe AddColumn tests")

	defer func() {
		_, _ = suite.db.NewDropTable().Table("test_ddl_add_col").IfExists().Exec(suite.ctx)
	}()

	suite.Run("AddVarCharColumn", func() {
		_, err := suite.db.NewAddColumn().
			Table("test_ddl_add_col").
			Column("name", orm.DataType.VarChar(200), orm.NotNull(), orm.Default("")).
			Exec(suite.ctx)
		suite.NoError(err, "Should add varchar column with NOT NULL and DEFAULT")
	})

	suite.Run("AddNullableColumn", func() {
		_, err := suite.db.NewAddColumn().
			Table("test_ddl_add_col").
			Column("description", orm.DataType.Text(), orm.Nullable()).
			Exec(suite.ctx)
		suite.NoError(err, "Should add nullable text column")
	})
}

// TestFluentChaining verifies that AddColumn queries support fluent method chaining.
func (suite *AddColumnTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing AddColumn fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewAddColumn().
		Table("test_ddl_model").
		Column("extra", orm.DataType.VarChar(50))
	suite.NotNil(q, "Should support fluent method chaining")
}
