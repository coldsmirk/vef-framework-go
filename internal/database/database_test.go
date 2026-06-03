package database_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
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

func (suite *DatabaseTestSuite) TestSQLiteConnection() {
	db, err := database.Open(config.DataSourceConfig{Kind: config.SQLite})
	suite.Require().NoError(err, "SQLite connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "SQLite")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

func (suite *DatabaseTestSuite) TestPostgreSQLConnection() {
	db, err := database.Open(*suite.postgresContainer.DataSource)
	suite.Require().NoError(err, "PostgreSQL connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "PostgreSQL")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

func (suite *DatabaseTestSuite) TestMySQLConnection() {
	db, err := database.Open(*suite.mysqlContainer.DataSource)
	suite.Require().NoError(err, "MySQL connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "MySQL")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

func (suite *DatabaseTestSuite) TestUnsupportedDatabaseKind() {
	db, err := database.Open(config.DataSourceConfig{Kind: "unsupported"})
	suite.Require().Error(err, "Should return error for unsupported database type")
	suite.Require().Nil(db, "Database instance should be nil on error")
	suite.Contains(err.Error(), "unsupported database type", "Error message should mention unsupported type")
}

func (suite *DatabaseTestSuite) TestSQLiteFileMode() {
	tempFile, err := os.CreateTemp("", "test_file_*.db")
	suite.Require().NoError(err, "Temporary file creation should succeed")

	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			suite.T().Logf("Failed to remove temp file: %v", err)
		}
	}()

	suite.Require().NoError(tempFile.Close(), "Temporary file should close successfully")

	db, err := database.Open(config.DataSourceConfig{Kind: config.SQLite, Path: tempFile.Name()})
	suite.Require().NoError(err, "File-based SQLite connection should succeed")
	suite.Require().NotNil(db, "Database instance should not be nil")

	suite.testBasicDBOperations(db, "SQLite File")

	suite.Require().NoError(db.Close(), "Database should close without error")
}

func (suite *DatabaseTestSuite) TestMySQLValidation() {
	db, err := database.Open(config.DataSourceConfig{
		Kind: config.MySQL,
		Host: "localhost",
		Port: 3306,
		User: "root",
	})
	suite.Require().Error(err, "Should return error when database name is missing")
	suite.Require().Nil(db, "Database instance should be nil on validation error")
	suite.Contains(err.Error(), "database name is required", "Error message should mention missing database name")
}

// testBasicDBOperations verifies that an opened *sql.DB is usable: it pings,
// runs a trivial scalar query, and reads the server version. Model round-trips
// belong to the orm layer (see crud and internal/orm tests), not the connection
// factory.
func (suite *DatabaseTestSuite) testBasicDBOperations(db *sql.DB, dbKind string) {
	suite.T().Logf("Testing basic operations for %s", dbKind)

	suite.Require().NoError(db.PingContext(suite.ctx), "Ping should succeed")

	var result int

	err := db.QueryRowContext(suite.ctx, "SELECT 1").Scan(&result)
	suite.Require().NoError(err, "Simple query should succeed")
	suite.Equal(1, result, "Query result should be 1")

	var version string

	switch dbKind {
	case "SQLite", "SQLite File":
		err = db.QueryRowContext(suite.ctx, "SELECT sqlite_version()").Scan(&version)
	default:
		err = db.QueryRowContext(suite.ctx, "SELECT version()").Scan(&version)
	}

	suite.Require().NoError(err, "Version query should succeed")
	suite.NotEmpty(version, "Version should not be empty")
	suite.T().Logf("%s version: %s", dbKind, version)
}

func TestDatabase(t *testing.T) {
	suite.Run(t, new(DatabaseTestSuite))
}
