package orm

// PrimaryKeyBuilder provides a fluent API for defining table-level PRIMARY KEY constraints.
type PrimaryKeyBuilder interface {
	// Name sets an explicit constraint name.
	// SQL: CONSTRAINT "name" PRIMARY KEY (...)
	Name(name string) PrimaryKeyBuilder
	// Columns sets the columns that form the primary key.
	Columns(columns ...string) PrimaryKeyBuilder
}

// PrimaryKeyDef holds the definition of a primary key constraint.
type PrimaryKeyDef struct {
	name    string
	columns []string
}

func (p *PrimaryKeyDef) Name(name string) PrimaryKeyBuilder {
	p.name = name

	return p
}

func (p *PrimaryKeyDef) Columns(columns ...string) PrimaryKeyBuilder {
	p.columns = columns

	return p
}
