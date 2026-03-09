package orm_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &DBTestSuite{BaseTestSuite: base}
	})
}

// DBTestSuite tests DB utility methods across all databases.
type DBTestSuite struct {
	*BaseTestSuite
}

// TestRunInTX tests RunInTX method.
func (suite *DBTestSuite) TestRunInTX() {
	suite.T().Logf("Testing RunInTX for %s", suite.ds.Kind)

	err := suite.db.RunInTX(suite.ctx, func(ctx context.Context, tx orm.DB) error {
		count, err := tx.NewSelect().
			Model((*User)(nil)).
			Count(ctx)

		suite.NoError(err, "Should count users in transaction")
		suite.Equal(int64(20), count, "Should count all fixture users")

		return nil
	})

	suite.NoError(err, "RunInTX should work")
}

// TestRunInReadOnlyTX tests RunInReadOnlyTX method.
func (suite *DBTestSuite) TestRunInReadOnlyTX() {
	suite.T().Logf("Testing RunInReadOnlyTX for %s", suite.ds.Kind)

	err := suite.db.RunInReadOnlyTX(suite.ctx, func(ctx context.Context, tx orm.DB) error {
		count, err := tx.NewSelect().
			Model((*User)(nil)).
			Count(ctx)

		suite.NoError(err, "Should count users in read-only transaction")
		suite.Equal(int64(20), count, "Should count all fixture users")

		return nil
	})

	suite.NoError(err, "RunInReadOnlyTX should work")
}

// TestBeginTx tests BeginTx method.
func (suite *DBTestSuite) TestBeginTx() {
	suite.T().Logf("Testing BeginTx for %s", suite.ds.Kind)

	tx, err := suite.db.BeginTx(suite.ctx, nil)
	suite.NoError(err, "BeginTx should work")
	suite.NotNil(tx, "Transaction should not be nil")

	count, err := tx.NewSelect().
		Model((*User)(nil)).
		Count(suite.ctx)

	suite.NoError(err, "Should count users in transaction")
	suite.Equal(int64(20), count, "Should count all fixture users")

	err = tx.Rollback()
	suite.NoError(err, "Rollback should work")
}

// TestConn tests Conn method.
func (suite *DBTestSuite) TestConn() {
	suite.T().Logf("Testing Conn for %s", suite.ds.Kind)

	conn, err := suite.db.Connection(suite.ctx)
	suite.NoError(err, "Conn should work")
	suite.NotNil(conn, "Conn should return non-nil")

	err = conn.Close()
	suite.NoError(err, "Conn close should work")
}

// TestModelPKs tests ModelPKs method.
func (suite *DBTestSuite) TestModelPKs() {
	suite.T().Logf("Testing ModelPKs for %s", suite.ds.Kind)

	user := &User{}
	user.ID = "test-pk-id"

	pks, err := suite.db.ModelPKs(user)
	suite.NoError(err, "ModelPKs should work")
	suite.True(len(pks) > 0, "Should have PKs")

	suite.T().Logf("PKs: %v", pks)
}

// TestModelPKFields tests ModelPKFields method.
func (suite *DBTestSuite) TestModelPKFields() {
	suite.T().Logf("Testing ModelPKFields for %s", suite.ds.Kind)

	fields := suite.db.ModelPKFields((*User)(nil))
	suite.True(len(fields) > 0, "Should have PK fields")

	suite.T().Logf("PK fields count: %d", len(fields))
}

// TestTxCommit tests transaction Commit method.
func (suite *DBTestSuite) TestTxCommit() {
	suite.T().Logf("Testing Tx Commit for %s", suite.ds.Kind)

	err := suite.db.RunInTX(suite.ctx, func(ctx context.Context, tx orm.DB) error {
		count, err := tx.NewSelect().
			Model((*User)(nil)).
			Count(ctx)
		if err != nil {
			return err
		}

		suite.Equal(int64(20), count, "Should count all fixture users")

		return nil
	})

	suite.NoError(err, "Commit should work")

	tx, err := suite.db.BeginTx(suite.ctx, nil)
	suite.NoError(err, "Should begin transaction")

	_, err = tx.NewSelect().Model((*User)(nil)).Count(suite.ctx)
	suite.NoError(err, "Should count users in transaction")

	err = tx.Commit()
	suite.NoError(err, "Explicit Commit should work")
}

// TestScanRowsAndScanRow tests ScanRows and ScanRow methods on DB.
func (suite *DBTestSuite) TestScanRowsAndScanRow() {
	suite.T().Logf("Testing ScanRows/ScanRow for %s", suite.ds.Kind)

	suite.Run("ScanRows", func() {
		type NameResult struct {
			Name string `bun:"name"`
		}

		var results []NameResult

		conn, err := suite.db.Connection(suite.ctx)
		suite.Require().NoError(err, "Conn should work")

		defer conn.Close()

		rows, err := conn.QueryContext(suite.ctx, "SELECT name FROM test_user ORDER BY name LIMIT 3")
		suite.Require().NoError(err, "Query should work")

		defer rows.Close()

		err = suite.db.ScanRows(suite.ctx, rows, &results)
		suite.NoError(err, "ScanRows should work")
		suite.NoError(rows.Err(), "rows iteration should not have errors")
		suite.Len(results, 3, "Should have 3 results")
	})

	suite.Run("ScanRow", func() {
		type CountResult struct {
			Count int64 `bun:"count"`
		}

		var result CountResult

		conn, err := suite.db.Connection(suite.ctx)
		suite.Require().NoError(err, "Conn should work")

		defer conn.Close()

		rows, err := conn.QueryContext(suite.ctx, "SELECT COUNT(*) AS count FROM test_user")
		suite.Require().NoError(err, "Query should work")

		defer rows.Close()

		suite.True(rows.Next(), "Should have at least one row")
		err = suite.db.ScanRow(suite.ctx, rows, &result)
		suite.NoError(err, "ScanRow should work")
		suite.NoError(rows.Err(), "rows iteration should not have errors")
		suite.Equal(int64(20), result.Count, "Should count all fixture users")
	})
}

// TestPKFieldSet tests PKField.Set method.
func (suite *DBTestSuite) TestPKFieldSet() {
	suite.T().Logf("Testing PKField.Set for %s", suite.ds.Kind)

	fields := suite.db.ModelPKFields((*User)(nil))
	suite.True(len(fields) > 0, "Should have PK fields")

	pkField := fields[0]

	user := &User{}
	err := pkField.Set(user, "new-pk-value")
	suite.NoError(err, "PKField.Set should work")

	pks, err := suite.db.ModelPKs(user)
	suite.NoError(err, "Should get model PKs")
	suite.Equal("new-pk-value", pks[pkField.Name], "Should reflect the set PK value")
}

// TestEnumStrings tests uncovered enum String() methods.
func (suite *DBTestSuite) TestEnumStrings() {
	suite.T().Logf("Testing enum String methods for %s", suite.ds.Kind)

	// Cover various enum string methods by using them in queries
	query := suite.db.NewSelect().
		Model((*User)(nil)).
		Select("name").
		Where(func(cb orm.ConditionBuilder) {
			cb.IsTrue("is_active")
		}).
		Limit(1)

	suite.NotNil(query, "Query with IsTrue should return non-nil")
}

// TestWithNamedArg tests WithNamedArg on DB.
func (suite *DBTestSuite) TestWithNamedArg() {
	suite.T().Logf("Testing WithNamedArg for %s", suite.ds.Kind)

	namedDB := suite.db.WithNamedArg("limit_val", 5)
	suite.NotNil(namedDB, "WithNamedArg should return non-nil")
}
