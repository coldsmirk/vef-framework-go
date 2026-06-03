package orm

import (
	"context"
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// Tx extends DB with Commit and Rollback for manual transaction control.
type Tx interface {
	DB
	// Commit commits the transaction.
	Commit() error
	// Rollback aborts the transaction and discards all changes.
	Rollback() error
}

// DB provides factory methods for creating queries and managing transactions.
type DB interface {
	// NewSelect creates a new SELECT query builder.
	NewSelect() SelectQuery
	// NewInsert creates a new INSERT query builder.
	NewInsert() InsertQuery
	// NewUpdate creates a new UPDATE query builder.
	NewUpdate() UpdateQuery
	// NewDelete creates a new DELETE query builder.
	NewDelete() DeleteQuery
	// NewMerge creates a new MERGE (UPSERT) query builder.
	NewMerge() MergeQuery
	// NewRaw creates a raw SQL query with parameter binding.
	NewRaw(query string, args ...any) RawQuery
	// NewCreateTable creates a new CREATE TABLE query builder.
	NewCreateTable() CreateTableQuery
	// NewDropTable creates a new DROP TABLE query builder.
	NewDropTable() DropTableQuery
	// NewCreateIndex creates a new CREATE INDEX query builder.
	NewCreateIndex() CreateIndexQuery
	// NewDropIndex creates a new DROP INDEX query builder.
	NewDropIndex() DropIndexQuery
	// NewTruncateTable creates a new TRUNCATE TABLE query builder.
	NewTruncateTable() TruncateTableQuery
	// NewAddColumn creates a new ALTER TABLE ADD COLUMN query builder.
	NewAddColumn() AddColumnQuery
	// NewDropColumn creates a new ALTER TABLE DROP COLUMN query builder.
	NewDropColumn() DropColumnQuery
	// RunInTx executes fn within a read-write transaction (READ COMMITTED isolation).
	// The transaction is committed if fn returns nil, rolled back otherwise.
	RunInTx(ctx context.Context, fn func(ctx context.Context, tx DB) error) error
	// RunInReadOnlyTx executes fn within a read-only transaction (READ COMMITTED isolation).
	RunInReadOnlyTx(ctx context.Context, fn func(ctx context.Context, tx DB) error) error
	// BeginTx starts a manual transaction with the given options. Caller must commit or rollback.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	// Connection acquires a dedicated database connection from the pool. It is
	// pool-scoped: calling it on a transaction-scoped DB (inside RunInTx /
	// RunInReadOnlyTx, or on a Tx) returns ErrConnectionInTx, because the
	// returned connection would not participate in the transaction.
	Connection(ctx context.Context) (*sql.Conn, error)
	// RegisterModel registers models for Bun relation mapping (e.g., many-to-many join tables).
	RegisterModel(models ...any)
	// ResetModel drops and recreates tables for the given models. Intended for testing only.
	ResetModel(ctx context.Context, models ...any) error
	// ScanRows scans all rows and closes *sql.Rows when done.
	ScanRows(ctx context.Context, rows *sql.Rows, dest ...any) error
	// ScanRow scans a single row without closing *sql.Rows.
	ScanRow(ctx context.Context, rows *sql.Rows, dest ...any) error
	// WithNamedArg returns a new DB that binds a named argument for use in raw SQL (e.g., ?name).
	WithNamedArg(name string, value any) DB
	// ModelPKs extracts primary key column names and their values from a model instance.
	ModelPKs(model any) (map[string]any, error)
	// ModelPKFields returns the primary key field descriptors for the given model.
	ModelPKFields(model any) []*PKField
	// TableOf returns the schema metadata (columns, relations, etc.) for the given model.
	TableOf(model any) *schema.Table
}

var (
	txOptions = &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	}
	readOnlyTxOptions = &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  true,
	}
)

// BunDB is a wrapper around the bun.DB type.
type BunDB struct {
	// db is the active query executor: the pool *bun.DB outside a transaction,
	// or a bun.Tx within one.
	db bun.IDB
	// bunDB is the originating pool *bun.DB. It is carried unchanged into tx
	// wrappers so pool-only operations (model schema lookups, pooled
	// connections) can reach it without recovering it from a synthetic query.
	bunDB *bun.DB
}

// newBunDB wraps a bun.IDB, recording the originating pool *bun.DB when the
// executor is a pool rather than a transaction.
func newBunDB(db bun.IDB) *BunDB {
	inst := &BunDB{db: db}
	if pool, ok := db.(*bun.DB); ok {
		inst.bunDB = pool
	}

	return inst
}

