package mold

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/mold"
)

// TestBadValues tests error handling for invalid inputs and registration panics.
func TestBadValues(t *testing.T) {
	transformer := New()
	transformer.Register("blah", func(_ context.Context, _ mold.FieldLevel) error { return nil })

	type InvalidTagStruct struct {
		Ignore string `mold:"-"`
		String string `mold:"blah,,blah"`
	}

	t.Run("InvalidTagFormat", func(t *testing.T) {
		var tt InvalidTagStruct

		err := transformer.Struct(context.Background(), &tt)
		assert.Error(t, err, "Should return error for invalid tag format")
		assert.Equal(t, "invalid tag \"\" found on field String", err.Error(), "Error message should mention invalid tag")
	})

	t.Run("NonPointerStruct", func(t *testing.T) {
		var tt InvalidTagStruct

		err := transformer.Struct(context.Background(), tt)
		assert.Error(t, err, "Should return error for non-pointer struct")
		assert.Equal(t, "mold: Struct(non-pointer mold.InvalidTagStruct)", err.Error(), "Error message should mention non-pointer")
	})

	t.Run("NilStruct", func(t *testing.T) {
		err := transformer.Struct(context.Background(), nil)
		assert.Error(t, err, "Should return error for nil struct")
		assert.Equal(t, "mold: Struct(nil)", err.Error(), "Error message should mention nil")
	})

	t.Run("InvalidTypeInt", func(t *testing.T) {
		var i int

		err := transformer.Struct(context.Background(), &i)
		assert.Error(t, err, "Should return error for invalid type")
		assert.Equal(t, "mold: (nil *int)", err.Error(), "Error message should mention invalid type")
	})

	t.Run("InterfaceNil", func(t *testing.T) {
		var iface any

		err := transformer.Struct(context.Background(), iface)
		assert.Error(t, err, "Should return error for nil interface")
		assert.Equal(t, "mold: Struct(nil)", err.Error(), "Error message should mention nil")
	})

	t.Run("InterfacePointerToNil", func(t *testing.T) {
		var iface any

		err := transformer.Struct(context.Background(), &iface)
		assert.Error(t, err, "Should return error for pointer to nil interface")
		assert.Equal(t, "mold: (nil *interface {})", err.Error(), "Error message should mention nil interface")
	})

	t.Run("NilPointerToStruct", func(t *testing.T) {
		var tst *InvalidTagStruct

		err := transformer.Struct(context.Background(), tst)
		assert.Error(t, err, "Should return error for nil pointer to struct")
		assert.Equal(t, "mold: Struct(nil *mold.InvalidTagStruct)", err.Error(), "Error message should mention nil pointer")
	})

	t.Run("NilPointerField", func(t *testing.T) {
		var tm *time.Time

		err := transformer.Field(context.Background(), tm, "blah")
		assert.Error(t, err, "Should return error for nil pointer field")
		assert.Equal(t, "mold: Field(nil *time.Time)", err.Error(), "Error message should mention nil pointer")
	})

	t.Run("RegistrationPanics", func(t *testing.T) {
		assert.PanicsWithValue(t, "mold: transformation tag cannot be empty", func() {
			transformer.Register("", nil)
		}, "Should panic when registering empty tag")
		assert.PanicsWithValue(t, "mold: transformation function cannot be nil", func() {
			transformer.Register("test", nil)
		}, "Should panic when registering nil function")
		assert.PanicsWithValue(t, "mold: tag \",\" either contains restricted characters or is the same as a restricted tag needed for normal operation", func() {
			transformer.Register(",", func(_ context.Context, _ mold.FieldLevel) error { return nil })
		}, "Should panic when registering restricted character")
	})

	t.Run("AliasRegistrationPanics", func(t *testing.T) {
		assert.PanicsWithValue(t, "mold: transformation alias cannot be empty", func() {
			transformer.RegisterAlias("", "")
		}, "Should panic when registering empty alias")
		assert.PanicsWithValue(t, "mold: aliased tags cannot be empty", func() {
			transformer.RegisterAlias("test", "")
		}, "Should panic when aliased tags are empty")
		assert.PanicsWithValue(t, "mold: alias \",\" either contains restricted characters or is the same as a restricted tag needed for normal operation", func() {
			transformer.RegisterAlias(",", "test")
		}, "Should panic when alias contains restricted character")
	})
}

