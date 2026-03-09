package database_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/database/sqlguard"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// DatabaseTestSuite tests database connection and configuration for PostgreSQL, MySQL, and SQLite.
type DatabaseTestSuite struct {
	suite.Suite

	ctx               context.Context
	postgresContainer *testx.PostgresContainer
	mysqlContainer    *testx.MySQLContainer
}

func (suite *DatabaseTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	suite.postgresContainer = testx.NewPostgresContainer(suite.ctx, suite.T())
	suite.mysqlContainer = testx.NewMySQLContainer(suite.ctx, suite.T())
}

// TestSQLiteConnection tests SQLite in-memory database connection and basic operations.
func (suite *DatabaseTestSuite) TestSQLiteConnection() {
	config := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	db, err := database.New(config)
	suite.Require().NoError(err, "SQLite connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "SQLite")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

// TestSQLiteWithOptions tests SQLite with custom configuration options.
func (suite *DatabaseTestSuite) TestSQLiteWithOptions() {
	config := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	db, err := database.New(config, database.DisableQueryHook())
	suite.Require().NoError(err, "SQLite with custom options should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "SQLite")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

// TestPostgreSQLConnection tests PostgreSQL database connection via Testcontainers.
func (suite *DatabaseTestSuite) TestPostgreSQLConnection() {
	config := suite.postgresContainer.DataSource

	db, err := database.New(config)
	suite.Require().NoError(err, "PostgreSQL connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "PostgreSQL")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

// TestMySQLConnection tests MySQL database connection via Testcontainers.
func (suite *DatabaseTestSuite) TestMySQLConnection() {
	config := suite.mysqlContainer.DataSource

	db, err := database.New(config)
	suite.Require().NoError(err, "MySQL connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "MySQL")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

// TestUnsupportedDatabaseKind tests error handling for unsupported database kinds.
func (suite *DatabaseTestSuite) TestUnsupportedDatabaseKind() {
	config := &config.DataSourceConfig{
		Kind: "unsupported",
	}

	db, err := database.New(config)
	suite.Error(err, "Should return error for unsupported database type")
	suite.Nil(db, "Database instance should be nil on error")
	suite.Contains(err.Error(), "unsupported database type", "Error message should mention unsupported type")
}

// TestSQLiteInMemoryMode tests SQLite in-memory mode explicitly.
func (suite *DatabaseTestSuite) TestSQLiteInMemoryMode() {
	config := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	db, err := database.New(config)
	suite.Require().NoError(err, "In-memory SQLite connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "SQLite In-Memory")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

// TestSQLiteFileMode tests SQLite file-based database mode.
func (suite *DatabaseTestSuite) TestSQLiteFileMode() {
	tempFile, err := os.CreateTemp("", "test_file_*.db")
	suite.Require().NoError(err, "Temporary file creation should succeed")

	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			suite.T().Logf("Failed to remove temp file: %v", err)
		}
	}()

	suite.Require().NoError(tempFile.Close(), "Temporary file should close successfully")

	config := &config.DataSourceConfig{
		Kind: config.SQLite,
		Path: tempFile.Name(),
	}

	db, err := database.New(config)
	suite.Require().NoError(err, "File-based SQLite connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "SQLite File")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

// TestMySQLValidation tests MySQL configuration validation for missing required fields.
func (suite *DatabaseTestSuite) TestMySQLValidation() {
	config := &config.DataSourceConfig{
		Kind: config.MySQL,
		Host: "localhost",
		Port: 3306,
		User: "root",
	}

	db, err := database.New(config)
	suite.Error(err, "Should return error when database name is missing")
	suite.Nil(db, "Database instance should be nil on validation error")
	suite.Contains(err.Error(), "database name is required", "Error message should mention missing database name")
}

// TestConnectionPoolConfiguration tests custom connection pool configuration.
func (suite *DatabaseTestSuite) TestConnectionPoolConfiguration() {
	config := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	customPoolConfig := &database.ConnectionPoolConfig{
		MaxIdleConns:    5,
		MaxOpenConns:    10,
		ConnMaxIdleTime: 1 * time.Minute,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := database.New(config, database.WithConnectionPool(customPoolConfig))
	suite.Require().NoError(err, "Connection with custom pool config should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	sqlDB := db.DB
	suite.NotNil(sqlDB, "Underlying SQL DB should not be nil")

	var result int

	err = db.NewSelect().ColumnExpr("1").Scan(suite.ctx, &result)
	suite.Require().NoError(err, "Query should succeed with connection pool")
	suite.Equal(1, result, "Query result should be 1")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

func (suite *DatabaseTestSuite) testBasicDBOperations(db *bun.DB, dbKind string) {
	suite.T().Logf("Testing basic operations for %s", dbKind)

	var result int

	err := db.NewSelect().ColumnExpr("1 as test").Scan(suite.ctx, &result)
	suite.Require().NoError(err, "Simple query should succeed")
	suite.Equal(1, result, "Query result should be 1")

	var version string
	switch dbKind {
	case "SQLite", "SQLite In-Memory", "SQLite File":
		err = db.NewSelect().ColumnExpr("sqlite_version()").Scan(suite.ctx, &version)
	case "PostgreSQL", "MySQL":
		err = db.NewSelect().ColumnExpr("version()").Scan(suite.ctx, &version)
	}

	suite.Require().NoError(err, "Version query should succeed")
	suite.NotEmpty(version, "Version should not be empty")
	suite.T().Logf("%s version: %s", dbKind, version)

	_, err = db.NewCreateTable().
		Model((*TestTable)(nil)).
		IfNotExists().
		Exec(suite.ctx)
	suite.Require().NoError(err, "Table creation should succeed")

	testData := &TestTable{
		Name:  fmt.Sprintf("test_%s", dbKind),
		Value: 42,
	}

	_, err = db.NewInsert().
		Model(testData).
		Exec(suite.ctx)
	suite.Require().NoError(err, "Insert should succeed")

	var retrieved TestTable

	err = db.NewSelect().
		Model(&retrieved).
		Where("name = ?", testData.Name).
		Scan(suite.ctx)
	suite.Require().NoError(err, "Select should succeed")
	suite.Equal(testData.Name, retrieved.Name, "Retrieved name should match")
	suite.Equal(testData.Value, retrieved.Value, "Retrieved value should match")

	_, err = db.NewDropTable().
		Model((*TestTable)(nil)).
		IfExists().
		Exec(suite.ctx)
	suite.Require().NoError(err, "Table cleanup should succeed")
}

type TestTable struct {
	ID    int64  `bun:"id,pk,autoincrement"`
	Name  string `bun:"name,notnull"`
	Value int    `bun:"value"`
}

// TestDatabaseTestSuite tests database test suite functionality.
func TestDatabase(t *testing.T) {
	suite.Run(t, new(DatabaseTestSuite))
}

// SQLGuardTestSuite tests SQL guard integration with raw SQL operations.
// Note: GoSQLX parser doesn't support double-quoted identifiers (bun's default),
// so we use NewRaw with unquoted SQL to test SQL guard functionality.
type SQLGuardTestSuite struct {
	suite.Suite

	ctx context.Context
}

func (suite *SQLGuardTestSuite) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *SQLGuardTestSuite) createTestDB(enableGuard bool) *bun.DB {
	cfg := &config.DataSourceConfig{
		Kind:           config.SQLite,
		EnableSQLGuard: enableGuard,
	}

	db, err := database.New(cfg)
	suite.Require().NoError(err, "Should not return error")

	// Create test table using raw SQL (unquoted)
	_, err = db.NewRaw("CREATE TABLE IF NOT EXISTS test_guard (id INTEGER PRIMARY KEY, name TEXT)").Exec(suite.ctx)
	suite.Require().NoError(err, "Should not return error")

	return db
}

func (suite *SQLGuardTestSuite) TestDropStatementBlocked() {
	db := suite.createTestDB(true)
	defer db.Close()

	_, err := db.NewRaw("DROP TABLE test_guard").Exec(suite.ctx)

	suite.Require().Error(err, "DROP should be blocked by SQL guard")

	// Context cancellation returns context.Canceled, the cause contains GuardError
	suite.True(errors.Is(err, context.Canceled), "Error should be context.Canceled")

	// Verify table still exists (DROP was actually blocked)
	var count int

	err = db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(suite.ctx, &count)
	suite.NoError(err, "Table should still exist after blocked DROP")
}

func (suite *SQLGuardTestSuite) TestTruncateStatementBlocked() {
	db := suite.createTestDB(true)
	defer db.Close()

	// Insert test data first
	_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(suite.ctx)
	suite.Require().NoError(err, "Should not return error")

	_, err = db.NewRaw("TRUNCATE TABLE test_guard").Exec(suite.ctx)

	suite.Require().Error(err, "TRUNCATE should be blocked by SQL guard")
	suite.True(errors.Is(err, context.Canceled), "Error should be context.Canceled")

	// Verify data still exists (TRUNCATE was actually blocked)
	var count int

	err = db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(suite.ctx, &count)
	suite.NoError(err, "Should query count after blocked TRUNCATE")
	suite.Equal(1, count, "Data should still exist after blocked TRUNCATE")
}

func (suite *SQLGuardTestSuite) TestDeleteWithoutWhereBlocked() {
	db := suite.createTestDB(true)
	defer db.Close()

	// Insert test data first
	_, err := db.NewRaw("INSERT INTO test_guard (name) VALUES ('test')").Exec(suite.ctx)
	suite.Require().NoError(err, "Should not return error")

	_, err = db.NewRaw("DELETE FROM test_guard").Exec(suite.ctx)

	suite.Require().Error(err, "DELETE without WHERE should be blocked by SQL guard")
	suite.True(errors.Is(err, context.Canceled), "Error should be context.Canceled")

	// Verify data still exists (DELETE was actually blocked)
	var count int

	err = db.NewRaw("SELECT COUNT(*) FROM test_guard").Scan(suite.ctx, &count)
	suite.NoError(err, "Should query count after blocked DELETE")
	suite.Equal(1, count, "Data should still exist after blocked DELETE without WHERE")
}

func (suite *SQLGuardTestSuite) TestDeleteWithWhereAllowed() {
	db := suite.createTestDB(true)
	defer db.Close()

	_, err := db.NewRaw("DELETE FROM test_guard WHERE name = 'nonexistent'").Exec(suite.ctx)

	suite.NoError(err, "DELETE with WHERE should be allowed")
}

func (suite *SQLGuardTestSuite) TestSelectAllowed() {
	db := suite.createTestDB(true)
	defer db.Close()

	var result []struct {
		ID   int
		Name string
	}

	err := db.NewRaw("SELECT id, name FROM test_guard").Scan(suite.ctx, &result)

	suite.NoError(err, "SELECT should be allowed")
}

func (suite *SQLGuardTestSuite) TestWhitelistBypassesGuard() {
	db := suite.createTestDB(true)
	defer db.Close()

	// DROP should be blocked without whitelist
	_, err := db.NewRaw("DROP TABLE test_guard").Exec(suite.ctx)
	suite.Error(err, "DROP should be blocked without whitelist")

	// DROP should work with whitelisted context
	ctx := sqlguard.WithWhitelist(suite.ctx)
	_, err = db.NewRaw("DROP TABLE test_guard").Exec(ctx)
	suite.NoError(err, "DROP should work with whitelisted context")
}

func (suite *SQLGuardTestSuite) TestDisabledGuardAllowsDangerousSQL() {
	db := suite.createTestDB(false)
	defer db.Close()

	// DROP should work when guard is disabled
	_, err := db.NewRaw("DROP TABLE test_guard").Exec(suite.ctx)
	suite.NoError(err, "DROP should work when SQL guard is disabled")
}

// TestSQLGuardTestSuite tests SQL guard test suite functionality.
func TestSQLGuard(t *testing.T) {
	suite.Run(t, new(SQLGuardTestSuite))
}
