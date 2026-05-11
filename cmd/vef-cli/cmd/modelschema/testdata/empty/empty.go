// Package empty is a test fixture containing no types that embed orm.BaseModel.
// It is used to verify that GenerateFile silently no-ops (no error, no output)
// when invoked on a file that has nothing to generate.
package empty

// NotAModel does not embed orm.BaseModel and must be ignored by the generator.
type NotAModel struct {
	Name string
}