// TestBasicTransform tests basic transformation functionality including nested and embedded structs.
func TestBasicTransform(t *testing.T) {
	type BasicStruct struct {
		String string `mold:"repl"`
	}

	transformer := New()
	transformer.Register("repl", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().SetString("test")

		return nil
	})

	t.Run("BasicStructTransformation", func(t *testing.T) {
		var tt BasicStruct

		val := reflect.ValueOf(tt)
		for range 3 {
			_, err := transformer.extractStructCache(val)
			require.NoError(t, err, "Extract struct cache should succeed")
		}

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Basic struct transformation should succeed")
		assert.Equal(t, "test", tt.String, "String field should be transformed")
	})

	t.Run("NestedStructTransformation", func(t *testing.T) {
		type NestedStruct struct {
			Basic  BasicStruct
			String string `mold:"repl"`
		}

		var tt2 NestedStruct

		err := transformer.Struct(context.Background(), &tt2)
		require.NoError(t, err, "Nested struct transformation should succeed")
		assert.Equal(t, "test", tt2.Basic.String, "Nested struct field should be transformed")
		assert.Equal(t, "test", tt2.String, "String field should be transformed")
	})

	t.Run("EmbeddedStructTransformation", func(t *testing.T) {
		type EmbeddedStruct struct {
			BasicStruct

			String string `mold:"repl"`
		}

		var tt3 EmbeddedStruct

		err := transformer.Struct(context.Background(), &tt3)
		require.NoError(t, err, "Embedded struct transformation should succeed")
		assert.Equal(t, "test", tt3.BasicStruct.String, "Embedded struct field should be transformed")
		assert.Equal(t, "test", tt3.String, "String field should be transformed")
	})

	t.Run("NilPointerField", func(t *testing.T) {
		type NilPointerStruct struct {
			Basic  *BasicStruct
			String string `mold:"repl"`
		}

		var tt4 NilPointerStruct

		err := transformer.Struct(context.Background(), &tt4)
		require.NoError(t, err, "Transformation with nil pointer field should succeed")
		assert.Nil(t, tt4.Basic, "Nil pointer field should remain nil")
		assert.Equal(t, "test", tt4.String, "String field should be transformed")
	})

	t.Run("NonNilPointerField", func(t *testing.T) {
		type NonNilPointerStruct struct {
			Basic  *BasicStruct
			String string `mold:"repl"`
		}

		tt5 := NonNilPointerStruct{Basic: &BasicStruct{}}
		err := transformer.Struct(context.Background(), &tt5)
		require.NoError(t, err, "Transformation with non-nil pointer field should succeed")
		assert.Equal(t, "test", tt5.Basic.String, "Pointer field should be transformed")
		assert.Equal(t, "test", tt5.String, "String field should be transformed")
	})

	t.Run("DefaultTagForPointer", func(t *testing.T) {
		type DefaultPointerStruct struct {
			Basic  *BasicStruct `mold:"default"`
			String string       `mold:"repl"`
		}

		var tt6 DefaultPointerStruct

		transformer.Register("default", func(_ context.Context, fl mold.FieldLevel) error {
			fl.Field().Set(reflect.New(fl.Field().Type().Elem()))

			return nil
		})
		err := transformer.Struct(context.Background(), &tt6)
		require.NoError(t, err, "Default tag transformation should succeed")
		assert.NotNil(t, tt6.Basic, "Pointer field should be initialized")
		assert.Equal(t, "test", tt6.Basic.String, "Initialized pointer field should be transformed")
		assert.Equal(t, "test", tt6.String, "String field should be transformed")
	})

	t.Run("FieldTransformation", func(t *testing.T) {
		type FieldTransformStruct struct {
			Basic  *BasicStruct `mold:"default"`
			String string       `mold:"repl"`
		}

		tt6 := FieldTransformStruct{}
		tt6.String = "BAD"

		var tString string

		go func() {
			err := transformer.Field(context.Background(), &tString, "repl")
			assert.NoError(t, err, "Concurrent field transformation should succeed")
		}()

		err := transformer.Field(context.Background(), &tt6.String, "repl")
		require.NoError(t, err, "Field transformation should succeed")
		assert.Equal(t, "test", tt6.String, "Field should be transformed")
	})

	t.Run("EmptyAndSkipTags", func(t *testing.T) {
		type EmptyTagStruct struct {
			String string `mold:"repl"`
		}

		tt6 := EmptyTagStruct{String: "BAD"}
		err := transformer.Field(context.Background(), &tt6.String, "")
		require.NoError(t, err, "Empty tag should be handled")

		err = transformer.Field(context.Background(), &tt6.String, "-")
		require.NoError(t, err, "Skip tag should be handled")
	})

	t.Run("FieldErrors", func(t *testing.T) {
		type FieldErrorStruct struct {
			String string `mold:"repl"`
		}

		tt6 := FieldErrorStruct{}
		err := transformer.Field(context.Background(), tt6.String, "test")
		assert.Error(t, err, "Should return error for non-pointer field")
		assert.Equal(t, "mold: Field(non-pointer string)", err.Error(), "Error message should mention non-pointer")

		err = transformer.Field(context.Background(), nil, "test")
		assert.Error(t, err, "Should return error for nil field")
		assert.Equal(t, "mold: Field(nil)", err.Error(), "Error message should mention nil")

		var iface any

		err = transformer.Field(context.Background(), iface, "test")
		assert.Error(t, err, "Should return error for nil interface")
		assert.Equal(t, "mold: Field(nil)", err.Error(), "Error message should mention nil")
	})

	t.Run("NonexistentTransformation", func(t *testing.T) {
		type NonexistentTransformStruct struct {
			String string `mold:"repl"`
		}

		tt6 := NonexistentTransformStruct{}

		var tString string

		done := make(chan struct{})

		go func() {
			err := transformer.Field(context.Background(), &tString, "nonexistant")
			assert.Error(t, err, "Concurrent nonexistent transformation should return error")
			close(done)
		}()

		err := transformer.Field(context.Background(), &tt6.String, "nonexistant")
		assert.Error(t, err, "Should return error for nonexistent transformation")
		assert.Equal(t, "unregistered/undefined transformation \"nonexistant\" found on field", err.Error(), "Error message should mention undefined transformation")

		<-done
		transformer.Register("dummy", func(_ context.Context, _ mold.FieldLevel) error { return nil })
		err = transformer.Field(context.Background(), &tt6.String, "dummy")
		assert.NoError(t, err, "Newly registered transformation should work")
	})
}

