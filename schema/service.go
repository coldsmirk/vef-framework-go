package schema

import "context"

// Table represents basic table information for listing.
type Table struct {
	Name    string `json:"name"`
	Schema  string `json:"schema,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// TableSchema represents detailed table structure information.
type TableSchema struct {
	Name        string       `json:"name"`
	Schema      string       `json:"schema,omitempty"`
	Comment     string       `json:"comment,omitempty"`
	Columns     []Column     `json:"columns"`
	PrimaryKey  *PrimaryKey  `json:"primaryKey,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	UniqueKeys  []UniqueKey  `json:"uniqueKeys,omitempty"`
	ForeignKeys []ForeignKey `json:"foreignKeys,omitempty"`
	Checks      []Check      `json:"checks,omitempty"`
}

// Column represents database column metadata.
type Column struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Nullable        bool   `json:"nullable"`
	Default         string `json:"default,omitempty"`
	Comment         string `json:"comment,omitempty"`
	IsPrimaryKey    bool   `json:"isPrimaryKey,omitempty"`
	IsAutoIncrement bool   `json:"isAutoIncrement,omitempty"`
}

// PrimaryKey represents the primary key constraint.
type PrimaryKey struct {
	Name    string   `json:"name,omitempty"`
	Columns []string `json:"columns"`
}

// Index represents a non-unique database index.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// UniqueKey represents a unique constraint or unique index.
type UniqueKey struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnUpdate   string   `json:"onUpdate,omitempty"`
	OnDelete   string   `json:"onDelete,omitempty"`
}

// Check represents a CHECK constraint.
type Check struct {
	Name string `json:"name"`
	Expr string `json:"expr"`
}

// View represents database view information.
type View struct {
	Name         string   `json:"name"`
	Schema       string   `json:"schema,omitempty"`
	Definition   string   `json:"definition"`
	Comment      string   `json:"comment,omitempty"`
	Columns      []string `json:"columns,omitempty"`
	Materialized bool     `json:"materialized,omitempty"`
}

// Trigger represents database trigger information.
type Trigger struct {
	Name       string   `json:"name"`
	Table      string   `json:"table,omitempty"`
	View       string   `json:"view,omitempty"`
	ActionTime string   `json:"actionTime"`
	Events     []string `json:"events"`
	ForEachRow bool     `json:"forEachRow"`
	Body       string   `json:"body"`
}

// Service defines the interface for read-only database schema inspection.
type Service interface {
	// ListTables returns all tables in the current database/schema.
	ListTables(ctx context.Context) ([]Table, error)
	// GetTableSchema returns detailed structure information about a specific table.
	GetTableSchema(ctx context.Context, name string) (*TableSchema, error)
	// ListViews returns all views in the current database/schema.
	ListViews(ctx context.Context) ([]View, error)
	// ListTriggers returns all triggers in the current database/schema.
	ListTriggers(ctx context.Context) ([]Trigger, error)
}
