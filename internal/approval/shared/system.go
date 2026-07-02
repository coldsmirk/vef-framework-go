package shared

import "github.com/coldsmirk/vef-framework-go/approval"

// SystemOperator is the operator identity stamped on actions the engine
// performs without a human decision: timeout auto-processing, auto-pass
// execution types, consecutive-approver passes, and similar. Sharing one
// identity keeps audit trails queryable by a single well-known operator ID.
var SystemOperator = approval.UserInfo{ID: "system", Name: "系统"}