// TestAlias tests transformation alias registration and usage.
func TestAlias(t *testing.T) {
	transformer := New()
	transformer.Register("repl", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().SetString("test")

		return nil
	})
	transformer.Register("repl2", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().SetString("test2")

		return nil
	})

	t.Run("MultipleTransformations", func(t *testing.T) {
		type MultipleTransformStruct struct {
			String string `mold:"repl,repl2"`
		}

		var tt MultipleTransformStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Multiple transformations should succeed")
		assert.Equal(t, "test2", tt.String, "Last transformation should take effect")
	})

	t.Run("AliasRegistrationAndUsage", func(t *testing.T) {
		transformer.RegisterAlias("rep", "repl,repl2")
		transformer.RegisterAlias("bad", "repl,,repl2")

		type AliasStruct struct {
			String string `mold:"rep"`
		}

		var tt2 AliasStruct

		err := transformer.Struct(context.Background(), &tt2)
		require.NoError(t, err, "Alias transformation should succeed")
		assert.Equal(t, "test2", tt2.String, "Alias should expand to multiple transformations")
	})

	t.Run("InvalidAliasWithEmptyTag", func(t *testing.T) {
		var s string

		err := transformer.Field(context.Background(), &s, "bad")
		assert.Error(t, err, "Should return error for invalid alias with empty tag")
	})

	t.Run("CombinedAliasUsage", func(t *testing.T) {
		var s string

		err := transformer.Field(context.Background(), &s, "repl,rep,bad")
		assert.Error(t, err, "Should return error for combined alias with invalid tag")
	})
}

