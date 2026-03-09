package csv

import "errors"

var (
	ErrDataMustBeSlice     = errors.New("data must be a slice")
	ErrNoDataRowsFound     = errors.New("no data rows found")
	ErrDuplicateColumnName = errors.New("duplicate column name")
	ErrFieldNotSettable    = errors.New("field is not settable")
)
