package excel

import "errors"

var (
	ErrDataMustBeSlice      = errors.New("data must be a slice")
	ErrSheetIndexOutOfRange = errors.New("sheet index out of range")
	ErrNoDataRowsFound      = errors.New("no data rows found")
	ErrDuplicateColumnName  = errors.New("duplicate column name")
	ErrFieldNotSettable     = errors.New("field is not settable")
)