// TestArray tests array transformations with dive functionality.
func TestArray(t *testing.T) {
	transformer := New()
	transformer.Register("defaultArr", func(_ context.Context, fl mold.FieldLevel) error {
		if hasValue(fl.Field()) {
			return nil
		}

		fl.Field().Set(reflect.MakeSlice(fl.Field().Type(), 2, 2))

		return nil
	})
	transformer.Register("defaultStr", func(_ context.Context, fl mold.FieldLevel) error {
		if fl.Field().String() == "ok" {
			return errors.New("ALREADY OK")
		}

		fl.Field().SetString("default")

		return nil
	})

	t.Run("DefaultArrayCreationAndDive", func(t *testing.T) {
		type ArrayStruct struct {
			Arr []string `mold:"defaultArr,dive,defaultStr"`
		}

		var tt ArrayStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Array creation and transformation should succeed")
		assert.Len(t, tt.Arr, 2, "Array should have 2 elements")
		assert.Equal(t, "default", tt.Arr[0], "First element should be transformed")
		assert.Equal(t, "default", tt.Arr[1], "Second element should be transformed")
	})

	t.Run("ExistingArrayTransformation", func(t *testing.T) {
		type ExistingArrayStruct struct {
			Arr []string `mold:"defaultArr,dive,defaultStr"`
		}

		tt2 := ExistingArrayStruct{
			Arr: make([]string, 1),
		}
		err := transformer.Struct(context.Background(), &tt2)
		require.NoError(t, err, "Existing array transformation should succeed")
		assert.Len(t, tt2.Arr, 1, "Array length should be preserved")
		assert.Equal(t, "default", tt2.Arr[0], "Array element should be transformed")
	})

	t.Run("TransformationErrorInArrayElement", func(t *testing.T) {
		type ArrayErrorStruct struct {
			Arr []string `mold:"defaultArr,dive,defaultStr"`
		}

		tt3 := ArrayErrorStruct{
			Arr: []string{"ok"},
		}
		err := transformer.Struct(context.Background(), &tt3)
		assert.Error(t, err, "Should return error for invalid array element")
		assert.Equal(t, "ALREADY OK", err.Error(), "Error message should match expected")
	})
}

// TestMap tests map transformations with dive functionality.
func TestMap(t *testing.T) {
	transformer := New()
	transformer.Register("defaultMap", func(_ context.Context, fl mold.FieldLevel) error {
		if hasValue(fl.Field()) {
			return nil
		}

		fl.Field().Set(reflect.MakeMap(fl.Field().Type()))

		return nil
	})
	transformer.Register("defaultStr", func(_ context.Context, fl mold.FieldLevel) error {
		if fl.Field().String() == "ok" {
			return errors.New("ALREADY OK")
		}

		fl.Field().SetString("default")

		return nil
	})

	t.Run("DefaultMapCreation", func(t *testing.T) {
		type MapStruct struct {
			Map map[string]string `mold:"defaultMap,dive,defaultStr"`
		}

		var tt MapStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Map creation should succeed")
		assert.Len(t, tt.Map, 0, "Created map should be empty")
	})

	t.Run("ExistingMapTransformation", func(t *testing.T) {
		type ExistingMapStruct struct {
			Map map[string]string `mold:"defaultMap,dive,defaultStr"`
		}

		tt2 := ExistingMapStruct{
			Map: map[string]string{"key": ""},
		}
		err := transformer.Struct(context.Background(), &tt2)
		require.NoError(t, err, "Existing map transformation should succeed")
		assert.Len(t, tt2.Map, 1, "Map length should be preserved")
		assert.Equal(t, "default", tt2.Map["key"], "Map value should be transformed")
	})

	t.Run("TransformationErrorInMapValue", func(t *testing.T) {
		type MapErrorStruct struct {
			Map map[string]string `mold:"defaultMap,dive,defaultStr"`
		}

		tt3 := MapErrorStruct{
			Map: map[string]string{"key": "ok"},
		}
		err := transformer.Struct(context.Background(), &tt3)
		assert.Error(t, err, "Should return error for invalid map value")
		assert.Equal(t, "ALREADY OK", err.Error(), "Error message should match expected")
	})
}

