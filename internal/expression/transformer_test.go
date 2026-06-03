package expression

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/expression"
	moldimpl "github.com/coldsmirk/vef-framework-go/internal/mold"
	"github.com/coldsmirk/vef-framework-go/mold"
)

// CapturingEngine records the inputs it receives and returns a preset value.
type CapturingEngine struct {
	value     expression.Value
	gotSource string
	gotEnv    any
}

func (e *CapturingEngine) Evaluate(_ context.Context, source string, env any) (expression.Value, error) {
	e.gotSource = source
	e.gotEnv = env

	return e.value, nil
}

func (*CapturingEngine) Compile(string, ...expression.CompileOption) (expression.Program, error) {
	return nil, nil
}

// FakeFieldLevel is a minimal mold.FieldLevel for unit-testing the transformer
// without a full struct walk.
type FakeFieldLevel struct {
	param string
	field reflect.Value
	str   reflect.Value
}

func (FakeFieldLevel) Transformer() mold.Transformer             { return nil }
func (FakeFieldLevel) Name() string                              { return "" }
func (FakeFieldLevel) Parent() reflect.Value                     { return reflect.Value{} }
func (f FakeFieldLevel) Field() reflect.Value                    { return f.field }
func (f FakeFieldLevel) Param() string                           { return f.param }
func (FakeFieldLevel) SiblingField(string) (reflect.Value, bool) { return reflect.Value{}, false }
func (f FakeFieldLevel) Struct() reflect.Value                   { return f.str }

func settableFloat() reflect.Value {
	return reflect.New(reflect.TypeFor[float64]()).Elem()
}

// OrderingEngine records the order in which expressions are evaluated.
type OrderingEngine struct {
	sources []string
}

func (e *OrderingEngine) Evaluate(_ context.Context, source string, _ any) (expression.Value, error) {
	e.sources = append(e.sources, source)

	return expression.NewValue(float64(0)), nil
}

func (*OrderingEngine) Compile(string, ...expression.CompileOption) (expression.Program, error) {
	return nil, nil
}

func TestFieldTransformer(t *testing.T) {
	t.Run("Tag", func(t *testing.T) {
		assert.Equal(t, "expr", NewFieldTransformer(new(CapturingEngine)).Tag(), "Tag should be expr")
	})

	t.Run("Success", func(t *testing.T) {
		type row struct {
			Price float64 `json:"price"`
			Qty   float64 `json:"qty"`
		}

		eng := &CapturingEngine{value: expression.NewValue(float64(6))}
		field := settableFloat()
		env := row{Price: 2, Qty: 3}

		err := NewFieldTransformer(eng).Transform(context.Background(), FakeFieldLevel{
			param: "price * qty",
			field: field,
			str:   reflect.ValueOf(env),
		})
		require.NoError(t, err, "Transform should succeed")
		assert.Equal(t, float64(6), field.Float(), "Field should be set to the engine result")
		assert.Equal(t, "price * qty", eng.gotSource, "Source should be the tag param")
		assert.Equal(t, env, eng.gotEnv, "Env should be the containing struct")
	})

	t.Run("NilEnvWithoutStruct", func(t *testing.T) {
		eng := &CapturingEngine{value: expression.NewValue(float64(2))}

		err := NewFieldTransformer(eng).Transform(context.Background(), FakeFieldLevel{
			param: "1 + 1",
			field: settableFloat(),
		})
		require.NoError(t, err, "Transform should succeed without a struct env")
		assert.Nil(t, eng.gotEnv, "Env should be nil when no struct is available")
	})

	t.Run("EmptyExpression", func(t *testing.T) {
		err := NewFieldTransformer(new(CapturingEngine)).Transform(context.Background(), FakeFieldLevel{
			param: "",
			field: settableFloat(),
		})
		assert.ErrorIs(t, err, ErrEmptyExpression, "Empty expression should error")
	})

	t.Run("NotSettable", func(t *testing.T) {
		err := NewFieldTransformer(new(CapturingEngine)).Transform(context.Background(), FakeFieldLevel{
			param: "x",
			field: reflect.ValueOf(3.0), // not addressable
		})
		assert.ErrorIs(t, err, ErrFieldNotSettable, "Non-settable field should error")
	})

	t.Run("DecodeError", func(t *testing.T) {
		eng := &CapturingEngine{value: expression.NewValue("not-a-number")}
		field := reflect.New(reflect.TypeFor[int]()).Elem()

		err := NewFieldTransformer(eng).Transform(context.Background(), FakeFieldLevel{param: "x", field: field})
		require.Error(t, err, "Decoding a string into int should fail")
	})
}

// TestFieldTransformerWithMold drives the transformer through a real mold struct
// walk, verifying the containing struct is exposed as the evaluation env.
func TestFieldTransformerWithMold(t *testing.T) {
	type Row struct {
		Price float64 `json:"price"`
		Qty   float64 `json:"qty"`
		Total float64 `json:"total" mold:"expr=price * qty"`
	}

	eng := &CapturingEngine{value: expression.NewValue(float64(6))}

	transformer := moldimpl.New()
	field := NewFieldTransformer(eng)
	transformer.Register(field.Tag(), field.Transform)

	row := Row{Price: 2, Qty: 3}
	err := transformer.Struct(context.Background(), &row)
	require.NoError(t, err, "Mold struct walk should succeed")

	assert.Equal(t, float64(6), row.Total, "Total should be computed by the expression")
	assert.Equal(t, "price * qty", eng.gotSource, "Engine should receive the tag expression")

	got, ok := eng.gotEnv.(Row)
	require.True(t, ok, "Env should be the containing struct")
	assert.Equal(t, float64(2), got.Price, "Env should carry sibling values")
}

// TestFieldTransformerEvaluationOrder guards against non-deterministic field
// ordering: expr fields must evaluate in declaration order so a derived field
// can reliably reference siblings declared above it. Repeated to defeat Go's
// randomized map iteration.
func TestFieldTransformerEvaluationOrder(t *testing.T) {
	type Row struct {
		A float64 `json:"a" mold:"expr=1"`
		B float64 `json:"b" mold:"expr=2"`
		C float64 `json:"c" mold:"expr=3"`
		D float64 `json:"d" mold:"expr=4"`
	}

	for range 25 {
		eng := new(OrderingEngine)
		transformer := moldimpl.New()
		field := NewFieldTransformer(eng)
		transformer.Register(field.Tag(), field.Transform)

		row := Row{}
		require.NoError(t, transformer.Struct(context.Background(), &row), "Mold struct walk should succeed")
		assert.Equal(t, []string{"1", "2", "3", "4"}, eng.sources, "Expr fields must evaluate in declaration order")
	}
}
