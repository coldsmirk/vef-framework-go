package crud

import "errors"

// ErrModelNoPrimaryKey indicates the model schema has no primary key.
var ErrModelNoPrimaryKey = errors.New("model has no primary key")

// ErrAuditUserCompositePK indicates the audit user model has a composite primary key which is not supported.
var ErrAuditUserCompositePK = errors.New("audit user model has composite primary key, only single primary key is supported")

// errSearchTypeMismatch indicates a type mismatch in search parameter conversion.
var errSearchTypeMismatch = errors.New("search type mismatch")

// errColumnNotFound indicates a column does not exist in the model.
var errColumnNotFound = errors.New("column does not exist in model")