// TestInterface tests interface transformations with nested structs.
func TestInterface(t *testing.T) {
	type InnerStruct struct {
		STR    string
		String string `mold:"defaultStr"`
	}

	type InnerErrorStruct struct {
		String string `mold:"error"`
	}

	transformer := New()
	transformer.Register("default", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().Set(reflect.ValueOf(InnerStruct{STR: "test"}))

		return nil
	})
	transformer.Register("default2", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().Set(reflect.ValueOf(InnerErrorStruct{}))

		return nil
	})
	transformer.Register("defaultStr", func(_ context.Context, fl mold.FieldLevel) error {
		if hasValue(fl.Field()) && fl.Field().String() == "ok" {
			return errors.New("ALREADY OK")
		}

		fl.Field().Set(reflect.ValueOf("default"))

		return nil
	})
	transformer.Register("error", func(_ context.Context, _ mold.FieldLevel) error {
		return errors.New("BAD VALUE")
	})

	t.Run("InterfaceWithStructTransformation", func(t *testing.T) {
		type InterfaceStruct struct {
			Iface any `mold:"default"`
		}

		var tt InterfaceStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Interface struct transformation should succeed")
		assert.NotNil(t, tt.Iface, "Interface should not be nil")

		inner, ok := tt.Iface.(InnerStruct)
		assert.True(t, ok, "Interface should contain InnerStruct")
		assert.Equal(t, "default", inner.String, "Inner String field should be transformed")
		assert.Equal(t, "test", inner.STR, "Inner STR field should match expected")
	})

	t.Run("InterfaceTransformationError", func(t *testing.T) {
		type InterfaceErrorStruct struct {
			Iface any `mold:"default2"`
		}

		var tt2 InterfaceErrorStruct

		err := transformer.Struct(context.Background(), &tt2)
		assert.Error(t, err, "Should return error for interface transformation with error")
	})

	t.Run("InterfaceStringTransformation", func(t *testing.T) {
		type InterfaceStringStruct struct {
			Iface any `mold:"defaultStr"`
		}

		var tt3 InterfaceStringStruct

		tt3.Iface = "String"
		err := transformer.Struct(context.Background(), &tt3)
		require.NoError(t, err, "Interface string transformation should succeed")
		assert.Equal(t, "default", tt3.Iface.(string), "Interface string should be transformed")
	})

	t.Run("InterfaceNilTransformation", func(t *testing.T) {
		type InterfaceNilStruct struct {
			Iface any `mold:"defaultStr,defaultStr"`
		}

		var tt4 InterfaceNilStruct

		tt4.Iface = nil
		err := transformer.Struct(context.Background(), &tt4)
		require.NoError(t, err, "Interface nil transformation should succeed")
		assert.Equal(t, "default", tt4.Iface.(string), "Nil interface should be transformed to default")
	})

	t.Run("InterfaceTransformationChainError", func(t *testing.T) {
		type InterfaceChainErrorStruct struct {
			Iface any `mold:"defaultStr,error"`
		}

		var tt5 InterfaceChainErrorStruct

		tt5.Iface = "String"
		err := transformer.Struct(context.Background(), &tt5)
		assert.Error(t, err, "Should return error for transformation chain with error")
	})
}

// TestInterfacePtr tests interface pointer transformations.
func TestInterfacePtr(t *testing.T) {
	type InnerPtrStruct struct {
		String string `mold:"defaultStr"`
	}

	transformer := New()
	transformer.Register("default", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().Set(reflect.ValueOf(new(InnerPtrStruct)))

		return nil
	})
	transformer.Register("defaultStr", func(_ context.Context, fl mold.FieldLevel) error {
		if fl.Field().String() == "ok" {
			return errors.New("ALREADY OK")
		}

		fl.Field().SetString("default")

		return nil
	})

	t.Run("InterfacePointerTransformation", func(t *testing.T) {
		type InterfacePtrStruct struct {
			Iface any `mold:"default"`
		}

		var tt InterfacePtrStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Interface pointer transformation should succeed")
		assert.NotNil(t, tt.Iface, "Interface should not be nil")

		inner, ok := tt.Iface.(*InnerPtrStruct)
		assert.True(t, ok, "Interface should contain pointer to InnerPtrStruct")
		assert.Equal(t, "default", inner.String, "Inner String field should be transformed")
	})

	t.Run("InterfaceStructWithoutTransformation", func(t *testing.T) {
		type InterfaceNoTransformStruct struct {
			Iface any
		}

		var tt2 InterfaceNoTransformStruct

		tt2.Iface = InnerPtrStruct{}
		err := transformer.Struct(context.Background(), &tt2)
		require.NoError(t, err, "Interface without transformation should succeed")
	})
}

