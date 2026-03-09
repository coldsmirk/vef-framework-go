package null

import (
	"github.com/guregu/null/v6"
)

type (
	String       = null.String
	Int          = null.Int
	Int16        = null.Int16
	Int32        = null.Int32
	Float        = null.Float
	Byte         = null.Byte
	Value[T any] = null.Value[T]
)

var (
	StringFrom    = null.StringFrom
	StringFromPtr = null.StringFromPtr
	NewString     = null.NewString
	IntFrom       = null.IntFrom
	IntFromPtr    = null.IntFromPtr
	NewInt        = null.NewInt
	Int16From     = null.Int16From
	Int16FromPtr  = null.Int16FromPtr
	NewInt16      = null.NewInt16
	Int32From     = null.Int32From
	Int32FromPtr  = null.Int32FromPtr
	NewInt32      = null.NewInt32
	FloatFrom     = null.FloatFrom
	FloatFromPtr  = null.FloatFromPtr
	NewFloat      = null.NewFloat
	ByteFrom      = null.ByteFrom
	ByteFromPtr   = null.ByteFromPtr
	NewByte       = null.NewByte
)

func ValueFrom[T any](t T) Value[T] {
	return null.ValueFrom(t)
}

func ValueFromPtr[T any](t *T) Value[T] {
	return null.ValueFromPtr(t)
}

func NewValue[T any](t T, valid bool) Value[T] {
	return null.NewValue(t, valid)
}
