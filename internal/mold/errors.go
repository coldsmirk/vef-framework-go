package mold

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrInvalidDive describes an invalid dive tag configuration.
	ErrInvalidDive = errors.New("invalid dive tag configuration")

	// ErrUndefinedKeysTag describes an undefined keys tag when endkeys tag is defined.
	ErrUndefinedKeysTag = errors.New("'" + endKeysTag + "' tag encountered without a corresponding '" + keysTag + "' tag")

	// ErrInvalidKeysTag describes a misuse of the keys tag.
	ErrInvalidKeysTag = errors.New("'" + keysTag + "' tag must be immediately preceded by the '" + diveTag + "' tag")
)

// ErrUndefinedTag defines a tag that does not exist.
type ErrUndefinedTag struct {
	tag   string
	field string
}

func (e *ErrUndefinedTag) Error() string {
	if e.field == "" {
		return fmt.Sprintf("unregistered/undefined transformation %q found on field", e.tag)
	}

	return fmt.Sprintf("unregistered/undefined transformation %q found on field %s", e.tag, e.field)
}

// ErrInvalidTag defines a bad value for a tag being used.
type ErrInvalidTag struct {
	tag   string
	field string
}

func (e *ErrInvalidTag) Error() string {
	return fmt.Sprintf("invalid tag %q found on field %s", e.tag, e.field)
}

// ErrInvalidTransformValue describes an invalid argument passed to Struct or Field.
type ErrInvalidTransformValue struct {
	typ reflect.Type
	fn  string
}

func (e *ErrInvalidTransformValue) Error() string {
	if e.typ == nil {
		return fmt.Sprintf("mold: %s(nil)", e.fn)
	}

	if e.typ.Kind() != reflect.Pointer {
		return fmt.Sprintf("mold: %s(non-pointer %s)", e.fn, e.typ.String())
	}

	return fmt.Sprintf("mold: %s(nil %s)", e.fn, e.typ.String())
}

// ErrInvalidTransformation describes an invalid argument passed to Struct or Field.
type ErrInvalidTransformation struct {
	typ reflect.Type
}

func (e *ErrInvalidTransformation) Error() string {
	return "mold: (nil " + e.typ.String() + ")"
}