func (d *BunDB) NewSelect() SelectQuery {
	return NewSelectQuery(d)
}

func (d *BunDB) NewInsert() InsertQuery {
	return NewInsertQuery(d)
}

func (d *BunDB) NewUpdate() UpdateQuery {
	return NewUpdateQuery(d)
}

func (d *BunDB) NewDelete() DeleteQuery {
	return NewDeleteQuery(d)
}

func (d *BunDB) NewMerge() MergeQuery {
	return NewMergeQuery(d)
}

func (d *BunDB) NewRaw(query string, args ...any) RawQuery {
	return newRawQuery(d, query, args...)
}

func (d *BunDB) NewCreateTable() CreateTableQuery {
	return NewCreateTableQuery(d)
}

func (d *BunDB) NewDropTable() DropTableQuery {
	return NewDropTableQuery(d)
}

func (d *BunDB) NewCreateIndex() CreateIndexQuery {
	return NewCreateIndexQuery(d)
}

func (d *BunDB) NewDropIndex() DropIndexQuery {
	return NewDropIndexQuery(d)
}

func (d *BunDB) NewTruncateTable() TruncateTableQuery {
	return NewTruncateTableQuery(d)
}

func (d *BunDB) NewAddColumn() AddColumnQuery {
	return NewAddColumnQuery(d)
}

func (d *BunDB) NewDropColumn() DropColumnQuery {
	return NewDropColumnQuery(d)
}

func (d *BunDB) RunInTx(ctx context.Context, fn func(context.Context, DB) error) error {
	return d.runInTx(ctx, txOptions, fn)
}

func (d *BunDB) RunInReadOnlyTx(ctx context.Context, fn func(context.Context, DB) error) error {
	return d.runInTx(ctx, readOnlyTxOptions, fn)
}

func (d *BunDB) runInTx(ctx context.Context, opts *sql.TxOptions, fn func(context.Context, DB) error) error {
	return d.db.RunInTx(ctx, opts, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, &BunDB{db: tx, bunDB: d.bunDB})
	})
}

func (d *BunDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &BunTx{BunDB{db: tx, bunDB: d.bunDB}}, nil
}

func (d *BunDB) Connection(ctx context.Context) (*sql.Conn, error) {
	if d.inTx() {
		return nil, ErrConnectionInTx
	}

	return d.bunDB.DB.Conn(ctx)
}

func (d *BunDB) RegisterModel(models ...any) {
	d.getBunDB().RegisterModel(models...)
}

func (d *BunDB) ResetModel(ctx context.Context, models ...any) error {
	return d.getBunDB().ResetModel(ctx, models...)
}

func (d *BunDB) ScanRows(ctx context.Context, rows *sql.Rows, dest ...any) error {
	return d.getBunDB().ScanRows(ctx, rows, dest...)
}

func (d *BunDB) ScanRow(ctx context.Context, rows *sql.Rows, dest ...any) error {
	return d.getBunDB().ScanRow(ctx, rows, dest...)
}

func (d *BunDB) WithNamedArg(name string, value any) DB {
	if db, ok := d.db.(*bun.DB); ok {
		return newBunDB(db.WithNamedArg(name, value))
	}

	logger.Panicf("%q is not supported within a transaction context", "WithNamedArg")

	return d
}

func (d *BunDB) ModelPKs(model any) (map[string]any, error) {
	fields := d.ModelPKFields(model)
	values := make(map[string]any, len(fields))

	for _, pk := range fields {
		v, err := pk.Value(model)
		if err != nil {
			return nil, err
		}

		values[pk.Name] = v
	}

	return values, nil
}

func (d *BunDB) ModelPKFields(model any) []*PKField {
	table := getTableSchema(model, d.getBunDB())
	fields := make([]*PKField, len(table.PKs))

	for i, pk := range table.PKs {
		fields[i] = NewPKField(pk)
	}

	return fields
}

func (d *BunDB) TableOf(model any) *schema.Table {
	return getTableSchema(model, d.getBunDB())
}

// inTx reports whether this wrapper is scoped to a transaction rather than the
// connection pool.
func (d *BunDB) inTx() bool {
	_, ok := d.db.(*bun.DB)

	return !ok
}

// getBunDB returns the originating pool *bun.DB, used for operations that are
// inherently pool-scoped (model schema lookups, model registration, pooled
// connections). It is identical whether the wrapper is pool- or tx-scoped
// because the pool handle is carried into tx wrappers.
func (d *BunDB) getBunDB() *bun.DB {
	return d.bunDB
}
