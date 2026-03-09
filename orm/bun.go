package orm

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

type (
	BaseModel         = bun.BaseModel
	BeforeScanRowHook = bun.BeforeScanRowHook
	AfterScanRowHook  = bun.AfterScanRowHook
	BunSelectQuery    = bun.SelectQuery
	BunInsertQuery    = bun.InsertQuery
	BunUpdateQuery    = bun.UpdateQuery
	BunDeleteQuery    = bun.DeleteQuery
	Table             = schema.Table
	Field             = schema.Field
	Relation          = schema.Relation
	Dialect           = schema.Dialect
)

// BeforeSelectHook is called before a SELECT query is executed.
// Implement this on model structs to modify the query or add conditions before data is fetched.
type BeforeSelectHook interface {
	bun.BeforeSelectHook
	// BeforeSelect is invoked with the query context and the select query about to be executed.
	BeforeSelect(ctx context.Context, query *BunSelectQuery) error
}

// AfterSelectHook is called after a SELECT query is executed.
// Implement this on model structs to post-process fetched data or perform side effects.
type AfterSelectHook interface {
	bun.AfterSelectHook
	// AfterSelect is invoked with the query context and the select query that was executed.
	AfterSelect(ctx context.Context, query *BunSelectQuery) error
}

// BeforeInsertHook is called before an INSERT query is executed.
// Implement this on model structs to set default values or validate data before insertion.
type BeforeInsertHook interface {
	bun.BeforeInsertHook
	// BeforeInsert is invoked with the query context and the insert query about to be executed.
	BeforeInsert(ctx context.Context, query *BunInsertQuery) error
}

// AfterInsertHook is called after an INSERT query is executed.
// Implement this on model structs to trigger side effects after successful insertion.
type AfterInsertHook interface {
	bun.AfterInsertHook
	// AfterInsert is invoked with the query context and the insert query that was executed.
	AfterInsert(ctx context.Context, query *BunInsertQuery) error
}

// BeforeUpdateHook is called before an UPDATE query is executed.
// Implement this on model structs to validate changes or modify fields before the update.
type BeforeUpdateHook interface {
	bun.BeforeUpdateHook
	// BeforeUpdate is invoked with the query context and the update query about to be executed.
	BeforeUpdate(ctx context.Context, query *BunUpdateQuery) error
}

// AfterUpdateHook is called after an UPDATE query is executed.
// Implement this on model structs to trigger side effects after successful update.
type AfterUpdateHook interface {
	bun.AfterUpdateHook
	// AfterUpdate is invoked with the query context and the update query that was executed.
	AfterUpdate(ctx context.Context, query *BunUpdateQuery) error
}

// BeforeDeleteHook is called before a DELETE query is executed.
// Implement this on model structs to validate or perform cleanup before deletion.
type BeforeDeleteHook interface {
	bun.BeforeDeleteHook
	// BeforeDelete is invoked with the query context and the delete query about to be executed.
	BeforeDelete(ctx context.Context, query *BunDeleteQuery) error
}

// AfterDeleteHook is called after a DELETE query is executed.
// Implement this on model structs to perform cleanup or trigger side effects after deletion.
type AfterDeleteHook interface {
	bun.AfterDeleteHook
	// AfterDelete is invoked with the query context and the delete query that was executed.
	AfterDelete(ctx context.Context, query *BunDeleteQuery) error
}