// TestStructLevel tests struct-level transformation registration and execution.
func TestStructLevel(t *testing.T) {
	type StructLevelStruct struct {
		String string
	}

	transformer := New()
	transformer.RegisterStructLevel(func(_ context.Context, sl mold.StructLevel) error {
		s := sl.Struct().Interface().(StructLevelStruct)
		if s.String == "error" {
			return errors.New("BAD VALUE")
		}

		s.String = "test"
		sl.Struct().Set(reflect.ValueOf(s))

		return nil
	}, StructLevelStruct{})

	t.Run("StructLevelTransformationSuccess", func(t *testing.T) {
		var tt StructLevelStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Struct level transformation should succeed")
		assert.Equal(t, "test", tt.String, "String field should be transformed by struct-level function")
	})

	t.Run("StructLevelTransformationError", func(t *testing.T) {
		var tt StructLevelStruct

		tt.String = "error"
		err := transformer.Struct(context.Background(), &tt)
		assert.Error(t, err, "Should return error for invalid struct-level transformation")
	})
}

// TestTimeType tests transformation of time.Time fields.
func TestTimeType(t *testing.T) {
	transformer := New()
	transformer.Register("default", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().Set(reflect.ValueOf(time.Now()))

		return nil
	})

	t.Run("TimeFieldTransformation", func(t *testing.T) {
		var tt time.Time

		err := transformer.Field(context.Background(), &tt, "default")
		require.NoError(t, err, "Time field transformation should succeed")
	})

	t.Run("TimeFieldWithInvalidDive", func(t *testing.T) {
		var tt time.Time

		err := transformer.Field(context.Background(), &tt, "default,dive")
		assert.Error(t, err, "Should return error for dive on time field")
		assert.True(t, errors.Is(err, ErrInvalidDive), "Error should be ErrInvalidDive")
	})
}

// TestParam tests transformations with parameters.
func TestParam(t *testing.T) {
	transformer := New()
	transformer.Register("ltrim", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().SetString(strings.TrimLeft(fl.Field().String(), fl.Param()))

		return nil
	})

	t.Run("ParameterTransformation", func(t *testing.T) {
		type ParameterStruct struct {
			String string `mold:"ltrim=#$_"`
		}

		tt := ParameterStruct{String: "_test"}
		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Parameter transformation should succeed")
		assert.Equal(t, "test", tt.String, "String should be left-trimmed based on parameter")
	})
}

