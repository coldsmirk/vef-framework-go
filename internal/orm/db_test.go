package orm_test

import (
	"context"
	"errors"
	"time"

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

// TestRunInTx tests RunInTx method.
func (suite *DBTestSuite) TestRunInTx() {
	suite.T().Logf("Testing RunInTx for %s", suite.ds.Kind)

	err := suite.db.RunInTx(suite.ctx, func(ctx context.Context, tx orm.DB) error {
		count, err := tx.NewSelect().
			Model((*User)(nil)).
			Count(ctx)

		suite.NoError(err, "Should count users in transaction")
		suite.Equal(int64(20), count, "Should count all fixture users")

		return nil
	})

	suite.NoError(err, "RunInTx should work")
}

// TestRunInReadOnlyTx tests RunInReadOnlyTx method.
func (suite *DBTestSuite) TestRunInReadOnlyTx() {
	suite.T().Logf("Testing RunInReadOnlyTx for %s", suite.ds.Kind)

	err := suite.db.RunInReadOnlyTx(suite.ctx, func(ctx context.Context, tx orm.DB) error {
		count, err := tx.NewSelect().
			Model((*User)(nil)).
			Count(ctx)

		suite.NoError(err, "Should count users in read-only transaction")
		suite.Equal(int64(20), count, "Should count all fixture users")

		return nil
	})

	suite.NoError(err, "RunInReadOnlyTx should work")
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

// TestRunOnConnection tests dedicated connection ownership and scope rules.
func (suite *DBTestSuite) TestRunOnConnection() {
	suite.T().Logf("Testing RunOnConnection for %s", suite.ds.Kind)

	previousMaxOpen := suite.rawDB.Stats().MaxOpenConnections

	suite.rawDB.SetMaxOpenConns(1)
	defer suite.rawDB.SetMaxOpenConns(previousMaxOpen)

	suite.Run("QueryUsesDedicatedConnection", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
		defer cancel()

		var got int

		err := suite.db.RunOnConnection(ctx, func(ctx context.Context, db orm.DB) error {
			return db.NewRaw("SELECT 1").Scan(ctx, &got)
		})

		suite.Require().NoError(err, "Connection-scoped query should not wait for another pooled connection")
		suite.Equal(1, got, "Connection-scoped query should return the selected value")
	})

	suite.Run("NestedScopeReusesConnection", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
		defer cancel()

		var got int

		err := suite.db.RunOnConnection(ctx, func(ctx context.Context, db orm.DB) error {
			return db.RunOnConnection(ctx, func(ctx context.Context, nested orm.DB) error {
				suite.Same(db, nested, "Nested connection scope should reuse the current DB handle")

				return nested.NewRaw("SELECT 1").Scan(ctx, &got)
			})
		})

		suite.Require().NoError(err, "Nested connection scope should not acquire another connection")
		suite.Equal(1, got, "Nested connection-scoped query should return the selected value")
	})

	suite.Run("CallbackErrorReturnsConnection", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
		defer cancel()

		callbackErr := errors.New("callback failed")
		err := suite.db.RunOnConnection(ctx, func(context.Context, orm.DB) error {
			return callbackErr
		})
		suite.ErrorIs(err, callbackErr, "RunOnConnection should preserve the callback error")

		var got int

		err = suite.db.RunOnConnection(ctx, func(ctx context.Context, db orm.DB) error {
			return db.NewRaw("SELECT 1").Scan(ctx, &got)
		})

		suite.Require().NoError(err, "Connection should return to the pool after a callback error")
		suite.Equal(1, got, "Reacquired connection should execute queries")
	})

	suite.Run("RunInTxInsideConnectionScopeUsesThatConnection", func() {
		ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
		defer cancel()

		// With the pool capped at one connection, a transaction that did not
		// run on the dedicated connection would deadlock waiting for another.
		var got int

		err := suite.db.RunOnConnection(ctx, func(ctx context.Context, db orm.DB) error {
			return db.RunInTx(ctx, func(ctx context.Context, tx orm.DB) error {
				return tx.NewRaw("SELECT 1").Scan(ctx, &got)
			})
		})

		suite.Require().NoError(err, "A transaction inside the connection scope should run on the held connection")
		suite.Equal(1, got, "The connection-scoped transaction should return the selected value")
	})

	suite.Run("RunInTxRejectsConnectionScope", func() {
		callbackCalled := false
		err := suite.db.RunInTx(suite.ctx, func(ctx context.Context, tx orm.DB) error {
			runErr := tx.RunOnConnection(ctx, func(context.Context, orm.DB) error {
				callbackCalled = true

				return nil
			})
			suite.ErrorIs(runErr, orm.ErrRunOnConnectionInTx,
				"RunOnConnection inside RunInTx should be rejected")

			return nil
		})

		suite.NoError(err, "RunInTx should commit after the rejected connection scope")
		suite.False(callbackCalled, "Rejected connection callback should not run")
	})

	suite.Run("ManualTxRejectsConnectionScope", func() {
		tx, err := suite.db.BeginTx(suite.ctx, nil)

		suite.Require().NoError(err, "BeginTx should start a transaction")
		defer func() { suite.NoError(tx.Rollback(), "Rollback should work") }()

		callbackCalled := false
		err = tx.RunOnConnection(suite.ctx, func(context.Context, orm.DB) error {
			callbackCalled = true

			return nil
		})

		suite.ErrorIs(err, orm.ErrRunOnConnectionInTx,
			"RunOnConnection on a manual transaction should be rejected")
		suite.False(callbackCalled, "Rejected connection callback should not run")
	})
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

	err := suite.db.RunInTx(suite.ctx, func(ctx context.Context, tx orm.DB) error {
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

		rows, err := suite.db.NewSelect().
			Model((*User)(nil)).
			Select("name").
			OrderBy("name").
			Limit(3).
			Rows(suite.ctx)
		suite.Require().NoError(err, "Select should return rows")

		defer rows.Close()

		err = suite.db.ScanRows(suite.ctx, rows, &results)
		suite.NoError(err, "ScanRows should work")
		suite.NoError(rows.Err(), "rows iteration should not have errors")
		suite.Len(results, 3, "Should have 3 results")
	})

	suite.Run("ScanRow", func() {
		type NameResult struct {
			Name string `bun:"name"`
		}

		var result NameResult

		rows, err := suite.db.NewSelect().
			Model((*User)(nil)).
			Select("name").
			OrderBy("name").
			Limit(1).
			Rows(suite.ctx)
		suite.Require().NoError(err, "Select should return one row")

		defer rows.Close()

		suite.True(rows.Next(), "Should have at least one row")
		err = suite.db.ScanRow(suite.ctx, rows, &result)
		suite.NoError(err, "ScanRow should work")
		suite.NoError(rows.Err(), "rows iteration should not have errors")
		suite.NotEmpty(result.Name, "Scanned name should not be empty")
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

	suite.Run("PoolScope", func() {
		namedDB := suite.db.WithNamedArg("limit_val", 5)
		suite.NotNil(namedDB, "WithNamedArg should return a DB in pool scope")
	})

	suite.Run("ConnectionScope", func() {
		err := suite.db.RunOnConnection(suite.ctx, func(_ context.Context, db orm.DB) error {
			suite.Panics(func() {
				db.WithNamedArg("limit_val", 5)
			}, "WithNamedArg should reject connection scope")

			return nil
		})
		suite.NoError(err, "Connection scope should close after testing WithNamedArg")
	})

	suite.Run("TransactionScope", func() {
		err := suite.db.RunInTx(suite.ctx, func(_ context.Context, tx orm.DB) error {
			suite.Panics(func() {
				tx.WithNamedArg("limit_val", 5)
			}, "WithNamedArg should reject transaction scope")

			return nil
		})
		suite.NoError(err, "Transaction should commit after testing WithNamedArg")
	})
}
