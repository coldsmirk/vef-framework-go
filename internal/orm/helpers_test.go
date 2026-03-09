package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/sortx"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &HelpersTestSuite{BaseTestSuite: base}
	})
}

// HelpersTestSuite tests helper functions across all databases.
type HelpersTestSuite struct {
	*BaseTestSuite
}

// TestApplySort tests ApplySort helper function.
func (suite *HelpersTestSuite) TestApplySort() {
	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("NULLS FIRST/LAST not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing ApplySort for %s", suite.ds.Kind)

	type Result struct {
		Name string `bun:"name"`
		Age  int16  `bun:"age"`
	}

	var results []Result

	query := suite.selectUsers().
		Select("name", "age")

	orm.ApplySort(query, []sortx.OrderSpec{
		{Column: "age", Direction: sortx.OrderDesc, NullsOrder: sortx.NullsLast},
		{Column: "name", Direction: sortx.OrderAsc, NullsOrder: sortx.NullsFirst},
	})

	err := query.Limit(5).Scan(suite.ctx, &results)

	suite.NoError(err, "ApplySort should work")
	suite.Len(results, 5, "Should have 5 results")

	suite.T().Logf("First result: %s (age %d)", results[0].Name, results[0].Age)
}
