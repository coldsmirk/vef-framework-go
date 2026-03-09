package orm

// ForeignKeyBuilder provides a fluent API for defining table-level foreign key constraints.
type ForeignKeyBuilder interface {
	// Name sets an explicit constraint name.
	// SQL: CONSTRAINT "name" FOREIGN KEY (...)
	Name(name string) ForeignKeyBuilder
	// Columns sets the local columns that form the foreign key.
	Columns(columns ...string) ForeignKeyBuilder
	// References sets the referenced table and columns.
	References(table string, columns ...string) ForeignKeyBuilder
	// OnDelete sets the referential action for DELETE operations.
	OnDelete(action ReferenceAction) ForeignKeyBuilder
	// OnUpdate sets the referential action for UPDATE operations.
	OnUpdate(action ReferenceAction) ForeignKeyBuilder
}

// ForeignKeyDef holds the definition of a foreign key constraint.
type ForeignKeyDef struct {
	name       string
	columns    []string
	refTable   string
	refColumns []string
	onDelete   *ReferenceAction
	onUpdate   *ReferenceAction
}

func (f *ForeignKeyDef) Name(name string) ForeignKeyBuilder {
	f.name = name

	return f
}

func (f *ForeignKeyDef) Columns(columns ...string) ForeignKeyBuilder {
	f.columns = columns

	return f
}

func (f *ForeignKeyDef) References(table string, columns ...string) ForeignKeyBuilder {
	f.refTable = table
	f.refColumns = columns

	return f
}

func (f *ForeignKeyDef) OnDelete(action ReferenceAction) ForeignKeyBuilder {
	f.onDelete = &action

	return f
}

func (f *ForeignKeyDef) OnUpdate(action ReferenceAction) ForeignKeyBuilder {
	f.onUpdate = &action

	return f
}
