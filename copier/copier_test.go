package copier

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// TestCopyBasic tests basic struct copying functionality.
func TestCopyBasic(t *testing.T) {
	t.Run("Struct", func(t *testing.T) {
		type Source struct {
			Name string
			Age  int
		}

		type Dest struct {
			Name string
			Age  int
		}

		src := Source{Name: "John", Age: 30}

		var dst Dest

		require.NoError(t, Copy(src, &dst), "Should copy struct")
		assert.Equal(t, "John", dst.Name, "Name should match")
		assert.Equal(t, 30, dst.Age, "Age should match")
	})
}

// TestCopyValueToPtr tests value → pointer converters.
func TestCopyValueToPtr(t *testing.T) {
	t.Run("StringToPtr", func(t *testing.T) {
		type Src struct{ V string }

		type Dst struct{ V *string }

		var dst Dst
		require.NoError(t, Copy(Src{V: "hello"}, &dst), "string → *string should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, "hello", *dst.V, "value should match")
	})

	t.Run("BoolToPtr", func(t *testing.T) {
		type Src struct{ V bool }

		type Dst struct{ V *bool }

		var dst Dst
		require.NoError(t, Copy(Src{V: true}, &dst), "bool → *bool should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.True(t, *dst.V, "value should match")
	})

	t.Run("IntToPtr", func(t *testing.T) {
		type Src struct{ V int }

		type Dst struct{ V *int }

		var dst Dst
		require.NoError(t, Copy(Src{V: 42}, &dst), "int → *int should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, 42, *dst.V, "value should match")
	})

	t.Run("Int8ToPtr", func(t *testing.T) {
		type Src struct{ V int8 }

		type Dst struct{ V *int8 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 8}, &dst), "int8 → *int8 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, int8(8), *dst.V, "value should match")
	})

	t.Run("Int16ToPtr", func(t *testing.T) {
		type Src struct{ V int16 }

		type Dst struct{ V *int16 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 100}, &dst), "int16 → *int16 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, int16(100), *dst.V, "value should match")
	})

	t.Run("Int32ToPtr", func(t *testing.T) {
		type Src struct{ V int32 }

		type Dst struct{ V *int32 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 12345}, &dst), "int32 → *int32 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, int32(12345), *dst.V, "value should match")
	})

	t.Run("Int64ToPtr", func(t *testing.T) {
		type Src struct{ V int64 }

		type Dst struct{ V *int64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 99999}, &dst), "int64 → *int64 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, int64(99999), *dst.V, "value should match")
	})

	t.Run("UintToPtr", func(t *testing.T) {
		type Src struct{ V uint }

		type Dst struct{ V *uint }

		var dst Dst
		require.NoError(t, Copy(Src{V: 7}, &dst), "uint → *uint should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, uint(7), *dst.V, "value should match")
	})

	t.Run("Uint8ToPtr", func(t *testing.T) {
		type Src struct{ V uint8 }

		type Dst struct{ V *uint8 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 255}, &dst), "uint8 → *uint8 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, uint8(255), *dst.V, "value should match")
	})

	t.Run("Uint16ToPtr", func(t *testing.T) {
		type Src struct{ V uint16 }

		type Dst struct{ V *uint16 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 500}, &dst), "uint16 → *uint16 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, uint16(500), *dst.V, "value should match")
	})

	t.Run("Uint32ToPtr", func(t *testing.T) {
		type Src struct{ V uint32 }

		type Dst struct{ V *uint32 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 70000}, &dst), "uint32 → *uint32 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, uint32(70000), *dst.V, "value should match")
	})

	t.Run("Uint64ToPtr", func(t *testing.T) {
		type Src struct{ V uint64 }

		type Dst struct{ V *uint64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 123456789}, &dst), "uint64 → *uint64 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, uint64(123456789), *dst.V, "value should match")
	})

	t.Run("Float32ToPtr", func(t *testing.T) {
		type Src struct{ V float32 }

		type Dst struct{ V *float32 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 1.5}, &dst), "float32 → *float32 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, float32(1.5), *dst.V, "value should match")
	})

	t.Run("Float64ToPtr", func(t *testing.T) {
		type Src struct{ V float64 }

		type Dst struct{ V *float64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 3.14}, &dst), "float64 → *float64 should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, 3.14, *dst.V, "value should match")
	})

	t.Run("DecimalToPtr", func(t *testing.T) {
		type Src struct{ V decimal.Decimal }

		type Dst struct{ V *decimal.Decimal }

		d := decimal.NewFromFloat(123.45)

		var dst Dst
		require.NoError(t, Copy(Src{V: d}, &dst), "Decimal → *Decimal should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.True(t, d.Equal(*dst.V), "value should match")
	})

	t.Run("TimeToPtr", func(t *testing.T) {
		type Src struct{ V time.Time }

		type Dst struct{ V *time.Time }

		v := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "time.Time → *time.Time should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, v, *dst.V, "value should match")
	})

	t.Run("DateTimeToPtr", func(t *testing.T) {
		type Src struct{ V timex.DateTime }

		type Dst struct{ V *timex.DateTime }

		v := timex.DateTime(time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "timex.DateTime → *timex.DateTime should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, v, *dst.V, "value should match")
	})

	t.Run("DateToPtr", func(t *testing.T) {
		type Src struct{ V timex.Date }

		type Dst struct{ V *timex.Date }

		v := timex.Date(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "timex.Date → *timex.Date should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, v, *dst.V, "value should match")
	})

	t.Run("TimexTimeToPtr", func(t *testing.T) {
		type Src struct{ V timex.Time }

		type Dst struct{ V *timex.Time }

		v := timex.Time(time.Date(0, 1, 1, 15, 30, 45, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "timex.Time → *timex.Time should succeed")
		require.NotNil(t, dst.V, "pointer should not be nil")
		assert.Equal(t, v, *dst.V, "value should match")
	})
}

// TestCopyPtrToValue tests pointer → value converters (non-nil and nil).
func TestCopyPtrToValue(t *testing.T) {
	t.Run("StringPtrToValue", func(t *testing.T) {
		type Src struct{ V *string }

		type Dst struct{ V string }

		var dst Dst
		require.NoError(t, Copy(Src{V: new("hello")}, &dst), "*string → string should succeed")
		assert.Equal(t, "hello", dst.V, "value should match")
	})

	t.Run("NilStringPtrToValue", func(t *testing.T) {
		type Src struct{ V *string }

		type Dst struct{ V string }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *string → string should use zero value")
		assert.Equal(t, "", dst.V, "nil pointer should produce zero value")
	})

	t.Run("BoolPtrToValue", func(t *testing.T) {
		type Src struct{ V *bool }

		type Dst struct{ V bool }

		var dst Dst
		require.NoError(t, Copy(Src{V: new(true)}, &dst), "*bool → bool should succeed")
		assert.True(t, dst.V, "value should match")
	})

	t.Run("NilBoolPtrToValue", func(t *testing.T) {
		type Src struct{ V *bool }

		type Dst struct{ V bool }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *bool → bool should use zero value")
		assert.False(t, dst.V, "nil pointer should produce zero value")
	})

	t.Run("Int64PtrToValue", func(t *testing.T) {
		type Src struct{ V *int64 }

		type Dst struct{ V int64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: new(int64(42))}, &dst), "*int64 → int64 should succeed")
		assert.Equal(t, int64(42), dst.V, "value should match")
	})

	t.Run("NilInt64PtrToValue", func(t *testing.T) {
		type Src struct{ V *int64 }

		type Dst struct{ V int64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *int64 → int64 should use zero value")
		assert.Equal(t, int64(0), dst.V, "nil pointer should produce zero value")
	})

	t.Run("Float64PtrToValue", func(t *testing.T) {
		type Src struct{ V *float64 }

		type Dst struct{ V float64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: new(3.14)}, &dst), "*float64 → float64 should succeed")
		assert.Equal(t, 3.14, dst.V, "value should match")
	})

	t.Run("NilFloat64PtrToValue", func(t *testing.T) {
		type Src struct{ V *float64 }

		type Dst struct{ V float64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *float64 → float64 should use zero value")
		assert.Equal(t, 0.0, dst.V, "nil pointer should produce zero value")
	})

	t.Run("DecimalPtrToValue", func(t *testing.T) {
		type Src struct{ V *decimal.Decimal }

		type Dst struct{ V decimal.Decimal }

		d := decimal.NewFromFloat(99.99)

		var dst Dst
		require.NoError(t, Copy(Src{V: &d}, &dst), "*Decimal → Decimal should succeed")
		assert.True(t, d.Equal(dst.V), "value should match")
	})

	t.Run("NilDecimalPtrToValue", func(t *testing.T) {
		type Src struct{ V *decimal.Decimal }

		type Dst struct{ V decimal.Decimal }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *Decimal → Decimal should use zero value")
		assert.True(t, decimal.Zero.Equal(dst.V), "nil pointer should produce zero value")
	})

	t.Run("TimePtrToValue", func(t *testing.T) {
		type Src struct{ V *time.Time }

		type Dst struct{ V time.Time }

		v := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

		var dst Dst
		require.NoError(t, Copy(Src{V: &v}, &dst), "*time.Time → time.Time should succeed")
		assert.Equal(t, v, dst.V, "value should match")
	})

	t.Run("NilTimePtrToValue", func(t *testing.T) {
		type Src struct{ V *time.Time }

		type Dst struct{ V time.Time }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *time.Time → time.Time should use zero value")
		assert.True(t, dst.V.IsZero(), "nil pointer should produce zero value")
	})

	t.Run("DateTimePtrToValue", func(t *testing.T) {
		type Src struct{ V *timex.DateTime }

		type Dst struct{ V timex.DateTime }

		v := timex.DateTime(time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: &v}, &dst), "*timex.DateTime → timex.DateTime should succeed")
		assert.Equal(t, v, dst.V, "value should match")
	})

	t.Run("NilDateTimePtrToValue", func(t *testing.T) {
		type Src struct{ V *timex.DateTime }

		type Dst struct{ V timex.DateTime }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "nil *timex.DateTime → timex.DateTime should use zero value")
		assert.True(t, time.Time(dst.V).IsZero(), "nil pointer should produce zero value")
	})
}

