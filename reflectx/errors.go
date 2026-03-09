package reflectx

import "errors"

// ErrCannotConvertType indicates source type cannot be converted to target type.
var ErrCannotConvertType = errors.New("cannot convert source type to target type")
