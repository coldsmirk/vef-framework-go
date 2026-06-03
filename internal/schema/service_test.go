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
	db, err := database.Open(*dsConfig)
	suite.Require().NoError(err, "Database connection should succeed")

	defer func() {
		suite.Require().NoError(db.Close(), "Database should close without error")
	}()

	suite.setupTestTables(db, dsConfig.Kind)

	defer suite.cleanupTestTables(db)

	svc, err := schema.NewService(db, &config.DataSourcesConfig{
		Map: map[string]config.DataSourceConfig{"primary": *dsConfig},
	})
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

		indexColumns := make([][]string, len(tableSchema.Indexes))
		for i, idx := range tableSchema.Indexes {
			indexColumns[i] = idx.Columns
		}

		suite.NotEmpty(tableSchema.Indexes, "service_test_products should report at least one index")
		suite.Contains(indexColumns, []string{"category_id"}, "Should report the category_id index")
		suite.Contains(indexColumns, []string{"price"}, "Should report the price index")

		uniqueColumns := make([][]string, len(tableSchema.UniqueKeys))
		for i, uk := range tableSchema.UniqueKeys {
			uniqueColumns[i] = uk.Columns
		}

		suite.NotEmpty(tableSchema.UniqueKeys, "service_test_products should report the sku unique key")
		suite.Contains(uniqueColumns, []string{"sku"}, "Unique key should cover the sku column")
	})

	suite.Run("GetTableSchemaWithForeignKeys", func() {
		tableSchema, err := svc.GetTableSchema(suite.ctx, "service_test_products")
		suite.NoError(err, "GetTableSchema should succeed")

		suite.Require().Len(tableSchema.ForeignKeys, 1, "service_test_products should report exactly one foreign key")

		fk := tableSchema.ForeignKeys[0]
		suite.Equal([]string{"category_id"}, fk.Columns, "FK should be on category_id")
		suite.Equal("service_test_categories", fk.RefTable, "FK should reference service_test_categories")
		suite.Equal([]string{"id"}, fk.RefColumns, "FK should reference the id column")
		suite.Equal("CASCADE", fk.OnDelete, "FK should cascade on delete")
	})

	suite.Run("ListViews", func() {
		views, err := svc.ListViews(suite.ctx)
		suite.NoError(err, "ListViews should succeed")
		suite.NotEmpty(views, "Views list should not be empty")

		viewNames := make([]string, len(views))
		for i, view := range views {
			viewNames[i] = view.Name
		}

		suite.T().Logf("%s views: %v", dbKind, viewNames)
		suite.Contains(viewNames, "service_test_product_view", "Should find service_test_product_view")
	})

	suite.Run("GetTableSchemaNotFound", func() {
		_, err := svc.GetTableSchema(suite.ctx, "nonexistent_table_xyz")
		suite.Error(err, "GetTableSchema should return error for nonexistent table")
	})

	if dbKind == "PostgreSQL" {
		suite.Run("GetTableSchemaWithIdentityColumn", func() {
			_, err := db.ExecContext(suite.ctx, `
				CREATE TABLE IF NOT EXISTS service_test_identity (
					id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
					name VARCHAR(100) NOT NULL
				)`)
			suite.Require().NoError(err, "Creating service_test_identity table should succeed")

			defer func() {
				_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS service_test_identity")
			}()

			tableSchema, err := svc.GetTableSchema(suite.ctx, "service_test_identity")
			suite.Require().NoError(err, "GetTableSchema should succeed")

			var idCol pkgschema.Column
			for _, col := range tableSchema.Columns {
				if col.Name == "id" {
					idCol = col
				}
			}

			suite.Equal("id", idCol.Name, "Should find the id column")
			suite.True(idCol.IsAutoIncrement, "GENERATED ALWAYS AS IDENTITY id should be auto increment")
		})
	}
}

func (suite *ServiceTestSuite) setupTestTables(db *sql.DB, dbKind config.DBKind) {
	var (
		categoriesSQL, productsSQL, viewSQL string
		additionalSQL                       []string
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
		viewSQL = `
			CREATE OR REPLACE VIEW service_test_product_view AS
			SELECT id, sku, name FROM service_test_products`

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
		viewSQL = `
			CREATE OR REPLACE VIEW service_test_product_view AS
			SELECT id, sku, name FROM service_test_products`

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
		viewSQL = `
			CREATE VIEW IF NOT EXISTS service_test_product_view AS
			SELECT id, sku, name FROM service_test_products`
	}

	_, err := db.ExecContext(suite.ctx, categoriesSQL)
	suite.Require().NoError(err, "Creating service_test_categories table should succeed")

	_, err = db.ExecContext(suite.ctx, productsSQL)
	suite.Require().NoError(err, "Creating service_test_products table should succeed")

	for _, sql := range additionalSQL {
		_, _ = db.ExecContext(suite.ctx, sql)
	}

	_, err = db.ExecContext(suite.ctx, viewSQL)
	suite.Require().NoError(err, "Creating service_test_product_view view should succeed")
}

func (suite *ServiceTestSuite) cleanupTestTables(db *sql.DB) {
	_, _ = db.ExecContext(suite.ctx, "DROP VIEW IF EXISTS service_test_product_view")
	_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS service_test_products")
	_, _ = db.ExecContext(suite.ctx, "DROP TABLE IF EXISTS service_test_categories")
}

func TestService(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}
