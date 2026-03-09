package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &DropIndexTestSuite{BaseTestSuite: base}
	})
}

// DropIndexTestSuite tests DropIndex operations across all databases.
type DropIndexTestSuite struct {
	*BaseTestSuite
}

// TestExtended tests extended DropIndex methods.
func (suite *DropIndexTestSuite) TestExtended() {
	suite.T().Logf("Testing DropIndex for %s", suite.ds.Kind)

	suite.Run("IfExists", func() {
		query := suite.db.NewDropIndex().
			Index("idx_nonexistent").
			IfExists()

		suite.NotNil(query, "Should return non-nil query with IfExists")
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("Concurrently", func() {
			query := suite.db.NewDropIndex().
				Index("idx_nonexistent").
				IfExists().
				Concurrently()

			suite.NotNil(query, "Should return non-nil query with Concurrently")
		})
	}
}

// TestCascadeAndRestrict tests DropIndex Cascade and Restrict methods.
func (suite *DropIndexTestSuite) TestCascadeAndRestrict() {
	suite.T().Logf("Testing DropIndex Cascade/Restrict for %s", suite.ds.Kind)

	suite.Run("Cascade", func() {
		query := suite.db.NewDropIndex().
			Index("idx_nonexistent").
			IfExists().
			Cascade()

		suite.NotNil(query, "Should return non-nil query with Cascade")
	})

	suite.Run("Restrict", func() {
		query := suite.db.NewDropIndex().
			Index("idx_nonexistent").
			IfExists().
			Restrict()

		suite.NotNil(query, "Should return non-nil query with Restrict")
	})
}

// TestFluentChaining verifies that DropIndex queries support fluent method chaining.
func (suite *DropIndexTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing DropIndex fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewDropIndex().
		Index("test_idx").
		IfExists()
	suite.NotNil(q, "Should support fluent method chaining")
}