// TestDiveKeys tests map transformations with keys/endkeys tags.
func TestDiveKeys(t *testing.T) {
	transformer := New()
	transformer.Register("default", func(_ context.Context, fl mold.FieldLevel) error {
		fl.Field().Set(reflect.ValueOf("after"))

		return nil
	})
	transformer.Register("err", func(_ context.Context, _ mold.FieldLevel) error {
		return errors.New("err")
	})

	t.Run("DiveKeysTransformation", func(t *testing.T) {
		type DiveKeysStruct struct {
			Map map[string]string `mold:"dive,keys,default,endkeys,default"`
		}

		test := DiveKeysStruct{
			Map: map[string]string{
				"b4": "b4",
			},
		}

		err := transformer.Struct(context.Background(), &test)
		require.NoError(t, err, "Dive keys transformation should succeed")

		val := test.Map["after"]
		assert.Equal(t, "after", val, "Map key and value should be transformed")
	})

	t.Run("FieldDiveKeysTransformation", func(t *testing.T) {
		m := map[string]string{
			"b4": "b4",
		}

		err := transformer.Field(context.Background(), &m, "dive,keys,default,endkeys,default")
		require.NoError(t, err, "Field dive keys transformation should succeed")

		val := m["after"]
		assert.Equal(t, "after", val, "Map key and value should be transformed")
	})

	t.Run("InvalidKeysTagUsage", func(t *testing.T) {
		m := map[string]string{"b4": "b4"}
		err := transformer.Field(context.Background(), &m, "keys,endkeys,default")
		assert.Equal(t, ErrInvalidKeysTag, err, "Should return ErrInvalidKeysTag for invalid keys usage")
	})

	t.Run("UndefinedKeysTag", func(t *testing.T) {
		m := map[string]string{"b4": "b4"}
		err := transformer.Field(context.Background(), &m, "dive,endkeys,default")
		assert.Equal(t, ErrUndefinedKeysTag, err, "Should return ErrUndefinedKeysTag when keys tag is missing")
	})

	t.Run("UndefinedTagInKeys", func(t *testing.T) {
		m := map[string]string{"b4": "b4"}
		err := transformer.Field(context.Background(), &m, "dive,keys,undefinedtag")

		var undefinedErr *ErrUndefinedTag
		assert.ErrorAs(t, err, &undefinedErr, "Error should be ErrUndefinedTag")
		assert.Equal(t, "undefinedtag", undefinedErr.tag, "Error should contain undefined tag name")
	})

	t.Run("ErrorInKeysTransformation", func(t *testing.T) {
		m := map[string]string{"b4": "b4"}
		err := transformer.Field(context.Background(), &m, "dive,keys,err,endkeys")
		assert.Error(t, err, "Should return error for keys transformation error")
	})

	t.Run("ErrorInValuesTransformation", func(t *testing.T) {
		m := map[string]string{"b4": "b4"}
		err := transformer.Field(context.Background(), &m, "dive,keys,default,endkeys,err")
		assert.Error(t, err, "Should return error for values transformation error")
	})
}

// TestStructArray tests struct array transformations with and without dive.
func TestStructArray(t *testing.T) {
	type ArrayInnerStruct struct {
		String string `mold:"defaultStr"`
	}

	transformer := New()
	transformer.Register("defaultArr", func(_ context.Context, fl mold.FieldLevel) error {
		if hasValue(fl.Field()) {
			return nil
		}

		fl.Field().Set(reflect.MakeSlice(fl.Field().Type(), 2, 2))

		return nil
	})
	transformer.Register("defaultStr", func(_ context.Context, fl mold.FieldLevel) error {
		if fl.Field().String() == "ok" {
			return errors.New("ALREADY OK")
		}

		fl.Field().SetString("default")

		return nil
	})

	t.Run("StructArrayTransformation", func(t *testing.T) {
		type StructArrayStruct struct {
			Inner    ArrayInnerStruct
			Arr      []ArrayInnerStruct `mold:"defaultArr"`
			ArrDive  []ArrayInnerStruct `mold:"defaultArr,dive"`
			ArrNoTag []ArrayInnerStruct
		}

		var tt StructArrayStruct

		err := transformer.Struct(context.Background(), &tt)
		require.NoError(t, err, "Struct array transformation should succeed")
		assert.Len(t, tt.Arr, 2, "Array should have 2 elements")
		assert.Len(t, tt.ArrDive, 2, "Array with dive should have 2 elements")
		assert.Empty(t, tt.Arr[0].String, "Array without dive should not transform elements")
		assert.Empty(t, tt.Arr[1].String, "Array without dive should not transform elements")
		assert.Equal(t, "default", tt.ArrDive[0].String, "Array with dive should transform elements")
		assert.Equal(t, "default", tt.ArrDive[1].String, "Array with dive should transform elements")
		assert.Equal(t, "default", tt.Inner.String, "Inner struct should be transformed")
	})

	t.Run("ExistingArrayWithoutDive", func(t *testing.T) {
		type ExistingArrayNoDiveStruct struct {
			Arr []ArrayInnerStruct `mold:"defaultArr"`
		}

		tt2 := ExistingArrayNoDiveStruct{
			Arr: make([]ArrayInnerStruct, 1),
		}
		err := transformer.Struct(context.Background(), &tt2)
		require.NoError(t, err, "Existing array without dive should succeed")
		assert.Len(t, tt2.Arr, 1, "Array length should be preserved")
		assert.Empty(t, tt2.Arr[0].String, "Array elements should not be transformed without dive")
	})

	t.Run("ExistingArrayValuesPreserved", func(t *testing.T) {
		type PreservedArrayStruct struct {
			Arr []ArrayInnerStruct `mold:"defaultArr"`
		}

		tt3 := PreservedArrayStruct{
			Arr: []ArrayInnerStruct{{"ok"}},
		}
		err := transformer.Struct(context.Background(), &tt3)
		require.NoError(t, err, "Existing array values should be preserved")
		assert.Len(t, tt3.Arr, 1, "Array length should be preserved")
		assert.Equal(t, "ok", tt3.Arr[0].String, "Existing array values should not change without dive")
	})

	t.Run("DiveTransformationError", func(t *testing.T) {
		type DiveErrorStruct struct {
			ArrDive []ArrayInnerStruct `mold:"defaultArr,dive"`
		}

		tt4 := DiveErrorStruct{
			ArrDive: []ArrayInnerStruct{{"ok"}},
		}
		err := transformer.Struct(context.Background(), &tt4)
		assert.Error(t, err, "Should return error for invalid array element transformation")
		assert.Equal(t, "ALREADY OK", err.Error(), "Error message should match expected")
	})

	t.Run("NoTagArray", func(t *testing.T) {
		type NoTagArrayStruct struct {
			ArrNoTag []ArrayInnerStruct
		}

		tt5 := NoTagArrayStruct{
			ArrNoTag: make([]ArrayInnerStruct, 1),
		}
		err := transformer.Struct(context.Background(), &tt5)
		require.NoError(t, err, "Array without tag should be handled")
		assert.Len(t, tt5.ArrNoTag, 1, "Array length should be preserved")
		assert.Empty(t, tt5.ArrNoTag[0].String, "Array elements should not be transformed without tag")
	})
}

