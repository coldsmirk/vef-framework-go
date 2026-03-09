package orm

// UniqueBuilder provides a fluent API for defining table-level UNIQUE constraints.
type UniqueBuilder interface {
	// Name sets an explicit constraint name.
	// SQL: CONSTRAINT "name" UNIQUE (...)
	Name(name string) UniqueBuilder
	// Columns sets the columns that form the unique constraint.
	Columns(columns ...string) UniqueBuilder
}

// UniqueDef holds the definition of a unique constraint.
type UniqueDef struct {
	name    string
	columns []string
}

func (u *UniqueDef) Name(name string) UniqueBuilder {
	u.name = name

	return u
}

func (u *UniqueDef) Columns(columns ...string) UniqueBuilder {
	u.columns = columns

	return u
}
