package approval

import "errors"

// ErrDBRequired is returned by every Service method given a nil orm.DB
// handle. The handle is the transaction boundary and the audit-operator
// carrier, so substituting a default for it would silently run the operation
// outside the caller's transaction and attribute it to the system.
var ErrDBRequired = errors.New("approval: operation requires an orm.DB handle")

// FormFieldIDs errors. A rule of a SelectionFormField kind names its field at
// deploy time, so both indicate a configuration fault rather than bad input.
var (
	// ErrFormFieldNameEmpty is returned when a rule names no form field.
	ErrFormFieldNameEmpty = errors.New("approval: form field name is empty")
	// ErrUnsupportedFieldValueType is returned when a form field holds a value
	// that cannot carry IDs — neither a string nor a list of them.
	ErrUnsupportedFieldValueType = errors.New("approval: unsupported form field value type")
)

var (
	errModelBusinessTypes = errors.New("approval: business model and parameters must be structs")
	errModelBusinessKey   = errors.New("approval: business model must have one string primary key")
)
