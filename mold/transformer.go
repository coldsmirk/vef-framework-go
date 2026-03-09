package mold

import (
	"context"
	"reflect"
)

// Transformer defines the main interface for struct transformers that provide tag-based data transformation.
type Transformer interface {
	// Struct applies transformations to the entire struct based on field tags
	Struct(ctx context.Context, value any) error
	// Field applies specified transformation tags to a single field
	Field(ctx context.Context, value any, tags string) error
}

// FieldTransformer defines the field-level transformer interface for extending custom field transformation logic.
type FieldTransformer interface {
	// Tag returns the tag name corresponding to this transformer, used for referencing in struct tags
	Tag() string
	// Transform executes field transformation logic, fl provides field-level context information
	Transform(ctx context.Context, fl FieldLevel) error
}

// StructTransformer defines the struct-level transformer interface for custom processing of entire structs.
type StructTransformer interface {
	// Transform executes struct-level transformation logic, sl provides struct-level context information
	Transform(ctx context.Context, sl StructLevel) error
}

// Interceptor defines the interceptor interface for redirecting transformation operations of certain types to inner values
// For example: sql.NullString transformations should operate on its inner string value.
type Interceptor interface {
	// Intercept intercepts the current value and returns the inner value that should be actually operated on
	Intercept(current reflect.Value) (inner reflect.Value)
}

// FieldLevel represents the interface for field level modifier function.
type FieldLevel interface {
	// Transformer represents a subset of the current *Transformer that is executing the current transformation.
	Transformer() Transformer
	// Name returns the name of the current field being modified.
	Name() string
	//
	// Parent returns the top level parent of the current value return by Field()
	//
	// This is used primarily for having the ability to nil out pointer type values.
	//
	// NOTE: that is there are several layers of abstractions eg. interface{} of interface{} of interface{} this
	//       function returns the first interface{}
	//
	Parent() reflect.Value
	// Field returns the current field value being modified.
	Field() reflect.Value
	// Param returns the param associated wth the given function modifier.
	Param() string
	// SiblingField returns the sibling field value of the same struct by field name
	// Returns the field reflect.Value and a boolean indicating if the field exists
	SiblingField(name string) (reflect.Value, bool)
}

// StructLevel represents the interface for struct level modifier function.
type StructLevel interface {
	// Transformer represents a subset of the current *Transformer that is executing the current transformation.
	Transformer() Transformer
	//
	// Parent returns the top level parent of the current value return by Struct().
	//
	// This is used primarily for having the ability to nil out pointer type values.
	//
	// NOTE: that is there are several layers of abstractions eg. interface{} of interface{} of interface{} this
	//       function returns the first interface{}.
	//
	Parent() reflect.Value
	// Struct returns the value of the current struct being modified.
	Struct() reflect.Value
}

// Func defines a transform function for use.
type Func func(ctx context.Context, fl FieldLevel) error

// StructLevelFunc accepts all values needed for struct level manipulation.
//
// Why does this exist? For structs for which you may not have access or rights to add tags too,
// from other packages your using.
type StructLevelFunc func(ctx context.Context, sl StructLevel) error

// InterceptorFunc is a way to intercept custom types to redirect the functions to be applied to an inner typ/value.
// Eg. Sql.NullString, the manipulation should be done on the inner string.
type InterceptorFunc func(current reflect.Value) (inner reflect.Value)
