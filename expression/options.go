package expression

// CompileOptions holds the resolved compilation settings an Engine applies
// when preparing a Program. Backend adapters read it to choose how to evaluate.
type CompileOptions struct {
	// Predicate marks the expression as boolean-valued; the backend evaluates
	// it in its boolean idiom.
	Predicate bool
}

// CompileOption customizes compilation.
type CompileOption func(*CompileOptions)

// AsPredicate marks the expression as boolean-valued.
func AsPredicate() CompileOption {
	return func(o *CompileOptions) {
		o.Predicate = true
	}
}
