package approval

import "errors"

// ErrDBRequired is returned by every Service method given a nil orm.DB
// handle. The handle is the transaction boundary and the audit-operator
// carrier, so substituting a default for it would silently run the operation
// outside the caller's transaction and attribute it to the system.
var ErrDBRequired = errors.New("approval: operation requires an orm.DB handle")
