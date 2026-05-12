package orm_test

import (
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CreateTableTestSuite{BaseTestSuite: base}
	})
}

// DDLModel is a dedicated test model for DDL operations, shared across DDL test suites.
type DDLModel struct {
	bun.BaseModel `bun:"table:test_ddl_model,alias:dm"`
	orm.Model

	Label string `json:"label" bun:"label,notnull"`
	Score int    `json:"score" bun:"score,notnull,default:0"`
}

// CreateTableTestSuite tests CreateTable operations across all databases.
type CreateTableTestSuite struct {
	*BaseTestSuite
}

func (suite *CreateTableTestSuite) identQuote() string {
	if suite.ds.Kind == config.MySQL {
		return "`"
	}

	return `"`
}

func (suite *CreateTableTestSuite) SetupSuite() {
	suite.db.RegisterModel((*DDLModel)(nil))
}

// TestCreateAndDrop tests creating and dropping a table via the orm.DB interface.
func (suite *CreateTableTestSuite) TestCreateAndDrop() {
	suite.T().Logf("Testing CreateTable for %s", suite.ds.Kind)

	suite.Run("CreateIfNotExists", func() {
		_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
		suite.Require().NoError(err, "Should drop table if it exists")

		_, err = suite.db.NewCreateTable().Model((*DDLModel)(nil)).IfNotExists().Exec(suite.ctx)
		suite.NoError(err, "Should create table with IfNotExists")

		_, err = suite.db.NewInsert().
			Model(&DDLModel{Label: "hello", Score: 42}).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert into newly created table")
	})

	suite.Run("IdempotentCreate", func() {
		_, err := suite.db.NewCreateTable().Model((*DDLModel)(nil)).IfNotExists().Exec(suite.ctx)
		suite.NoError(err, "Should be idempotent with IfNotExists")
	})

	suite.Run("DropIfExists", func() {
		_, err := suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
		suite.NoError(err, "Should drop existing table")

		_, err = suite.db.NewDropTable().Model((*DDLModel)(nil)).IfExists().Exec(suite.ctx)
		suite.NoError(err, "Should be idempotent with IfExists")
	})
}

// TestString tests String() output for CreateTable.
func (suite *CreateTableTestSuite) TestString() {
	suite.T().Logf("Testing CreateTable String for %s", suite.ds.Kind)

	sql := suite.db.NewCreateTable().
		Model((*DDLModel)(nil)).
		IfNotExists().
		String()
	suite.Contains(sql, "CREATE TABLE", "Should contain CREATE TABLE keyword")
	suite.Contains(sql, "IF NOT EXISTS", "Should contain IF NOT EXISTS clause")
}

