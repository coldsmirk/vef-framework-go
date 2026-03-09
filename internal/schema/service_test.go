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
	pkgschema "github.com/coldsmirk/vef-framework-go/schema"
)

// ServiceTestSuite tests the DefaultService implementation.
type ServiceTestSuite struct {
	suite.Suite

	ctx               context.Context
	postgresContainer *testx.PostgresContainer
	mysqlContainer    *testx.MySQLContainer
}

func (suite *ServiceTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	suite.postgresContainer = testx.NewPostgresContainer(suite.ctx, suite.T())
	suite.mysqlContainer = testx.NewMySQLContainer(suite.ctx, suite.T())
}

func (suite *ServiceTestSuite) TestPostgresService() {
	suite.T().Log("Testing Service for PostgreSQL")
	suite.runServiceTests(suite.postgresContainer.DataSource, "PostgreSQL")
}

func (suite *ServiceTestSuite) TestMySQLService() {
	suite.T().Log("Testing Service for MySQL")
	suite.runServiceTests(suite.mysqlContainer.DataSource, "MySQL")
}

func (suite *ServiceTestSuite) TestSQLiteService() {
	suite.T().Log("Testing Service for SQLite")

	dsConfig := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	suite.runServiceTests(dsConfig, "SQLite")
}

func (suite *ServiceTestSuite) runServiceTests(dsConfig *config.DataSourceConfig, dbKind string) {
	db, err := database.New(dsConfig)
	suite.Require().NoError(err, "Database connection should succeed")

	defer func() {
		suite.Require().NoError(db.Close(), "Database should close without error")
	}()

	suite.setupTestTables(db.DB, dsConfig.Kind)

	defer suite.cleanupTestTables(db.DB)

	svc, err := schema.NewService(db.DB, dsConfig)
	suite.Require().NoError(err, "Service creation should succeed")

	suite.Run("ListTables", func() {
		tables, err := svc.ListTables(suite.ctx)
		suite.NoError(err, "ListTables should succeed")
		suite.NotEmpty(tables, "Tables list should not be empty")

		tableNames := make([]string, len(tables))
		for i, t := range tables {
			tableNames[i] = t.Name
		}

		suite.T().Logf("%s tables: %v", dbKind, tableNames)
		suite.Contains(tableNames, "service_test_categories", "Should find service_test_categories table")
		suite.Contains(tableNames, "service_test_products", "Should find service_test_products table")

		// Verify table structure
		for _, table := range tables {
			suite.NotEmpty(table.Name, "Table name should not be empty")
			suite.T().Logf("Table: %s, Schema: %s, Comment: %s", table.Name, table.Schema, table.Comment)
		}
	})

	suite.Run("GetTableSchemaWithColumns", func() {
		tableSchema, err := svc.GetTableSchema(suite.ctx, "service_test_categories")
		suite.NoError(err, "GetTableSchema should succeed")
		suite.NotNil(tableSchema, "TableSchema should not be nil")
		suite.Equal("service_test_categories", tableSchema.Name, "Table name should match")

		suite.NotEmpty(tableSchema.Columns, "Columns should not be empty")

		columnMap := make(map[string]pkgschema.Column)
		for _, col := range tableSchema.Columns {
			columnMap[col.Name] = col
		}

		suite.T().Logf("%s service_test_categories columns: %d", dbKind, len(tableSchema.Columns))

		for _, col := range tableSchema.Columns {
			suite.T().Logf("  Column: %s, Type: %s, Nullable: %v, PK: %v, AutoIncrement: %v",
				col.Name, col.Type, col.Nullable, col.IsPrimaryKey, col.IsAutoIncrement)
		}

		suite.Contains(columnMap, "id", "Should have id column")
		suite.Contains(columnMap, "name", "Should have name column")

		idCol := columnMap["id"]
		suite.True(idCol.IsPrimaryKey, "id should be primary key")
		suite.True(idCol.IsAutoIncrement, "id should be auto increment")
	})

	suite.Run("GetTableSchemaWithPrimaryKey", func() {
		tableSchema, err := svc.GetTableSchema(suite.ctx, "service_test_categories")
		suite.NoError(err, "GetTableSchema should succeed")

		suite.NotNil(tableSchema.PrimaryKey, "PrimaryKey should not be nil")
		suite.NotEmpty(tableSchema.PrimaryKey.Columns, "PrimaryKey columns should not be empty")
		suite.Contains(tableSchema.PrimaryKey.Columns, "id", "Primary key should include id column")

		suite.T().Logf("%s service_test_categories PrimaryKey: %v",
			dbKind, tableSchema.PrimaryKey.Columns)
	})

	suite.Run("GetTableSchemaWithIndexes", func() {
		tableSchema, err := svc.GetTableSchema(suite.ctx, "service_test_products")
		suite.NoError(err, "GetTableSchema should succeed")

		suite.T().Logf("%s service_test_products indexes: %d", dbKind, len(tableSchema.Indexes))

		for _, idx := range tableSchema.Indexes {
			suite.T().Logf("  Index: %s, Columns: %v", idx.Name, idx.Columns)
		}

		suite.T().Logf("%s service_test_products unique keys: %d", dbKind, len(tableSchema.UniqueKeys))

		for _, uk := range tableSchema.UniqueKeys {
			suite.T().Logf("  UniqueKey: %s, Columns: %v", uk.Name, uk.Columns)
		}
	})

	suite.Run("GetTableSchemaWithForeignKeys", func() {
		tableSchema, err := svc.GetTableSchema(suite.ctx, "service_test_products")
		suite.NoError(err, "GetTableSchema should succeed")

		suite.T().Logf("%s service_test_products foreign keys: %d", dbKind, len(tableSchema.ForeignKeys))

		for _, fk := range tableSchema.ForeignKeys {
			suite.T().Logf("  ForeignKey: %s, Columns: %v -> %s(%v), OnDelete: %s, OnUpdate: %s",
				fk.Name, fk.Columns, fk.RefTable, fk.RefColumns, fk.OnDelete, fk.OnUpdate)
		}

		if len(tableSchema.ForeignKeys) > 0 {
			fk := tableSchema.ForeignKeys[0]
			suite.NotEmpty(fk.Columns, "FK columns should not be empty")
			suite.NotEmpty(fk.RefTable, "FK ref table should not be empty")
			suite.NotEmpty(fk.RefColumns, "FK ref columns should not be empty")
		}
	})

	suite.Run("GetTableSchemaNotFound", func() {
		_, err := svc.GetTableSchema(suite.ctx, "nonexistent_table_xyz")
		suite.Error(err, "GetTableSchema should return error for nonexistent table")
	})
}

