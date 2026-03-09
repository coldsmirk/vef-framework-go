package mold

import (
	"reflect"

	"github.com/coldsmirk/vef-framework-go/mold"
)

type MoldStructLevel struct {
	transformer *MoldTransformer
	parent      reflect.Value
	current     reflect.Value
}

func (s MoldStructLevel) Transformer() mold.Transformer {
	return s.transformer
}

func (s MoldStructLevel) Parent() reflect.Value {
	return s.parent
}

func (s MoldStructLevel) Struct() reflect.Value {
	return s.current
}
