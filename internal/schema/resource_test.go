package schema_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/schema"
	"github.com/coldsmirk/vef-framework-go/security"
)

// SchemaResourceTestSuite tests the schema API resource functionality.
type SchemaResourceTestSuite struct {
	apptest.Suite

	ctx               context.Context
	postgresContainer *testx.PostgresContainer
	mysqlContainer    *testx.MySQLContainer
}

func (s *SchemaResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.postgresContainer = testx.NewPostgresContainer(s.ctx, s.T())
	s.mysqlContainer = testx.NewMySQLContainer(s.ctx, s.T())
}

func (s *SchemaResourceTestSuite) TestPostgresResource() {
	s.T().Log("Testing Schema Resource for PostgreSQL")
	s.runResourceTests(s.postgresContainer.DataSource, "PostgreSQL")
}

func (s *SchemaResourceTestSuite) TestMySQLResource() {
	s.T().Log("Testing Schema Resource for MySQL")
	s.runResourceTests(s.mysqlContainer.DataSource, "MySQL")
}

func (s *SchemaResourceTestSuite) TestSQLiteResource() {
	s.T().Log("Testing Schema Resource for SQLite")

	dsConfig := &config.DataSourceConfig{
		Kind: config.SQLite,
	}

	s.runResourceTests(dsConfig, "SQLite")
}

func (s *SchemaResourceTestSuite) runResourceTests(dsConfig *config.DataSourceConfig, dbKind string) {
	var sqlDB *sql.DB

	s.SetupApp(
		fx.Replace(
			&config.DataSourcesConfig{
				Map: map[string]config.DataSourceConfig{
					"primary": *dsConfig,
				},
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		fx.Populate(&sqlDB),
	)

	defer s.TearDownApp()

	token := s.GenerateToken(security.NewUser("test-admin", "admin"))

	s.setupTestTables(sqlDB, dsConfig.Kind)
	defer s.cleanupTestTables(sqlDB, dsConfig.Kind)

	s.Run("ListTables", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/schema",
				Action:   "list_tables",
				Version:  "v1",
			},
		}, token)

		s.Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "list_tables should succeed")

		tables := s.ReadDataAsSlice(body.Data)

		tableNames := make([]string, 0, len(tables))
		for _, t := range tables {
			tableMap, ok := t.(map[string]any)
			if ok {
				if name, exists := tableMap["name"]; exists {
					tableNames = append(tableNames, name.(string))
				}
			}
		}

		s.T().Logf("%s tables found via API: %v", dbKind, tableNames)
		s.Contains(tableNames, "resource_test_orders", "Should find resource_test_orders table")
		s.Contains(tableNames, "resource_test_items", "Should find resource_test_items table")
	})

	s.Run("GetTableSchemaSuccess", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/schema",
				Action:   "get_table_schema",
				Version:  "v1",
			},
			Params: map[string]any{
				"name": "resource_test_orders",
			},
		}, token)

		s.Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "get_table_schema should succeed")

		tableSchema := s.ReadDataAsMap(body.Data)

		s.Equal("resource_test_orders", tableSchema["name"], "Table name should match")

		columns, ok := tableSchema["columns"].([]any)
		s.True(ok, "Columns should be an array")
		s.NotEmpty(columns, "Columns should not be empty")

		columnNames := make([]string, 0, len(columns))
		for _, col := range columns {
			colMap, ok := col.(map[string]any)
			if ok {
				if name, exists := colMap["name"]; exists {
					columnNames = append(columnNames, name.(string))
				}
			}
		}

		s.T().Logf("%s resource_test_orders columns via API: %v", dbKind, columnNames)
		s.Contains(columnNames, "id", "Should have id column")
		s.Contains(columnNames, "customer_name", "Should have customer_name column")
		s.Contains(columnNames, "total_amount", "Should have total_amount column")
	})

	s.Run("GetTableSchemaWithPrimaryKey", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/schema",
				Action:   "get_table_schema",
				Version:  "v1",
			},
			Params: map[string]any{
				"name": "resource_test_orders",
			},
		}, token)

		body := s.ReadResult(resp)
		tableSchema := s.ReadDataAsMap(body.Data)

		pk, hasPK := tableSchema["primaryKey"]
		s.True(hasPK, "Should have primaryKey")
		s.NotNil(pk, "PrimaryKey should not be nil")

		pkMap, ok := pk.(map[string]any)
		s.True(ok, "PrimaryKey should be a map")

		pkColumns, ok := pkMap["columns"].([]any)
		s.True(ok, "PrimaryKey columns should be an array")
		s.NotEmpty(pkColumns, "PrimaryKey columns should not be empty")

		s.T().Logf("%s resource_test_orders primary key via API: %v", dbKind, pkColumns)
	})

	s.Run("GetTableSchemaNotFound", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/schema",
				Action:   "get_table_schema",
				Version:  "v1",
			},
			Params: map[string]any{
				"name": "nonexistent_table_xyz",
			},
		}, token)

		s.Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK (error in body)")

		body := s.ReadResult(resp)
		s.False(body.IsOk(), "get_table_schema should fail for nonexistent table")
		s.Equal(schema.ErrCodeTableNotFound, body.Code, "Error code should be ErrCodeTableNotFound")

		s.T().Logf("%s table not found error: code=%d, message=%s", dbKind, body.Code, body.Message)
	})

	s.Run("GetTableSchemaValidationError", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/schema",
				Action:   "get_table_schema",
				Version:  "v1",
			},
			Params: map[string]any{
				// Missing required "name" parameter
			},
		}, token)

		body := s.ReadResult(resp)
		s.False(body.IsOk(), "get_table_schema should fail without name parameter")

		s.T().Logf("%s validation error: code=%d, message=%s", dbKind, body.Code, body.Message)
	})

	s.Run("ListViews", func() {
		resp := s.MakeRPCRequestWithToken(api.Request{
			Identifier: api.Identifier{
				Resource: "sys/schema",
				Action:   "list_views",
				Version:  "v1",
			},
		}, token)

		s.Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

		body := s.ReadResult(resp)
		s.True(body.IsOk(), "list_views should succeed")

		views := s.ReadDataAsSlice(body.Data)

		viewNames := make([]string, 0, len(views))
		for _, view := range views {
			viewMap, ok := view.(map[string]any)
			if ok {
				if name, exists := viewMap["name"]; exists {
					viewNames = append(viewNames, name.(string))
				}
			}
		}

		s.T().Logf("%s views found via API: %v", dbKind, viewNames)
		s.Contains(viewNames, "resource_test_order_view", "Should find resource_test_order_view")
	})
}

