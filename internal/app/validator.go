package app

import "github.com/coldsmirk/vef-framework-go/validator"

// StructValidator implements Fiber's struct validator interface.
type StructValidator struct{}

// Validate delegates to the framework's validator.
func (*StructValidator) Validate(out any) error {
	return validator.Validate(out)
}

func newStructValidator() *StructValidator {
	return &StructValidator{}
}