// TestSiblingField tests sibling field access and modification during transformation.
func TestSiblingField(t *testing.T) {
	transformer := New()

	transformer.Register("translate", func(_ context.Context, fl mold.FieldLevel) error {
		statusValue := fl.Field().Int()

		var translatedName string

		switch statusValue {
		case 1:
			translatedName = "Active"
		case 2:
			translatedName = "Inactive"
		case 3:
			translatedName = "Pending"
		default:
			translatedName = "Unknown"
		}

		if statusNameField, ok := fl.SiblingField("StatusName"); ok {
			if statusNameField.CanSet() {
				statusNameField.SetString(translatedName)
			}
		}

		return nil
	})

	type UserStruct struct {
		Status     int `mold:"translate"`
		StatusName string
		Name       string
	}

	t.Run("SiblingFieldTranslation", func(t *testing.T) {
		user := &UserStruct{
			Status: 2,
			Name:   "John Doe",
		}

		err := transformer.Struct(context.Background(), user)
		require.NoError(t, err, "Sibling field translation should succeed")

		assert.Equal(t, 2, user.Status, "Status should remain unchanged")
		assert.Equal(t, "Inactive", user.StatusName, "StatusName should be translated based on Status")
		assert.Equal(t, "John Doe", user.Name, "Name should remain unchanged")
	})

	t.Run("SiblingFieldNotFound", func(t *testing.T) {
		transformer.Register("test_missing", func(_ context.Context, fl mold.FieldLevel) error {
			if _, ok := fl.SiblingField("NonExistentField"); ok {
				return errors.New("should not find non-existent field")
			}

			return nil
		})

		type TestStruct struct {
			Field1 string `mold:"test_missing"`
			Field2 string
		}

		test := &TestStruct{Field1: "test"}
		err := transformer.Struct(context.Background(), test)
		require.NoError(t, err, "Should handle non-existent sibling field gracefully")
	})

	t.Run("MultipleStatusTranslations", func(t *testing.T) {
		tests := []struct {
			status   int
			expected string
		}{
			{1, "Active"},
			{2, "Inactive"},
			{3, "Pending"},
			{99, "Unknown"},
		}

		for _, tt := range tests {
			user := &UserStruct{
				Status: tt.status,
				Name:   "Test User",
			}

			err := transformer.Struct(context.Background(), user)
			require.NoError(t, err, "Status translation should succeed")
			assert.Equal(t, tt.expected, user.StatusName, "StatusName should match expected translation")
		}
	})
}
