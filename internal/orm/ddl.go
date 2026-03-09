package orm

import "fmt"

// CreateTableQuery builds and executes CREATE TABLE queries.
type CreateTableQuery interface {
	Executor
	TableTarget[CreateTableQuery]
	fmt.Stringer

	// Column adds a column definition with data type and optional constraints.
	Column(name string, dataType DataTypeDef, constraints ...ColumnConstraint) CreateTableQuery
	// Temp creates a temporary table that is dropped at the end of the session.
	Temp() CreateTableQuery
	// IfNotExists adds IF NOT EXISTS to skip creation when the table already exists.
	IfNotExists() CreateTableQuery
	// DefaultVarChar sets the default VARCHAR length for string columns in model-based creation.
	DefaultVarChar(n int) CreateTableQuery
	// PrimaryKey adds a composite primary key constraint using the builder DSL.
	PrimaryKey(builder func(PrimaryKeyBuilder)) CreateTableQuery
	// Unique adds a unique constraint using the builder DSL.
	Unique(builder func(UniqueBuilder)) CreateTableQuery
	// Check adds a check constraint using the builder DSL.
	Check(builder func(CheckBuilder)) CreateTableQuery
	// ForeignKey adds a foreign key constraint using the builder DSL.
	ForeignKey(builder func(ForeignKeyBuilder)) CreateTableQuery
	// PartitionBy configures table partitioning with the given strategy and columns.
	PartitionBy(strategy PartitionStrategy, columns ...string) CreateTableQuery
	// TableSpace assigns the table to a specific tablespace for storage management.
	TableSpace(tablespace string) CreateTableQuery
	// WithForeignKeys creates foreign keys from model relations.
	WithForeignKeys() CreateTableQuery
}

// DropTableQuery builds and executes DROP TABLE queries.
type DropTableQuery interface {
	Executor
	TableTarget[DropTableQuery]
	fmt.Stringer

	// IfExists adds IF EXISTS to prevent errors when the table does not exist.
	IfExists() DropTableQuery
	// Cascade automatically drops dependent objects (e.g., indexes, views).
	Cascade() DropTableQuery
	// Restrict refuses to drop the table if any dependent objects exist (default behavior).
	Restrict() DropTableQuery
}

// CreateIndexQuery builds and executes CREATE INDEX queries.
type CreateIndexQuery interface {
	Executor
	TableTarget[CreateIndexQuery]

	// Index sets the index name.
	Index(name string) CreateIndexQuery
	// Column specifies the columns to include in the index.
	Column(columns ...string) CreateIndexQuery
	// ColumnExpr adds an expression-based index column (e.g., for functional indexes).
	ColumnExpr(builder func(ExprBuilder) any) CreateIndexQuery
	// ExcludeColumn removes columns from the model's default index column set.
	ExcludeColumn(columns ...string) CreateIndexQuery
	// Unique creates a unique index that enforces uniqueness on the indexed columns.
	Unique() CreateIndexQuery
	// Concurrently creates the index concurrently (PostgreSQL only).
	Concurrently() CreateIndexQuery
	// IfNotExists skips creation when the index already exists.
	IfNotExists() CreateIndexQuery
	// Include adds covering columns (PostgreSQL INCLUDE clause).
	Include(columns ...string) CreateIndexQuery
	// Using sets the index method (e.g., BTREE, HASH, GIN, GiST).
	Using(method IndexMethod) CreateIndexQuery
	// Where adds a partial index condition to only index rows matching the condition.
	Where(builder func(ConditionBuilder)) CreateIndexQuery
}

// DropIndexQuery builds and executes DROP INDEX queries.
type DropIndexQuery interface {
	Executor

	// Index sets the index name to drop.
	Index(name string) DropIndexQuery
	// IfExists prevents errors when the index does not exist.
	IfExists() DropIndexQuery
	// Concurrently drops the index concurrently (PostgreSQL only).
	Concurrently() DropIndexQuery
	// Cascade automatically drops dependent objects.
	Cascade() DropIndexQuery
	// Restrict refuses to drop the index if any dependent objects exist.
	Restrict() DropIndexQuery
}

// TruncateTableQuery builds and executes TRUNCATE TABLE queries.
type TruncateTableQuery interface {
	Executor
	TableTarget[TruncateTableQuery]

	// ContinueIdentity preserves the current sequence values (default behavior, opposite of RESTART IDENTITY).
	ContinueIdentity() TruncateTableQuery
	// Cascade automatically truncates dependent tables via foreign key references.
	Cascade() TruncateTableQuery
	// Restrict refuses to truncate if other tables reference this one via foreign keys.
	Restrict() TruncateTableQuery
}

// AddColumnQuery builds and executes ALTER TABLE ADD COLUMN queries.
type AddColumnQuery interface {
	Executor
	TableTarget[AddColumnQuery]

	// Column defines the new column with data type and optional constraints.
	Column(name string, dataType DataTypeDef, constraints ...ColumnConstraint) AddColumnQuery
	// IfNotExists skips the addition when the column already exists.
	IfNotExists() AddColumnQuery
}

// DropColumnQuery builds and executes ALTER TABLE DROP COLUMN queries.
type DropColumnQuery interface {
	Executor
	TableTarget[DropColumnQuery]

	// Column specifies the columns to drop.
	Column(columns ...string) DropColumnQuery
}
