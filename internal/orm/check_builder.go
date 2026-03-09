package orm

// CheckBuilder provides a fluent API for defining table-level CHECK constraints.
type CheckBuilder interface {
	// Name sets an explicit constraint name.
	// SQL: CONSTRAINT "name" CHECK (...)
	Name(name string) CheckBuilder
	// Condition sets the check condition using the ConditionBuilder.
	Condition(builder func(ConditionBuilder)) CheckBuilder
}

// CheckDef holds the definition of a table-level check constraint.
type CheckDef struct {
	name             string
	conditionBuilder func(ConditionBuilder)
}

func (c *CheckDef) Name(name string) CheckBuilder {
	c.name = name

	return c
}

func (c *CheckDef) Condition(builder func(ConditionBuilder)) CheckBuilder {
	c.conditionBuilder = builder

	return c
}
