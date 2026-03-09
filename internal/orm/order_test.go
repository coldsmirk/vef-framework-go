package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &OrderTestSuite{BaseTestSuite: base}
	})
}

// OrderTestSuite tests extended order builder methods across all databases.
type OrderTestSuite struct {
	*BaseTestSuite
}

// TestOrderByExpr tests OrderBuilder.Expr method.
func (suite *OrderTestSuite) TestOrderByExpr() {
	suite.T().Logf("Testing OrderBuilder.Expr for %s", suite.ds.Kind)

	type Result struct {
		Name string `bun:"name"`
		Age  int16  `bun:"age"`
	}

	var results []Result

	err := suite.selectUsers().
		Select("name", "age").
		OrderByExpr(func(eb orm.ExprBuilder) any {
			return eb.Order(func(ob orm.OrderBuilder) {
				ob.Expr(eb.Add(eb.Column("age"), 1)).Asc()
			})
		}).
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "OrderBuilder.Expr should work")
	suite.Len(results, 5, "Should have 5 results")

	suite.T().Logf("First result: %s (age %d)", results[0].Name, results[0].Age)
}

// TestOrderByNullsFirst tests OrderBuilder.NullsFirst method.
func (suite *OrderTestSuite) TestOrderByNullsFirst() {
	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("NULLS FIRST not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing OrderBuilder.NullsFirst for %s", suite.ds.Kind)

	type Result struct {
		Title       string  `bun:"title"`
		Description *string `bun:"description"`
	}

	var results []Result

	err := suite.selectPosts().
		Select("p.title", "p.description").
		OrderByExpr(func(eb orm.ExprBuilder) any {
			return eb.Order(func(ob orm.OrderBuilder) {
				ob.Column("description").Asc().NullsFirst()
			})
		}).
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "NullsFirst should work")
	suite.Len(results, 5, "Should have 5 results")

	suite.T().Logf("First result desc: %v", results[0].Description)
}

// TestOrderByNullsLast tests OrderBuilder.NullsLast method.
func (suite *OrderTestSuite) TestOrderByNullsLast() {
	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("NULLS LAST not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing OrderBuilder.NullsLast for %s", suite.ds.Kind)

	type Result struct {
		Title       string  `bun:"title"`
		Description *string `bun:"description"`
	}

	var results []Result

	err := suite.selectPosts().
		Select("p.title", "p.description").
		OrderByExpr(func(eb orm.ExprBuilder) any {
			return eb.Order(func(ob orm.OrderBuilder) {
				ob.Column("description").Desc().NullsLast()
			})
		}).
		Limit(5).
		Scan(suite.ctx, &results)

	suite.NoError(err, "NullsLast should work")
	suite.Len(results, 5, "Should have 5 results")

	suite.T().Logf("First result desc: %v", results[0].Description)
}
