package expression

import "errors"

var (
	// ErrEmptyExpression is returned when an expr field tag carries no expression.
	ErrEmptyExpression = errors.New("expression: empty expression in field tag")
	// ErrFieldNotSettable is returned when the target field cannot be assigned.
	ErrFieldNotSettable = errors.New("expression: target field is not settable")
)
