package mold

import (
	"github.com/coldsmirk/vef-framework-go/mold"
)

// NewTransformer creates a new transformer instance, integrating all registered transformers and interceptors.
func NewTransformer(fieldTransformers []mold.FieldTransformer, structTransformers []mold.StructTransformer, interceptors []mold.Interceptor) mold.Transformer {
	transformer := New()

	for _, ft := range fieldTransformers {
		transformer.Register(ft.Tag(), ft.Transform)
	}

	for _, st := range structTransformers {
		transformer.RegisterStructLevel(st.Transform)
	}

	for _, interceptor := range interceptors {
		transformer.RegisterInterceptor(interceptor.Intercept)
	}

	return transformer
}
