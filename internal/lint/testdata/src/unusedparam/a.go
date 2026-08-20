package unusedparam

func AllNamed(a int, b string) error { return nil } // want `every parameter is unused`

func AllBlank(_ int, _ string) error { return nil } // want `every parameter is unused`

// GroupedBlank declares three parameters, not two: dropping the names without
// repeating the grouped type would change its arity.
func GroupedBlank(_, _ string, _ int) error { return nil } // want `every parameter is unused`

func GroupedNamed(a, b int, c string) error { return nil } // want `every parameter is unused`

func Variadic(values ...int) error { return nil } // want `every parameter is unused`

func Functional(fn func(int) error, count int) error { return nil } // want `every parameter is unused`

func AlreadyOmitted(int, string) error { return nil }

func NoParams() error { return nil }

func PartlyUnused(a int, b string) error { // want `parameter "b" is unused`
	_ = a

	return nil
}

func PartlyGrouped(a, b int) error { // want `parameter "b" is unused`
	_ = a

	return nil
}

func PartlyBlank(a int, _ string) error {
	_ = a

	return nil
}

func AllUsed(a int, b string) error {
	_, _ = a, b

	return nil
}

// Literal keeps its parameter because the closure reads it, and the closure's
// own unused parameter is out of scope for this rule.
func Literal(a int) func(int) int {
	return func(unused int) int { return a }
}

// AnnotatedGroup annotates one name of a grouped parameter. The names and the
// type they share are rewritten as a single span, so the comment between them
// would be deleted with them — the diagnostic therefore stands without a fix,
// and the golden file keeps this signature unchanged.
func AnnotatedGroup( // want `every parameter is unused; omit the names and keep only the types \(no fix offered: a comment sits inside the parameter list\)`
	a, // the count
	b int,
) error {
	return nil
}
