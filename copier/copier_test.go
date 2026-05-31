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

// TestCopyValueToPtr tests value to pointer converters.
func TestCopyValueToPtr(t *testing.T) {
	t.Run("StringToPtr", func(t *testing.T) {
		type Src struct{ V string }

		type Dst struct{ V *string }

		var dst Dst
		require.NoError(t, Copy(Src{V: "hello"}, &dst), "String to *string conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, "hello", *dst.V, "Copied field value should match the source value")
	})

	t.Run("BoolToPtr", func(t *testing.T) {
		type Src struct{ V bool }

		type Dst struct{ V *bool }

		var dst Dst
		require.NoError(t, Copy(Src{V: true}, &dst), "Bool to *bool conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.True(t, *dst.V, "Copied field value should match the source value")
	})

	t.Run("IntToPtr", func(t *testing.T) {
		type Src struct{ V int }

		type Dst struct{ V *int }

		var dst Dst
		require.NoError(t, Copy(Src{V: 42}, &dst), "Int to *int conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, 42, *dst.V, "Copied field value should match the source value")
	})

	t.Run("Int8ToPtr", func(t *testing.T) {
		type Src struct{ V int8 }

		type Dst struct{ V *int8 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 8}, &dst), "Int8 to *int8 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, int8(8), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Int16ToPtr", func(t *testing.T) {
		type Src struct{ V int16 }

		type Dst struct{ V *int16 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 100}, &dst), "Int16 to *int16 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, int16(100), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Int32ToPtr", func(t *testing.T) {
		type Src struct{ V int32 }

		type Dst struct{ V *int32 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 12345}, &dst), "Int32 to *int32 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, int32(12345), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Int64ToPtr", func(t *testing.T) {
		type Src struct{ V int64 }

		type Dst struct{ V *int64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 99999}, &dst), "Int64 to *int64 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, int64(99999), *dst.V, "Copied field value should match the source value")
	})

	t.Run("UintToPtr", func(t *testing.T) {
		type Src struct{ V uint }

		type Dst struct{ V *uint }

		var dst Dst
		require.NoError(t, Copy(Src{V: 7}, &dst), "Uint to *uint conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, uint(7), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Uint8ToPtr", func(t *testing.T) {
		type Src struct{ V uint8 }

		type Dst struct{ V *uint8 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 255}, &dst), "Uint8 to *uint8 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, uint8(255), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Uint16ToPtr", func(t *testing.T) {
		type Src struct{ V uint16 }

		type Dst struct{ V *uint16 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 500}, &dst), "Uint16 to *uint16 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, uint16(500), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Uint32ToPtr", func(t *testing.T) {
		type Src struct{ V uint32 }

		type Dst struct{ V *uint32 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 70000}, &dst), "Uint32 to *uint32 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, uint32(70000), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Uint64ToPtr", func(t *testing.T) {
		type Src struct{ V uint64 }

		type Dst struct{ V *uint64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 123456789}, &dst), "Uint64 to *uint64 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, uint64(123456789), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Float32ToPtr", func(t *testing.T) {
		type Src struct{ V float32 }

		type Dst struct{ V *float32 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 1.5}, &dst), "Float32 to *float32 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, float32(1.5), *dst.V, "Copied field value should match the source value")
	})

	t.Run("Float64ToPtr", func(t *testing.T) {
		type Src struct{ V float64 }

		type Dst struct{ V *float64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: 3.14}, &dst), "Float64 to *float64 conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, 3.14, *dst.V, "Copied field value should match the source value")
	})

	t.Run("DecimalToPtr", func(t *testing.T) {
		type Src struct{ V decimal.Decimal }

		type Dst struct{ V *decimal.Decimal }

		d := decimal.NewFromFloat(123.45)

		var dst Dst
		require.NoError(t, Copy(Src{V: d}, &dst), "Decimal to *Decimal conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.True(t, d.Equal(*dst.V), "Copied field value should match the source value")
	})

	t.Run("TimeToPtr", func(t *testing.T) {
		type Src struct{ V time.Time }

		type Dst struct{ V *time.Time }

		v := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "Time value to *time.Time conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, v, *dst.V, "Copied field value should match the source value")
	})

	t.Run("DateTimeToPtr", func(t *testing.T) {
		type Src struct{ V timex.DateTime }

		type Dst struct{ V *timex.DateTime }

		v := timex.DateTime(time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "DateTime value to *timex.DateTime conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, v, *dst.V, "Copied field value should match the source value")
	})

	t.Run("DateToPtr", func(t *testing.T) {
		type Src struct{ V timex.Date }

		type Dst struct{ V *timex.Date }

		v := timex.Date(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "Date value to *timex.Date conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, v, *dst.V, "Copied field value should match the source value")
	})

	t.Run("TimexTimeToPtr", func(t *testing.T) {
		type Src struct{ V timex.Time }

		type Dst struct{ V *timex.Time }

		v := timex.Time(time.Date(0, 1, 1, 15, 30, 45, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: v}, &dst), "Time value to *timex.Time conversion should succeed")
		require.NotNil(t, dst.V, "Destination pointer field should not be nil")
		assert.Equal(t, v, *dst.V, "Copied field value should match the source value")
	})
}

// TestCopyPtrToValue tests pointer to value converters (non-nil and nil).
func TestCopyPtrToValue(t *testing.T) {
	t.Run("StringPtrToValue", func(t *testing.T) {
		type Src struct{ V *string }

		type Dst struct{ V string }

		var dst Dst
		require.NoError(t, Copy(Src{V: new("hello")}, &dst), "Pointer to string conversion should succeed")
		assert.Equal(t, "hello", dst.V, "Copied field value should match the source value")
	})

	t.Run("NilStringPtrToValue", func(t *testing.T) {
		type Src struct{ V *string }

		type Dst struct{ V string }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *string to string conversion should use the zero value")
		assert.Equal(t, "", dst.V, "Nil source pointer should produce the destination zero value")
	})

	t.Run("BoolPtrToValue", func(t *testing.T) {
		type Src struct{ V *bool }

		type Dst struct{ V bool }

		var dst Dst
		require.NoError(t, Copy(Src{V: new(true)}, &dst), "Pointer to bool conversion should succeed")
		assert.True(t, dst.V, "Copied field value should match the source value")
	})

	t.Run("NilBoolPtrToValue", func(t *testing.T) {
		type Src struct{ V *bool }

		type Dst struct{ V bool }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *bool to bool conversion should use the zero value")
		assert.False(t, dst.V, "Nil source pointer should produce the destination zero value")
	})

	t.Run("Int64PtrToValue", func(t *testing.T) {
		type Src struct{ V *int64 }

		type Dst struct{ V int64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: new(int64(42))}, &dst), "Pointer to int64 conversion should succeed")
		assert.Equal(t, int64(42), dst.V, "Copied field value should match the source value")
	})

	t.Run("NilInt64PtrToValue", func(t *testing.T) {
		type Src struct{ V *int64 }

		type Dst struct{ V int64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *int64 to int64 conversion should use the zero value")
		assert.Equal(t, int64(0), dst.V, "Nil source pointer should produce the destination zero value")
	})

	t.Run("Float64PtrToValue", func(t *testing.T) {
		type Src struct{ V *float64 }

		type Dst struct{ V float64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: new(3.14)}, &dst), "Pointer to float64 conversion should succeed")
		assert.Equal(t, 3.14, dst.V, "Copied field value should match the source value")
	})

	t.Run("NilFloat64PtrToValue", func(t *testing.T) {
		type Src struct{ V *float64 }

		type Dst struct{ V float64 }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *float64 to float64 conversion should use the zero value")
		assert.Equal(t, 0.0, dst.V, "Nil source pointer should produce the destination zero value")
	})

	t.Run("DecimalPtrToValue", func(t *testing.T) {
		type Src struct{ V *decimal.Decimal }

		type Dst struct{ V decimal.Decimal }

		d := decimal.NewFromFloat(99.99)

		var dst Dst
		require.NoError(t, Copy(Src{V: &d}, &dst), "Pointer to Decimal conversion should succeed")
		assert.True(t, d.Equal(dst.V), "Copied field value should match the source value")
	})

	t.Run("NilDecimalPtrToValue", func(t *testing.T) {
		type Src struct{ V *decimal.Decimal }

		type Dst struct{ V decimal.Decimal }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *Decimal to Decimal conversion should use the zero value")
		assert.True(t, decimal.Zero.Equal(dst.V), "Nil source pointer should produce the destination zero value")
	})

	t.Run("TimePtrToValue", func(t *testing.T) {
		type Src struct{ V *time.Time }

		type Dst struct{ V time.Time }

		v := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

		var dst Dst
		require.NoError(t, Copy(Src{V: &v}, &dst), "Pointer to time.Time conversion should succeed")
		assert.Equal(t, v, dst.V, "Copied field value should match the source value")
	})

	t.Run("NilTimePtrToValue", func(t *testing.T) {
		type Src struct{ V *time.Time }

		type Dst struct{ V time.Time }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *time.Time to time.Time conversion should use the zero value")
		assert.True(t, dst.V.IsZero(), "Nil source pointer should produce the destination zero value")
	})

	t.Run("DateTimePtrToValue", func(t *testing.T) {
		type Src struct{ V *timex.DateTime }

		type Dst struct{ V timex.DateTime }

		v := timex.DateTime(time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC))

		var dst Dst
		require.NoError(t, Copy(Src{V: &v}, &dst), "Pointer to timex.DateTime conversion should succeed")
		assert.Equal(t, v, dst.V, "Copied field value should match the source value")
	})

	t.Run("NilDateTimePtrToValue", func(t *testing.T) {
		type Src struct{ V *timex.DateTime }

		type Dst struct{ V timex.DateTime }

		var dst Dst
		require.NoError(t, Copy(Src{V: nil}, &dst), "Nil *timex.DateTime to timex.DateTime conversion should use the zero value")
		assert.True(t, time.Time(dst.V).IsZero(), "Nil source pointer should produce the destination zero value")
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
