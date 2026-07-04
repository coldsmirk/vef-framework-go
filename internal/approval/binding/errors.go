package binding

import "errors"

// ErrBindingMisconfigured signals that a Flow with BindingMode=business is
// missing one of the required columns (business_table / business_pk_field /
// business_status_field), carries an unsafe identifier, or resolved to an
// empty record id. The Listener acknowledges instead of retrying: a
// misconfiguration does not heal by retry.
var ErrBindingMisconfigured = errors.New("approval: business binding misconfigured")
