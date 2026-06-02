package expression

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/expression"
	"github.com/coldsmirk/vef-framework-go/mold"
)

// fieldTransformerTag is the mold struct tag handled by this transformer.
const fieldTransformerTag = "expr"

// fieldTransformer computes a struct field from an expression evaluated against
// the containing struct, enabling derived fields such as:
//
//	Total float64 `json:"total" mold:"expr=price * quantity"`
//
// The expression follows mold's tag grammar: it is introduced with "=" and any
// comma it contains must be escaped as 0x2C, since mold splits tags on commas.
//
// Fields are evaluated in declaration order, so an expression may reference
// sibling fields declared above it (including earlier derived fields); a
// reference to a field declared below reads that field's zero value.
type fieldTransformer struct {
	engine expression.Engine
}

// NewFieldTransformer creates a mold field transformer backed by engine.
func NewFieldTransformer(engine expression.Engine) mold.FieldTransformer {
	return &fieldTransformer{engine: engine}
}

func (*fieldTransformer) Tag() string {
	return fieldTransformerTag
}

func (t *fieldTransformer) Transform(ctx context.Context, fl mold.FieldLevel) error {
	source := fl.Param()
	if source == "" {
		return ErrEmptyExpression
	}

	field := fl.Field()
	if !field.CanSet() {
		return ErrFieldNotSettable
	}

	// The containing struct is the evaluation environment so the expression can
	// reference sibling fields by their serialized names.
	var env any
	if s := fl.Struct(); s.IsValid() {
		env = s.Interface()
	}

	value, err := t.engine.Evaluate(ctx, source, env)
	if err != nil {
		return err
	}

	return value.Decode(field.Addr().Interface())
}