func (suite *ServiceTestSuite) setupTestTables(db *sql.DB, dbKind config.DBKind) {
	var (
		categoriesSQL, productsSQL string
		additionalSQL              []string
	)

	switch dbKind {
	case config.Postgres:
		categoriesSQL = `
			CREATE TABLE IF NOT EXISTS service_test_categories (
				id SERIAL PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				description TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
		productsSQL = `
			CREATE TABLE IF NOT EXISTS service_test_products (
				id SERIAL PRIMARY KEY,
				category_id INTEGER NOT NULL REFERENCES service_test_categories(id) ON DELETE CASCADE,
				sku VARCHAR(50) NOT NULL,
				name VARCHAR(200) NOT NULL,
				price DECIMAL(10, 2) NOT NULL,
				stock INTEGER DEFAULT 0,
				is_active BOOLEAN DEFAULT true,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT uq_product_sku UNIQUE (sku)
			)`
		additionalSQL = []string{
			"CREATE INDEX IF NOT EXISTS idx_products_category ON service_test_products(category_id)",
			"CREATE INDEX IF NOT EXISTS idx_products_price ON service_test_products(price)",
		}

	case config.MySQL:
		categoriesSQL = `
			CREATE TABLE IF NOT EXISTS service_test_categories (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				description TEXT,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
		productsSQL = `
			CREATE TABLE IF NOT EXISTS service_test_products (
				id INT AUTO_INCREMENT PRIMARY KEY,
				category_id INT NOT NULL,
				sku VARCHAR(50) NOT NULL,
				name VARCHAR(200) NOT NULL,
				price DECIMAL(10, 2) NOT NULL,
				stock INT DEFAULT 0,
				is_active BOOLEAN DEFAULT true,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT fk_product_category FOREIGN KEY (category_id) REFERENCES service_test_categories(id) ON DELETE CASCADE,
				UNIQUE KEY uq_product_sku (sku),
				INDEX idx_products_category (category_id),
				INDEX idx_products_price (price)
			)`

	case config.SQLite:
		categoriesSQL = `
			CREATE TABLE IF NOT EXISTS service_test_categories (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				description TEXT,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			)`
		productsSQL = `
			CREATE TABLE IF NOT EXISTS service_test_products (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				category_id INTEGER NOT NULL REFERENCES service_test_categories(id) ON DELETE CASCADE,
				sku TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				price REAL NOT NULL,
				stock INTEGER DEFAULT 0,
				is_active INTEGER DEFAULT 1,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			)`
		additionalSQL = []string{
			"CREATE INDEX IF NOT EXISTS idx_products_category ON service_test_products(category_id)",
			"CREATE INDEX IF NOT EXISTS idx_products_price ON service_test_products(price)",
		}
	}

	_, err := db.ExecContext(suite.ctx, categoriesSQL)
	suite.Require().NoError(err, "Creating service_test_categories table should succeed")

	_, err = db.ExecContext(suite.ctx, productsSQL)
	suite.Require().NoError(err, "Creating service_test_products table should succeed")

	for _, sql := range additionalSQL {
		_, _ = db.ExecContext(suite.ctx, sql)
	}
}

func (suite *ServiceTestSuite) cleanupTestTables(db *sql.DB) {
	_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS service_test_products")
	_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS service_test_categories")
}

// TestServiceTestSuite tests service test suite functionality.
func TestService(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}
