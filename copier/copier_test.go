package copier

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/null"
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

// TestCopyConverters tests type converters between null and non-null types.
func TestCopyConverters(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "NullStringToString",
			run: func(t *testing.T) {
				type Source struct {
					Value null.String
				}

				type Dest struct {
					Value string
				}

				src := Source{Value: null.StringFrom("test")}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.String to string")
				assert.Equal(t, "test", dst.Value, "Converted value should match")
			},
		},
		{
			name: "StringToNullString",
			run: func(t *testing.T) {
				type Source struct {
					Value string
				}

				type Dest struct {
					Value null.String
				}

				src := Source{Value: "test"}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert string to null.String")
				assert.True(t, dst.Value.Valid, "null.String should be valid")
				assert.Equal(t, "test", dst.Value.String, "Converted value should match")
			},
		},
		{
			name: "NullIntToInt64",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Int
				}

				type Dest struct {
					Value int64
				}

				src := Source{Value: null.IntFrom(42)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Int to int64")
				assert.Equal(t, int64(42), dst.Value, "Converted value should match")
			},
		},
		{
			name: "Int64ToNullInt",
			run: func(t *testing.T) {
				type Source struct {
					Value int64
				}

				type Dest struct {
					Value null.Int
				}

				src := Source{Value: 42}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert int64 to null.Int")
				assert.True(t, dst.Value.Valid, "null.Int should be valid")
				assert.Equal(t, int64(42), dst.Value.Int64, "Converted value should match")
			},
		},
		{
			name: "NullInt16ToInt16",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Int16
				}

				type Dest struct {
					Value int16
				}

				src := Source{Value: null.Int16From(100)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Int16 to int16")
				assert.Equal(t, int16(100), dst.Value, "Converted value should match")
			},
		},
		{
			name: "Int16ToNullInt16",
			run: func(t *testing.T) {
				type Source struct {
					Value int16
				}

				type Dest struct {
					Value null.Int16
				}

				src := Source{Value: 200}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert int16 to null.Int16")
				assert.True(t, dst.Value.Valid, "null.Int16 should be valid")
				assert.Equal(t, int16(200), dst.Value.Int16, "Converted value should match")
			},
		},
		{
			name: "NullInt32ToInt32",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Int32
				}

				type Dest struct {
					Value int32
				}

				src := Source{Value: null.Int32From(12345)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Int32 to int32")
				assert.Equal(t, int32(12345), dst.Value, "Converted value should match")
			},
		},
		{
			name: "Int32ToNullInt32",
			run: func(t *testing.T) {
				type Source struct {
					Value int32
				}

				type Dest struct {
					Value null.Int32
				}

				src := Source{Value: 54321}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert int32 to null.Int32")
				assert.True(t, dst.Value.Valid, "null.Int32 should be valid")
				assert.Equal(t, int32(54321), dst.Value.Int32, "Converted value should match")
			},
		},
		{
			name: "NullFloatToFloat64",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Float
				}

				type Dest struct {
					Value float64
				}

				src := Source{Value: null.FloatFrom(3.14)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Float to float64")
				assert.Equal(t, 3.14, dst.Value, "Converted value should match")
			},
		},
		{
			name: "Float64ToNullFloat",
			run: func(t *testing.T) {
				type Source struct {
					Value float64
				}

				type Dest struct {
					Value null.Float
				}

				src := Source{Value: 3.14}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert float64 to null.Float")
				assert.True(t, dst.Value.Valid, "null.Float should be valid")
				assert.Equal(t, 3.14, dst.Value.Float64, "Converted value should match")
			},
		},
		{
			name: "NullByteToByte",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Byte
				}

				type Dest struct {
					Value byte
				}

				src := Source{Value: null.ByteFrom(255)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Byte to byte")
				assert.Equal(t, byte(255), dst.Value, "Converted value should match")
			},
		},
		{
			name: "ByteToNullByte",
			run: func(t *testing.T) {
				type Source struct {
					Value byte
				}

				type Dest struct {
					Value null.Byte
				}

				src := Source{Value: 128}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert byte to null.Byte")
				assert.True(t, dst.Value.Valid, "null.Byte should be valid")
				assert.Equal(t, byte(128), dst.Value.Byte, "Converted value should match")
			},
		},
		{
			name: "NullBoolToBool",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Bool
				}

				type Dest struct {
					Value bool
				}

				src := Source{Value: null.BoolFrom(true)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Bool to bool")
				assert.True(t, dst.Value, "Converted value should be true")
			},
		},
		{
			name: "BoolToNullBool",
			run: func(t *testing.T) {
				type Source struct {
					Value bool
				}

				type Dest struct {
					Value null.Bool
				}

				src := Source{Value: true}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert bool to null.Bool")
				assert.True(t, dst.Value.Valid, "null.Bool should be valid")
				assert.True(t, dst.Value.Bool, "Converted value should be true")
			},
		},
		{
			name: "NullDateTimeToDateTime",
			run: func(t *testing.T) {
				type Source struct {
					Value null.DateTime
				}

				type Dest struct {
					Value timex.DateTime
				}

				testValue := timex.Of(time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC))
				src := Source{Value: null.DateTimeFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.DateTime to timex.DateTime")
				assert.Equal(t, testValue, dst.Value, "Converted value should match")
			},
		},
		{
			name: "DateTimeToNullDateTime",
			run: func(t *testing.T) {
				type Source struct {
					Value timex.DateTime
				}

				type Dest struct {
					Value null.DateTime
				}

				testValue := timex.Of(time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC))
				src := Source{Value: testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert timex.DateTime to null.DateTime")
				assert.True(t, dst.Value.Valid, "null.DateTime should be valid")
				assert.Equal(t, testValue, dst.Value.V, "Converted value should match")
			},
		},
		{
			name: "NullDateToDate",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Date
				}

				type Dest struct {
					Value timex.Date
				}

				testValue := timex.DateOf(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC))
				src := Source{Value: null.DateFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Date to timex.Date")
				assert.Equal(t, testValue, dst.Value, "Converted value should match")
			},
		},
		{
			name: "DateToNullDate",
			run: func(t *testing.T) {
				type Source struct {
					Value timex.Date
				}

				type Dest struct {
					Value null.Date
				}

				testValue := timex.DateOf(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC))
				src := Source{Value: testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert timex.Date to null.Date")
				assert.True(t, dst.Value.Valid, "null.Date should be valid")
				assert.Equal(t, testValue, dst.Value.V, "Converted value should match")
			},
		},
		{
			name: "NullTimeToTime",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Time
				}

				type Dest struct {
					Value timex.Time
				}

				testValue := timex.TimeOf(time.Date(0, 1, 1, 15, 30, 45, 0, time.UTC))
				src := Source{Value: null.TimeFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Time to timex.Time")
				assert.Equal(t, testValue, dst.Value, "Converted value should match")
			},
		},
		{
			name: "TimeToNullTime",
			run: func(t *testing.T) {
				type Source struct {
					Value timex.Time
				}

				type Dest struct {
					Value null.Time
				}

				testValue := timex.TimeOf(time.Date(0, 1, 1, 15, 30, 45, 0, time.UTC))
				src := Source{Value: testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert timex.Time to null.Time")
				assert.True(t, dst.Value.Valid, "null.Time should be valid")
				assert.Equal(t, testValue, dst.Value.V, "Converted value should match")
			},
		},
		{
			name: "NullDecimalToDecimal",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Decimal
				}

				type Dest struct {
					Value decimal.Decimal
				}

				testDecimal := decimal.NewFromFloat(123.45)
				src := Source{Value: null.DecimalFrom(testDecimal)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Decimal to decimal.Decimal")
				assert.True(t, testDecimal.Equal(dst.Value), "Converted value should match")
			},
		},
		{
			name: "DecimalToNullDecimal",
			run: func(t *testing.T) {
				type Source struct {
					Value decimal.Decimal
				}

				type Dest struct {
					Value null.Decimal
				}

				testDecimal := decimal.NewFromFloat(123.45)
				src := Source{Value: testDecimal}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert decimal.Decimal to null.Decimal")
				assert.True(t, dst.Value.Valid, "null.Decimal should be valid")
				assert.True(t, testDecimal.Equal(dst.Value.Decimal), "Converted value should match")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}

// TestCopyPointerConverters tests type converters between null types and pointers.
func TestCopyPointerConverters(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "NullStringToStringPtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.String
				}

				type Dest struct {
					Value *string
				}

				src := Source{Value: null.StringFrom("pointer")}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.String to string pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, "pointer", *dst.Value, "Converted value should match")
			},
		},
		{
			name: "StringPtrToNullString",
			run: func(t *testing.T) {
				type Source struct {
					Value *string
				}

				type Dest struct {
					Value null.String
				}

				value := "pointer"
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert string pointer to null.String")
				assert.True(t, dst.Value.Valid, "null.String should be valid")
				assert.Equal(t, "pointer", dst.Value.String, "Converted value should match")
			},
		},
		{
			name: "NilStringPtrToNullString",
			run: func(t *testing.T) {
				type Source struct {
					Value *string
				}

				type Dest struct {
					Value null.String
				}

				var (
					src Source
					dst Dest
				)

				require.NoError(t, Copy(src, &dst), "Should handle nil string pointer")
				assert.False(t, dst.Value.Valid, "null.String should be invalid")
			},
		},
		{
			name: "InvalidNullStringToStringPtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.String
				}

				type Dest struct {
					Value *string
				}

				src := Source{Value: null.NewString("", false)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should handle invalid null.String")
				assert.Nil(t, dst.Value, "Pointer should be nil for invalid null.String")
			},
		},
		{
			name: "NullIntToIntPtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Int
				}

				type Dest struct {
					Value *int64
				}

				src := Source{Value: null.IntFrom(42)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Int to int64 pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, int64(42), *dst.Value, "Converted value should match")
			},
		},
		{
			name: "IntPtrToNullInt",
			run: func(t *testing.T) {
				type Source struct {
					Value *int64
				}

				type Dest struct {
					Value null.Int
				}

				value := int64(42)
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert int64 pointer to null.Int")
				assert.True(t, dst.Value.Valid, "null.Int should be valid")
				assert.Equal(t, int64(42), dst.Value.Int64, "Converted value should match")
			},
		},
		{
			name: "NullBoolToBoolPtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Bool
				}

				type Dest struct {
					Value *bool
				}

				src := Source{Value: null.BoolFrom(true)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Bool to bool pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.True(t, *dst.Value, "Converted value should be true")
			},
		},
		{
			name: "BoolPtrToNullBool",
			run: func(t *testing.T) {
				type Source struct {
					Value *bool
				}

				type Dest struct {
					Value null.Bool
				}

				value := false
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert bool pointer to null.Bool")
				assert.True(t, dst.Value.Valid, "null.Bool should be valid")
				assert.False(t, dst.Value.Bool, "Converted value should be false")
			},
		},
		{
			name: "NullInt16ToInt16Ptr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Int16
				}

				type Dest struct {
					Value *int16
				}

				src := Source{Value: null.Int16From(123)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Int16 to int16 pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, int16(123), *dst.Value, "Converted value should match")
			},
		},
		{
			name: "Int16PtrToNullInt16",
			run: func(t *testing.T) {
				type Source struct {
					Value *int16
				}

				type Dest struct {
					Value null.Int16
				}

				value := int16(321)
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert int16 pointer to null.Int16")
				assert.True(t, dst.Value.Valid, "null.Int16 should be valid")
				assert.Equal(t, int16(321), dst.Value.Int16, "Converted value should match")
			},
		},
		{
			name: "NullInt32ToInt32Ptr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Int32
				}

				type Dest struct {
					Value *int32
				}

				src := Source{Value: null.Int32From(111)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Int32 to int32 pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, int32(111), *dst.Value, "Converted value should match")
			},
		},
		{
			name: "Int32PtrToNullInt32",
			run: func(t *testing.T) {
				type Source struct {
					Value *int32
				}

				type Dest struct {
					Value null.Int32
				}

				value := int32(222)
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert int32 pointer to null.Int32")
				assert.True(t, dst.Value.Valid, "null.Int32 should be valid")
				assert.Equal(t, int32(222), dst.Value.Int32, "Converted value should match")
			},
		},
		{
			name: "NullFloatToFloatPtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Float
				}

				type Dest struct {
					Value *float64
				}

				src := Source{Value: null.FloatFrom(9.87)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Float to float64 pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, 9.87, *dst.Value, "Converted value should match")
			},
		},
		{
			name: "FloatPtrToNullFloat",
			run: func(t *testing.T) {
				type Source struct {
					Value *float64
				}

				type Dest struct {
					Value null.Float
				}

				value := 6.54
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert float64 pointer to null.Float")
				assert.True(t, dst.Value.Valid, "null.Float should be valid")
				assert.Equal(t, value, dst.Value.Float64, "Converted value should match")
			},
		},
		{
			name: "NullByteToBytePtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Byte
				}

				type Dest struct {
					Value *byte
				}

				src := Source{Value: null.ByteFrom(77)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Byte to byte pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, byte(77), *dst.Value, "Converted value should match")
			},
		},
		{
			name: "BytePtrToNullByte",
			run: func(t *testing.T) {
				type Source struct {
					Value *byte
				}

				type Dest struct {
					Value null.Byte
				}

				value := byte(88)
				src := Source{Value: &value}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert byte pointer to null.Byte")
				assert.True(t, dst.Value.Valid, "null.Byte should be valid")
				assert.Equal(t, byte(88), dst.Value.Byte, "Converted value should match")
			},
		},
		{
			name: "NullDateTimeToDateTimePtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.DateTime
				}

				type Dest struct {
					Value *timex.DateTime
				}

				testValue := timex.Of(time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC))
				src := Source{Value: null.DateTimeFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.DateTime to timex.DateTime pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, testValue, *dst.Value, "Converted value should match")
			},
		},
		{
			name: "DateTimePtrToNullDateTime",
			run: func(t *testing.T) {
				type Source struct {
					Value *timex.DateTime
				}

				type Dest struct {
					Value null.DateTime
				}

				testValue := timex.Of(time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC))
				src := Source{Value: &testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert timex.DateTime pointer to null.DateTime")
				assert.True(t, dst.Value.Valid, "null.DateTime should be valid")
				assert.Equal(t, testValue, dst.Value.V, "Converted value should match")
			},
		},
		{
			name: "NullDateToDatePtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Date
				}

				type Dest struct {
					Value *timex.Date
				}

				testValue := timex.DateOf(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
				src := Source{Value: null.DateFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Date to timex.Date pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, testValue, *dst.Value, "Converted value should match")
			},
		},
		{
			name: "DatePtrToNullDate",
			run: func(t *testing.T) {
				type Source struct {
					Value *timex.Date
				}

				type Dest struct {
					Value null.Date
				}

				testValue := timex.DateOf(time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC))
				src := Source{Value: &testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert timex.Date pointer to null.Date")
				assert.True(t, dst.Value.Valid, "null.Date should be valid")
				assert.Equal(t, testValue, dst.Value.V, "Converted value should match")
			},
		},
		{
			name: "NullTimeToTimePtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Time
				}

				type Dest struct {
					Value *timex.Time
				}

				testValue := timex.TimeOf(time.Date(0, 1, 1, 10, 20, 30, 0, time.UTC))
				src := Source{Value: null.TimeFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Time to timex.Time pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.Equal(t, testValue, *dst.Value, "Converted value should match")
			},
		},
		{
			name: "TimePtrToNullTime",
			run: func(t *testing.T) {
				type Source struct {
					Value *timex.Time
				}

				type Dest struct {
					Value null.Time
				}

				testValue := timex.TimeOf(time.Date(0, 1, 1, 5, 10, 15, 0, time.UTC))
				src := Source{Value: &testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert timex.Time pointer to null.Time")
				assert.True(t, dst.Value.Valid, "null.Time should be valid")
				assert.Equal(t, testValue, dst.Value.V, "Converted value should match")
			},
		},
		{
			name: "NullDecimalToDecimalPtr",
			run: func(t *testing.T) {
				type Source struct {
					Value null.Decimal
				}

				type Dest struct {
					Value *decimal.Decimal
				}

				testValue := decimal.NewFromFloat(456.78)
				src := Source{Value: null.DecimalFrom(testValue)}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert null.Decimal to decimal.Decimal pointer")
				require.NotNil(t, dst.Value, "Pointer should not be nil")
				assert.True(t, testValue.Equal(*dst.Value), "Converted value should match")
			},
		},
		{
			name: "DecimalPtrToNullDecimal",
			run: func(t *testing.T) {
				type Source struct {
					Value *decimal.Decimal
				}

				type Dest struct {
					Value null.Decimal
				}

				testValue := decimal.NewFromFloat(654.32)
				src := Source{Value: &testValue}

				var dst Dest

				require.NoError(t, Copy(src, &dst), "Should convert decimal.Decimal pointer to null.Decimal")
				assert.True(t, dst.Value.Valid, "null.Decimal should be valid")
				assert.True(t, testValue.Equal(dst.Value.Decimal), "Converted value should match")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}

// TestCopyIntegration tests integration scenarios with multiple field conversions.
func TestCopyIntegration(t *testing.T) {
	t.Run("NullToBasic", func(t *testing.T) {
		type Source struct {
			Name   null.String
			Age    null.Int
			Active null.Bool
		}

		type Dest struct {
			Name   string
			Age    int64
			Active bool
		}

		src := Source{
			Name:   null.StringFrom("John Doe"),
			Age:    null.IntFrom(30),
			Active: null.BoolFrom(true),
		}

		var dst Dest

		require.NoError(t, Copy(src, &dst), "Should convert multiple null types to basic types")
		assert.Equal(t, "John Doe", dst.Name, "Name should match")
		assert.Equal(t, int64(30), dst.Age, "Age should match")
		assert.True(t, dst.Active, "Active should be true")
	})

	t.Run("BasicToNull", func(t *testing.T) {
		type Source struct {
			Name   string
			Age    int64
			Active bool
		}

		type Dest struct {
			Name   null.String
			Age    null.Int
			Active null.Bool
		}

		src := Source{
			Name:   "Jane Doe",
			Age:    28,
			Active: false,
		}

		var dst Dest

		require.NoError(t, Copy(src, &dst), "Should convert multiple basic types to null types")
		assert.True(t, dst.Name.Valid, "Name should be valid")
		assert.Equal(t, "Jane Doe", dst.Name.String, "Name should match")
		assert.True(t, dst.Age.Valid, "Age should be valid")
		assert.Equal(t, int64(28), dst.Age.Int64, "Age should match")
		assert.True(t, dst.Active.Valid, "Active should be valid")
		assert.False(t, dst.Active.Bool, "Active should be false")
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

// TestCopyCustomTypeConverters tests copy with custom type converters.
func TestCopyCustomTypeConverters(t *testing.T) {
	t.Run("CustomStringToIntConverter", func(t *testing.T) {
		type Source struct {
			Value string
		}

		type Dest struct {
			Value string
		}

		src := Source{Value: "hello"}

		var dst Dest

		require.NoError(t, Copy(src, &dst, WithTypeConverters()), "Should copy with empty custom converters")
		assert.Equal(t, "hello", dst.Value, "Value should match")
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
