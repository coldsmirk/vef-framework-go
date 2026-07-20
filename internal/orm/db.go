package orm

import (
	"context"
	"database/sql"
	"errors"

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
	// InTx reports whether this handle is transaction-scoped (obtained inside
	// RunInTx / RunInReadOnlyTx or from BeginTx) rather than pool-scoped.
	InTx() bool
	// RunInReadOnlyTx executes fn within a read-only transaction (READ COMMITTED isolation).
	RunInReadOnlyTx(ctx context.Context, fn func(ctx context.Context, tx DB) error) error
	// BeginTx starts a manual transaction with the given options. Caller must commit or rollback.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	// RunOnConnection executes fn on one dedicated connection. Nested calls reuse
	// that connection. Calling it from a transaction-scoped DB returns
	// ErrRunOnConnectionInTx because the transaction already owns its connection.
	RunOnConnection(ctx context.Context, fn func(ctx context.Context, db DB) error) error
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

type dbScope uint8

const (
	dbScopePool dbScope = iota
	dbScopeConnection
	dbScopeTransaction
)

// BunDB is a wrapper around the bun.DB type.
type BunDB struct {
	// db is the active query executor for the wrapper's explicit scope.
	db bun.IDB
	// bunDB is the originating pool handle used for pool-only metadata operations.
	bunDB *bun.DB
	scope dbScope
}

func newBunDB(db *bun.DB) *BunDB {
	return newScopedBunDB(db, db, dbScopePool)
}

func newScopedBunDB(db bun.IDB, bunDB *bun.DB, scope dbScope) *BunDB {
	return &BunDB{db: db, bunDB: bunDB, scope: scope}
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
		return fn(ctx, newScopedBunDB(tx, d.bunDB, dbScopeTransaction))
	})
}

func (d *BunDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &BunTx{BunDB: *newScopedBunDB(tx, d.bunDB, dbScopeTransaction)}, nil
}

func (d *BunDB) RunOnConnection(ctx context.Context, fn func(context.Context, DB) error) (err error) {
	switch d.scope {
	case dbScopeConnection:
		return fn(ctx, d)
	case dbScopeTransaction:
		return ErrRunOnConnectionInTx
	case dbScopePool:
		// Acquire the connection below.
	default:
		panic("orm: invalid database scope")
	}

	conn, err := d.bunDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, conn.Close())
	}()

	return fn(ctx, newScopedBunDB(conn, d.bunDB, dbScopeConnection))
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
	if d.scope == dbScopePool {
		return newBunDB(d.bunDB.WithNamedArg(name, value))
	}

	logger.Panicf("%q is only supported on a pool-scoped DB", "WithNamedArg")

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

func (d *BunDB) InTx() bool {
	return d.scope == dbScopeTransaction
}

func (d *BunDB) getBunDB() *bun.DB {
	return d.bunDB
}
