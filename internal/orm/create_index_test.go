package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CreateIndexTestSuite{BaseTestSuite: base}
	})
}

// CreateIndexTestSuite tests CreateIndex operations across all databases.
type CreateIndexTestSuite struct {
	*BaseTestSuite
}

func (suite *CreateIndexTestSuite) SetupSuite() {
	suite.db.RegisterModel((*DDLModel)(nil))
}

// TestCreateAndDrop tests creating and dropping indexes via the orm.DB interface.
func (suite *CreateIndexTestSuite) TestCreateAndDrop() {
	suite.T().Logf("Testing CreateIndex for %s", suite.ds.Kind)

	_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	suite.Require().NoError(err, "Should drop table if it exists")
	_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).Exec(suite.ctx)
	suite.Require().NoError(err, "Should create table for index tests")

	defer func() {
		_, _ = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
	}()

	suite.Run("CreateIndex", func() {
		_, err := suite.db.NewCreateIndex().
			Model((*DDLModel)(nil)).
			Index("idx_ddl_model_label").
			Column("label").
			Exec(suite.ctx)
		suite.NoError(err, "Should create index on label column")
	})

	suite.Run("CreateUniqueIndex", func() {
		_, err := suite.db.NewCreateIndex().
			Model((*DDLModel)(nil)).
			Index("idx_ddl_model_label_unique").
			Column("label").
			Unique().
			Exec(suite.ctx)
		suite.NoError(err, "Should create unique index")
	})

	suite.Run("DropIndex", func() {
		// MySQL requires table context for DROP INDEX and does not support IF EXISTS
		if suite.ds.Kind == config.MySQL {
			_, err := suite.db.NewRaw("ALTER TABLE test_ddl_model DROP INDEX idx_ddl_model_label").
				Exec(suite.ctx)
			suite.NoError(err, "Should drop index on MySQL via raw SQL")
		} else {
			_, err := suite.db.NewDropIndex().
				Index("idx_ddl_model_label").
				IfExists().
				Exec(suite.ctx)
			suite.NoError(err, "Should drop index via DropIndex query")
		}
	})
}

// TestExtended tests extended CreateIndex methods.
func (suite *CreateIndexTestSuite) TestExtended() {
	suite.T().Logf("Testing CreateIndex extended methods for %s", suite.ds.Kind)

	suite.Run("BasicIndex", func() {
		query := suite.db.NewCreateIndex().
			Table("test_tag").
			Index("idx_test_tag_name_ddl").
			Column("name").
			IfNotExists()

		suite.NotNil(query, "Should return non-nil query")
	})

	suite.Run("ColumnExprAndExclude", func() {
		query := suite.db.NewCreateIndex().
			Table("test_tag").
			Index("idx_test_tag_expr").
			ColumnExpr(func(eb orm.ExprBuilder) any {
				return eb.Lower(eb.Column("name"))
			}).
			ExcludeColumn("name")

		suite.NotNil(query, "Should return non-nil query with ColumnExpr")
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("Concurrently", func() {
			query := suite.db.NewCreateIndex().
				Table("test_tag").
				Index("idx_test_tag_conc").
				Column("name").
				Concurrently()

			suite.NotNil(query, "Should return non-nil query with Concurrently")
		})

		suite.Run("IncludeAndUsing", func() {
			query := suite.db.NewCreateIndex().
				Table("test_employee").
				Index("idx_test_employee_inc").
				Column("name").
				Include("email").
				Using(orm.IndexBTree)

			suite.NotNil(query, "Should return non-nil query with Include and Using")
		})

		suite.Run("WherePartialIndex", func() {
			query := suite.db.NewCreateIndex().
				Table("test_employee").
				Index("idx_test_employee_where").
				Column("name").
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("status", "active")
				})

			suite.NotNil(query, "Should return non-nil partial index query")
		})

		suite.Run("UsingHash", func() {
			query := suite.db.NewCreateIndex().
				Table("test_employee").
				Index("idx_test_employee_hash").
				Column("name").
				Using(orm.IndexHash)

			suite.NotNil(query, "Should return non-nil query with Hash index")
		})
	}
}

// TestFluentChaining verifies that CreateIndex queries support fluent method chaining.
func (suite *CreateIndexTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing CreateIndex fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewCreateIndex().
		Model((*DDLModel)(nil)).
		Index("test_idx").
		Column("label").
		Unique().
		IfNotExists()
	suite.NotNil(q, "Should support fluent method chaining")
}
