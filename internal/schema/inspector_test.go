package schema_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/database"
	"github.com/coldsmirk/vef-framework-go/internal/schema"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

// InspectorTestSuite tests the AtlasInspector implementation.
type InspectorTestSuite struct {
	suite.Suite

	ctx               context.Context
	postgresContainer *testx.PostgresContainer
	mysqlContainer    *testx.MySQLContainer
}

func (suite *InspectorTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	suite.postgresContainer = testx.NewPostgresContainer(suite.ctx, suite.T())
	suite.mysqlContainer = testx.NewMySQLContainer(suite.ctx, suite.T())
}

func (suite *InspectorTestSuite) TestPostgresInspector() {
	suite.T().Log("Testing Inspector for PostgreSQL")
	suite.runInspectorTests(suite.postgresContainer.DataSource, "PostgreSQL")
}

func (suite *InspectorTestSuite) TestMySQLInspector() {
	suite.T().Log("Testing Inspector for MySQL")
	suite.runInspectorTests(suite.mysqlContainer.DataSource, "MySQL")
}

func (suite *InspectorTestSuite) TestSQLiteInspector() {
	suite.T().Log("Testing Inspector for SQLite")

	dsConfig := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	suite.runInspectorTests(dsConfig, "SQLite")
}

func (suite *InspectorTestSuite) runInspectorTests(dsConfig *config.DataSourceConfig, dbKind string) {
	db, err := database.New(dsConfig)
	suite.Require().NoError(err, "Database connection should succeed")

	defer func() {
		suite.Require().NoError(db.Close(), "Database should close without error")
	}()

	suite.setupTestTables(db.DB, dsConfig.Kind)

	defer suite.cleanupTestTables(db.DB)

	inspector, err := schema.NewInspector(db.DB, dsConfig.Kind, dsConfig.Schema)
	suite.Require().NoError(err, "Inspector creation should succeed")

	suite.Run("InspectSchema", func() {
		result, err := inspector.InspectSchema(suite.ctx)
		suite.NoError(err, "InspectSchema should succeed")
		suite.NotNil(result, "Schema result should not be nil")

		tableNames := make([]string, len(result.Tables))
		for i, t := range result.Tables {
			tableNames[i] = t.Name
		}

		suite.T().Logf("%s tables found: %v", dbKind, tableNames)
		suite.Contains(tableNames, "inspector_test_users", "Should find inspector_test_users table")
		suite.Contains(tableNames, "inspector_test_posts", "Should find inspector_test_posts table")
	})

	suite.Run("InspectTable", func() {
		table, err := inspector.InspectTable(suite.ctx, "inspector_test_users")
		suite.NoError(err, "InspectTable should succeed")
		suite.NotNil(table, "Table result should not be nil")
		suite.Equal("inspector_test_users", table.Name, "Table name should match")

		columnNames := make([]string, len(table.Columns))
		for i, col := range table.Columns {
			columnNames[i] = col.Name
		}

		suite.T().Logf("%s inspector_test_users columns: %v", dbKind, columnNames)
		suite.Contains(columnNames, "id", "Should have id column")
		suite.Contains(columnNames, "name", "Should have name column")
		suite.Contains(columnNames, "email", "Should have email column")
	})

	suite.Run("InspectTableNotFound", func() {
		_, err := inspector.InspectTable(suite.ctx, "nonexistent_table_xyz")
		suite.Error(err, "InspectTable should return error for nonexistent table")
		suite.ErrorIs(err, schema.ErrTableNotFound, "Error should be ErrTableNotFound")
	})
}

func (suite *InspectorTestSuite) setupTestTables(db *sql.DB, dbKind config.DBKind) {
	var usersSQL, postsSQL string

	switch dbKind {
	case config.Postgres:
		usersSQL = `
			CREATE TABLE IF NOT EXISTS inspector_test_users (
				id SERIAL PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				email VARCHAR(255) UNIQUE NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
		postsSQL = `
			CREATE TABLE IF NOT EXISTS inspector_test_posts (
				id SERIAL PRIMARY KEY,
				user_id INTEGER NOT NULL REFERENCES inspector_test_users(id) ON DELETE CASCADE,
				title VARCHAR(200) NOT NULL,
				content TEXT,
				status VARCHAR(20) DEFAULT 'draft',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`

	case config.MySQL:
		usersSQL = `
			CREATE TABLE IF NOT EXISTS inspector_test_users (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				email VARCHAR(255) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE KEY idx_email (email)
			)`
		postsSQL = `
			CREATE TABLE IF NOT EXISTS inspector_test_posts (
				id INT AUTO_INCREMENT PRIMARY KEY,
				user_id INT NOT NULL,
				title VARCHAR(200) NOT NULL,
				content TEXT,
				status VARCHAR(20) DEFAULT 'draft',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES inspector_test_users(id) ON DELETE CASCADE,
				INDEX idx_user_id (user_id),
				INDEX idx_status (status)
			)`

	case config.SQLite:
		usersSQL = `
			CREATE TABLE IF NOT EXISTS inspector_test_users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				email TEXT UNIQUE NOT NULL,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			)`
		postsSQL = `
			CREATE TABLE IF NOT EXISTS inspector_test_posts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL REFERENCES inspector_test_users(id) ON DELETE CASCADE,
				title TEXT NOT NULL,
				content TEXT,
				status TEXT DEFAULT 'draft',
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			)`
	}

	_, err := db.ExecContext(suite.ctx, usersSQL)
	suite.Require().NoError(err, "Creating inspector_test_users table should succeed")

	_, err = db.ExecContext(suite.ctx, postsSQL)
	suite.Require().NoError(err, "Creating inspector_test_posts table should succeed")
}

func (suite *InspectorTestSuite) cleanupTestTables(db *sql.DB) {
	_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS inspector_test_posts")
	_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS inspector_test_users")
}

// TestInspectorTestSuite tests inspector test suite functionality.
func TestInspector(t *testing.T) {
	suite.Run(t, new(InspectorTestSuite))
}