// TestExtended tests extended CreateTable methods.
func (suite *CreateTableTestSuite) TestExtended() {
	suite.T().Logf("Testing CreateTable extended methods for %s", suite.ds.Kind)

	suite.Run("VarCharAndForeignKey", func() {
		query := suite.db.NewCreateTable().
			Model((*Tag)(nil)).
			Table("test_ddl_temp").
			IfNotExists().
			DefaultVarChar(100).
			WithForeignKeys()

		suite.NotNil(query, "Should return non-nil query")
	})

	suite.Run("ExtraColumn", func() {
		query := suite.db.NewCreateTable().
			Model((*Tag)(nil)).
			Column("extra_col", orm.DataType.Text())

		suite.NotNil(query, "Should return non-nil query with extra column")
	})

	suite.Run("ColumnWithConstraints", func() {
		query := suite.db.NewCreateTable().
			Table("test_ddl_typed").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey(), orm.AutoIncrement()).
			Column("name", orm.DataType.VarChar(200), orm.NotNull()).
			Column("price", orm.DataType.Numeric(10, 2), orm.NotNull(), orm.Default(0)).
			Column("active", orm.DataType.Boolean(), orm.Default(true)).
			IfNotExists()

		suite.NotNil(query, "Should return non-nil query with typed columns")
		suite.Contains(query.String(), "CREATE TABLE", "Should contain CREATE TABLE keyword")
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("PartitionByAndTableSpace", func() {
			query := suite.db.NewCreateTable().
				Model((*Tag)(nil)).
				Table("test_ddl_partition").
				PartitionBy(orm.PartitionRange, "name").
				TableSpace("pg_default")

			suite.NotNil(query, "Should return non-nil query with partitioning")
		})

		suite.Run("ForeignKey", func() {
			query := suite.db.NewCreateTable().
				Model((*Tag)(nil)).
				Table("test_ddl_fk").
				ForeignKey(func(fk orm.ForeignKeyBuilder) {
					fk.Columns("name").References("test_employee", "name")
				})

			suite.NotNil(query, "Should return non-nil query with foreign key")
		})

		suite.Run("ForeignKeyWithActions", func() {
			query := suite.db.NewCreateTable().
				Table("test_ddl_fk_actions").
				Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
				Column("parent_id", orm.DataType.BigInt()).
				ForeignKey(func(fk orm.ForeignKeyBuilder) {
					fk.Columns("parent_id").
						References("test_ddl_fk_actions", "id").
						OnDelete(orm.ReferenceCascade).
						OnUpdate(orm.ReferenceRestrict)
				}).
				IfNotExists()

			suite.NotNil(query, "Should return non-nil query with FK actions")
		})
	}

	suite.Run("InlineReferences", func() {
		query := suite.db.NewCreateTable().
			Table("test_ddl_inline_fk").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("tag_id", orm.DataType.BigInt(), orm.References("test_tag", "id")).
			IfNotExists()

		suite.NotNil(query, "Should return non-nil query with inline references")
	})
}