// TestCopyOptions tests copy options like IgnoreEmpty and CaseInsensitive.
func TestCopyOptions(t *testing.T) {
	t.Run("IgnoreEmpty", func(t *testing.T) {
		type Source struct {
			Name string
			Age  int
		}

		type Dest struct {
			Name string
			Age  int
		}

		dst := Dest{Name: "Initial Name", Age: 25}
		src := Source{Name: "", Age: 30}

		require.NoError(t, Copy(src, &dst, WithIgnoreEmpty()), "Should copy with ignore empty option")
		assert.Equal(t, 30, dst.Age, "Age should be updated")
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		type Source struct {
			NAME string
		}

		type Dest struct {
			Name string
		}

		src := Source{NAME: "John Doe"}

		var dst Dest

		require.NoError(t, Copy(src, &dst, WithCaseInsensitive()), "Should copy with case insensitive option")
		assert.Equal(t, "John Doe", dst.Name, "Name should match despite case difference")
	})
}

// TestCopyDeepCopy tests copy with deep copy option.
func TestCopyDeepCopy(t *testing.T) {
	t.Run("DeepCopySlice", func(t *testing.T) {
		type Source struct {
			Tags []string
		}

		type Dest struct {
			Tags []string
		}

		src := Source{Tags: []string{"a", "b", "c"}}

		var dst Dest

		require.NoError(t, Copy(src, &dst, WithDeepCopy()), "Should copy with deep copy option")
		assert.Equal(t, []string{"a", "b", "c"}, dst.Tags, "Tags should match")

		// Modify source to verify deep copy
		src.Tags[0] = "modified"
		assert.Equal(t, "a", dst.Tags[0], "Deep copy should not share underlying array")
	})

	t.Run("DeepCopyNestedStruct", func(t *testing.T) {
		type Inner struct {
			Value string
		}

		type Source struct {
			Inner *Inner
		}

		type Dest struct {
			Inner *Inner
		}

		src := Source{Inner: &Inner{Value: "test"}}

		var dst Dest

		require.NoError(t, Copy(src, &dst, WithDeepCopy()), "Should deep copy nested struct")
		require.NotNil(t, dst.Inner, "Inner should not be nil")
		assert.Equal(t, "test", dst.Inner.Value, "Inner value should match")
	})
}

// TestCopyFieldNameMapping tests copy with field name mapping.
func TestCopyFieldNameMapping(t *testing.T) {
	t.Run("MappedFields", func(t *testing.T) {
		type Source struct {
			FullName string
		}

		type Dest struct {
			Name string
		}

		src := Source{FullName: "John Doe"}

		var dst Dest

		require.NoError(t, Copy(src, &dst, WithFieldNameMapping(
			FieldNameMapping{
				SrcType: Source{},
				DstType: Dest{},
				Mapping: map[string]string{
					"FullName": "Name",
				},
			},
		)), "Should copy with field name mapping")
		assert.Equal(t, "John Doe", dst.Name, "Mapped field should match")
	})
}

// TestCopyError tests error handling for invalid inputs.
func TestCopyError(t *testing.T) {
	t.Run("NonPointerDestination", func(t *testing.T) {
		type Source struct {
			Name string
		}

		type Dest struct {
			Name string
		}

		src := Source{Name: "John"}
		dst := Dest{}

		err := Copy(src, dst)
		assert.Error(t, err, "Should return error for non-pointer destination")
	})
}
