package orm_test

import (
	"github.com/stretchr/testify/suite"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &RawQueryTestSuite{BaseTestSuite: base}
	})
}

// RawQueryTestSuite tests RawQuery methods across all databases.
type RawQueryTestSuite struct {
	*BaseTestSuite
}

// TestRawQueryScan tests RawQuery Scan method.
func (suite *RawQueryTestSuite) TestRawQueryScan() {
	suite.T().Logf("Testing RawQuery Scan for %s", suite.ds.Kind)

	type Result struct {
		Count int64 `bun:"count"`
	}

	var result Result

	err := suite.db.NewRaw("SELECT COUNT(*) AS count FROM test_user").
		Scan(suite.ctx, &result)

	suite.NoError(err, "RawQuery Scan should work")
	suite.Equal(int64(20), result.Count, "Should count all fixture users")
}

// TestRawQueryExec tests RawQuery Exec method.
func (suite *RawQueryTestSuite) TestRawQueryExec() {
	suite.T().Logf("Testing RawQuery Exec for %s", suite.ds.Kind)

	// Use a no-op update to test Exec
	res, err := suite.db.NewRaw("UPDATE test_user SET name = name WHERE 1 = 0").
		Exec(suite.ctx)

	suite.NoError(err, "RawQuery Exec should work")
	suite.NotNil(res, "Result should not be nil")

	affected, err := res.RowsAffected()
	suite.NoError(err, "Should get rows affected count")
	suite.Equal(int64(0), affected, "Should affect zero rows with false condition")
}