// TestConstraint tests table-level constraints (composite PK, composite unique, named FK, CHECK).
func (suite *CreateTableTestSuite) TestConstraint() {
	suite.T().Logf("Testing CreateTable constraints for %s", suite.ds.Kind)

	suite.Run("CompositePrimaryKey", func() {
		tableName := "test_ddl_composite_pk"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("user_id", orm.DataType.BigInt(), orm.NotNull()).
			Column("product_id", orm.DataType.BigInt(), orm.NotNull()).
			Column("quantity", orm.DataType.Integer(), orm.Default(1)).
			PrimaryKey(func(pk orm.PrimaryKeyBuilder) {
				pk.Columns("user_id", "product_id")
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with composite primary key")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("NamedPrimaryKey", func() {
		tableName := "test_ddl_named_pk"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("order_id", orm.DataType.BigInt(), orm.NotNull()).
			Column("line_id", orm.DataType.Integer(), orm.NotNull()).
			PrimaryKey(func(pk orm.PrimaryKeyBuilder) {
				pk.Columns("order_id", "line_id").Name("pk_order_lines")
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with named composite PK")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("CompositeUnique", func() {
		tableName := "test_ddl_composite_uq"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("email", orm.DataType.VarChar(200), orm.NotNull()).
			Column("tenant_id", orm.DataType.BigInt(), orm.NotNull()).
			Unique(func(u orm.UniqueBuilder) {
				u.Columns("email", "tenant_id")
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with composite unique constraint")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("NamedUnique", func() {
		tableName := "test_ddl_named_uq"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("name", orm.DataType.VarChar(100), orm.NotNull()).
			Column("code", orm.DataType.VarChar(50), orm.NotNull()).
			Unique(func(u orm.UniqueBuilder) {
				u.Columns("name", "code").Name("uq_name_code")
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with named composite unique")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("MultipleConstraints", func() {
		tableName := "test_ddl_multi_constraints"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("tenant_id", orm.DataType.BigInt(), orm.NotNull()).
			Column("user_id", orm.DataType.BigInt(), orm.NotNull()).
			Column("email", orm.DataType.VarChar(200), orm.NotNull()).
			PrimaryKey(func(pk orm.PrimaryKeyBuilder) {
				pk.Columns("tenant_id", "user_id")
			}).
			Unique(func(u orm.UniqueBuilder) {
				u.Columns("tenant_id", "email").Name("uq_tenant_email")
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with PK and unique constraints")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("NamedForeignKey", func() {
		tableName := "test_ddl_named_fk"
		parentTable := "test_ddl_named_fk_parent"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
		_, _ = suite.db.NewDropTable().Table(parentTable).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(parentTable).
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			IfNotExists().
			Exec(suite.ctx)
		suite.Require().NoError(err, "Should create parent table")

		_, err = suite.db.NewCreateTable().
			Table(tableName).
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("parent_id", orm.DataType.BigInt(), orm.NotNull()).
			ForeignKey(func(fk orm.ForeignKeyBuilder) {
				fk.Name("fk_child_parent").
					Columns("parent_id").
					References(parentTable, "id").
					OnDelete(orm.ReferenceCascade)
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with named foreign key")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
		_, _ = suite.db.NewDropTable().Table(parentTable).IfExists().Exec(suite.ctx)
	})

	suite.Run("CheckConstraint", func() {
		tableName := "test_ddl_check"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("min_val", orm.DataType.Integer(), orm.NotNull()).
			Column("max_val", orm.DataType.Integer(), orm.NotNull()).
			Check(func(ck orm.CheckBuilder) {
				ck.Condition(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualColumn("min_val", "max_val")
				})
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with CHECK constraint")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("NamedCheckConstraint", func() {
		tableName := "test_ddl_named_check"
		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table(tableName).
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("start_date", orm.DataType.Timestamp(), orm.NotNull()).
			Column("end_date", orm.DataType.Timestamp(), orm.NotNull()).
			Check(func(ck orm.CheckBuilder) {
				ck.Name("ck_date_range").
					Condition(func(cb orm.ConditionBuilder) {
						cb.LessThanColumn("start_date", "end_date")
					})
			}).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with named CHECK constraint")

		_, _ = suite.db.NewDropTable().Table(tableName).IfExists().Exec(suite.ctx)
	})

	suite.Run("ConstraintStringOutput", func() {
		sql := suite.db.NewCreateTable().
			Table("orders").
			Column("user_id", orm.DataType.BigInt(), orm.NotNull()).
			Column("product_id", orm.DataType.BigInt(), orm.NotNull()).
			PrimaryKey(func(pk orm.PrimaryKeyBuilder) {
				pk.Columns("user_id", "product_id").Name("pk_orders")
			}).
			Unique(func(u orm.UniqueBuilder) {
				u.Columns("product_id", "user_id").Name("uq_product_user")
			}).
			String()
		suite.Contains(sql, "PRIMARY KEY", "Should contain PRIMARY KEY clause")
		suite.Contains(sql, "UNIQUE", "Should contain UNIQUE clause")
		suite.Contains(sql, "pk_orders", "Should contain PK constraint name")
		suite.Contains(sql, "uq_product_user", "Should contain unique constraint name")
	})
}

// TestColumnLevelCheck tests creating a table with column-level CHECK constraints.
func (suite *CreateTableTestSuite) TestColumnLevelCheck() {
	suite.T().Logf("Testing CreateTable column-level CHECK for %s", suite.ds.Kind)

	suite.Run("ColumnWithCheck", func() {
		_, _ = suite.db.NewDropTable().Table("test_ddl_col_check").IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table("test_ddl_col_check").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("age", orm.DataType.Integer(), orm.NotNull(), orm.Check(func(cb orm.ConditionBuilder) {
				cb.GreaterThan("age", 0)
			})).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with column-level CHECK")

		_, _ = suite.db.NewDropTable().Table("test_ddl_col_check").IfExists().Exec(suite.ctx)
	})

	suite.Run("CheckWithoutCondition", func() {
		// When conditionBuilder is nil, Check should be a no-op
		q := suite.db.NewCreateTable().
			Table("test_ddl_no_check").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Check(func(orm.CheckBuilder) {}).
			IfNotExists()
		suite.NotNil(q, "Should return non-nil query when Check has no condition")
	})
}

// TestRawCreateSQL tests model-less table creation with Temp and TableSpace.
func (suite *CreateTableTestSuite) TestRawCreateSQL() {
	suite.T().Logf("Testing CreateTable raw SQL for %s", suite.ds.Kind)

	suite.Run("TempTable", func() {
		sql := suite.db.NewCreateTable().
			Table("test_ddl_temp_raw").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Temp().
			IfNotExists().
			String()
		suite.Contains(sql, "TEMPORARY", "Should contain TEMPORARY keyword")
		suite.Contains(sql, "IF NOT EXISTS", "Should contain IF NOT EXISTS clause")
	})

	suite.Run("TableSpace", func() {
		quote := suite.identQuote()
		sql := suite.db.NewCreateTable().
			Table("test_ddl_ts_raw").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			TableSpace("pg_default").
			String()
		suite.Contains(sql, "TABLESPACE", "Should contain TABLESPACE keyword")
		suite.Contains(sql, "TABLESPACE "+quote+"pg_default"+quote, "Should quote tablespace name")
	})

	suite.Run("TableSpaceUnsafeName", func() {
		quote := suite.identQuote()
		tableSpace := "pg_default; DROP TABLE y; --"
		sql := suite.db.NewCreateTable().
			Table("test_ddl_ts_raw").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			TableSpace(tableSpace).
			String()
		suite.Contains(sql, "TABLESPACE "+quote+tableSpace+quote, "Should quote unsafe tablespace identifier")
	})

	suite.Run("PartitionBy", func() {
		quote := suite.identQuote()
		sql := suite.db.NewCreateTable().
			Table("test_ddl_part_raw").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("created_at", orm.DataType.Timestamp(), orm.NotNull()).
			PartitionBy(orm.PartitionRange, "created_at").
			String()
		suite.Contains(sql, "PARTITION BY", "Should contain PARTITION BY clause")
		suite.Contains(sql, "RANGE ("+quote+"created_at"+quote+")", "Should quote partition columns")
	})
}

// TestExplicitColumns tests creating a table with explicit type-safe column definitions.
func (suite *CreateTableTestSuite) TestExplicitColumns() {
	suite.T().Logf("Testing CreateTable explicit columns for %s", suite.ds.Kind)

	suite.Run("BasicTypedTable", func() {
		_, _ = suite.db.NewDropTable().Table("test_ddl_typed_basic").IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table("test_ddl_typed_basic").
			Column("id", orm.DataType.BigInt(), orm.PrimaryKey()).
			Column("name", orm.DataType.VarChar(100), orm.NotNull()).
			Column("description", orm.DataType.Text(), orm.Nullable()).
			Column("score", orm.DataType.Integer(), orm.NotNull(), orm.Default(0)).
			Column("is_active", orm.DataType.Boolean(), orm.Default(true)).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with typed columns")

		_, _ = suite.db.NewDropTable().Table("test_ddl_typed_basic").IfExists().Exec(suite.ctx)
	})

	suite.Run("NumericAndDateTypes", func() {
		_, _ = suite.db.NewDropTable().Table("test_ddl_typed_numeric").IfExists().Exec(suite.ctx)

		_, err := suite.db.NewCreateTable().
			Table("test_ddl_typed_numeric").
			Column("id", orm.DataType.Integer(), orm.PrimaryKey()).
			Column("amount", orm.DataType.Numeric(10, 2), orm.NotNull()).
			Column("created_at", orm.DataType.Timestamp()).
			Column("data", orm.DataType.JSON()).
			IfNotExists().
			Exec(suite.ctx)
		suite.NoError(err, "Should create table with numeric and date columns")

		_, _ = suite.db.NewDropTable().Table("test_ddl_typed_numeric").IfExists().Exec(suite.ctx)
	})
}

// TestFluentChaining verifies that CreateTable queries support fluent method chaining.
func (suite *CreateTableTestSuite) TestFluentChaining() {
	suite.T().Logf("Testing CreateTable fluent chaining for %s", suite.ds.Kind)

	q := suite.db.NewCreateTable().
		Model((*DDLModel)(nil)).
		IfNotExists().
		Temp()
	suite.NotNil(q, "Should support fluent method chaining")
}
