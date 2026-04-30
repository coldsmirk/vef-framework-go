package excel

import "errors"

// ErrSheetIndexOutOfRange indicates the configured sheet index exceeds the
// actual sheet count of the workbook.
var ErrSheetIndexOutOfRange = errors.New("sheet index out of range")