func (s *SchemaResourceTestSuite) setupTestTables(db *sql.DB, dbKind config.DBKind) {
	var ordersSQL, itemsSQL, viewSQL string

	switch dbKind {
	case config.Postgres:
		ordersSQL = `
			CREATE TABLE IF NOT EXISTS resource_test_orders (
				id SERIAL PRIMARY KEY,
				customer_name VARCHAR(100) NOT NULL,
				total_amount DECIMAL(10, 2) NOT NULL DEFAULT 0,
				status VARCHAR(20) DEFAULT 'pending',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
		itemsSQL = `
			CREATE TABLE IF NOT EXISTS resource_test_items (
				id SERIAL PRIMARY KEY,
				order_id INTEGER NOT NULL REFERENCES resource_test_orders(id) ON DELETE CASCADE,
				product_name VARCHAR(200) NOT NULL,
				quantity INTEGER NOT NULL DEFAULT 1,
				unit_price DECIMAL(10, 2) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
		viewSQL = `
			CREATE OR REPLACE VIEW resource_test_order_view AS
			SELECT id, customer_name, total_amount FROM resource_test_orders`

	case config.MySQL:
		ordersSQL = `
			CREATE TABLE IF NOT EXISTS resource_test_orders (
				id INT AUTO_INCREMENT PRIMARY KEY,
				customer_name VARCHAR(100) NOT NULL,
				total_amount DECIMAL(10, 2) NOT NULL DEFAULT 0,
				status VARCHAR(20) DEFAULT 'pending',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`
		itemsSQL = `
			CREATE TABLE IF NOT EXISTS resource_test_items (
				id INT AUTO_INCREMENT PRIMARY KEY,
				order_id INT NOT NULL,
				product_name VARCHAR(200) NOT NULL,
				quantity INT NOT NULL DEFAULT 1,
				unit_price DECIMAL(10, 2) NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT fk_items_order FOREIGN KEY (order_id) REFERENCES resource_test_orders(id) ON DELETE CASCADE,
				INDEX idx_items_order (order_id)
			)`
		viewSQL = `
			CREATE OR REPLACE VIEW resource_test_order_view AS
			SELECT id, customer_name, total_amount FROM resource_test_orders`

	case config.SQLite:
		ordersSQL = `
			CREATE TABLE IF NOT EXISTS resource_test_orders (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				customer_name TEXT NOT NULL,
				total_amount REAL NOT NULL DEFAULT 0,
				status TEXT DEFAULT 'pending',
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			)`
		itemsSQL = `
			CREATE TABLE IF NOT EXISTS resource_test_items (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				order_id INTEGER NOT NULL REFERENCES resource_test_orders(id) ON DELETE CASCADE,
				product_name TEXT NOT NULL,
				quantity INTEGER NOT NULL DEFAULT 1,
				unit_price REAL NOT NULL,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP
			)`
		viewSQL = `
			CREATE VIEW IF NOT EXISTS resource_test_order_view AS
			SELECT id, customer_name, total_amount FROM resource_test_orders`
	}

	_, err := db.ExecContext(s.ctx, ordersSQL)
	s.Require().NoError(err, "Creating resource_test_orders table should succeed")

	_, err = db.ExecContext(s.ctx, itemsSQL)
	s.Require().NoError(err, "Creating resource_test_items table should succeed")

	_, err = db.ExecContext(s.ctx, viewSQL)
	s.Require().NoError(err, "Creating resource_test_order_view view should succeed")
}

func (s *SchemaResourceTestSuite) cleanupTestTables(db *sql.DB, _ config.DBKind) {
	_, _ = db.ExecContext(s.ctx, "DROP VIEW IF EXISTS resource_test_order_view")
	_, _ = db.ExecContext(s.ctx, "DROP TABLE IF EXISTS resource_test_items")
	_, _ = db.ExecContext(s.ctx, "DROP TABLE IF EXISTS resource_test_orders")
}

// TestSchemaResourceTestSuite tests schema resource test suite functionality.
func TestSchemaResource(t *testing.T) {
	suite.Run(t, new(SchemaResourceTestSuite))
}
