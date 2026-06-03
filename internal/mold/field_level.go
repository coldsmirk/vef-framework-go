package mold

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/mold"
)

type MoldFieldLevel struct {
	transformer *MoldTransformer
	name        string
	parent      reflect.Value
	current     reflect.Value
	param       string
	// container and sc are intentionally the zero value when the field is transformed
	// outside a struct walk (Transformer.Field, or dive/map/slice elements). Struct() and
	// SiblingField() report invalid/false in that case; the translate and expression
	// transformers depend on this. Do not populate them on the non-struct path.
	container reflect.Value
	sc        *cStruct
}

func (f *MoldFieldLevel) Transformer() mold.Transformer {
	return f.transformer
}

func (f *MoldFieldLevel) Name() string {
	return f.name
}

func (f *MoldFieldLevel) Parent() reflect.Value {
	return f.parent
}

func (f *MoldFieldLevel) Field() reflect.Value {
	return f.current
}

func (f *MoldFieldLevel) Param() string {
	return f.param
}

func (f *MoldFieldLevel) SiblingField(name string) (reflect.Value, bool) {
	// Check if we have valid struct value and cache
	if !f.container.IsValid() || f.sc == nil {
		return reflect.Value{}, false
	}

	// Find the field in the struct cache
	cf, ok := f.sc.fields[name]
	if !ok {
		return reflect.Value{}, false
	}

	return f.container.Field(cf.idx), true
}

func (f *MoldFieldLevel) Struct() reflect.Value {
	return f.container
}
