package approval

// NodeResolveContext is the runtime snapshot that node-level principal
// resolution runs against — shared verbatim by assignee and CC resolution, so
// a host kind written for one has exactly the same information available in
// the other.
//
// It carries the live models rather than a copy of their scalars: a custom
// kind such as "the applicant's head nurse" needs the applicant's department,
// a tenant-scoped expert panel needs the tenant, and a rule keyed to one step
// of one flow needs the node key — enumerating those would fix today's guesses
// about what a host might read into the contract. The models are the
// framework's, not the resolver's: treat them as read-only.
type NodeResolveContext struct {
	// Instance is the running approval instance, including the applicant
	// snapshot, the tenant, the flow code, and the Globals the host's
	// InstanceGlobalsResolver captured at initiation.
	Instance *Instance
	// Node is the flow node being processed. Its Key is the designer-assigned
	// identifier, stable across deploys of the same step.
	Node *FlowNode
	// FormData is the form data as it stands at resolution time. It is
	// passed separately from Instance.FormData because an in-flight
	// transaction resolves against the data being written, not the row.
	FormData FormData
	// UserResolver resolves display info for user IDs. Resolvers that produce
	// bare IDs may use it to fill in names and departments; those that get
	// UserInfo from the host AssigneeService need not.
	UserResolver UserInfoResolver
}

// Applicant returns the applicant snapshot recorded on the instance — who
// started it, and which department they belonged to at that moment.
func (c *NodeResolveContext) Applicant() UserInfo {
	if c.Instance == nil {
		return UserInfo{}
	}

	return UserInfo{
		ID:             c.Instance.ApplicantID,
		Name:           c.Instance.ApplicantName,
		DepartmentID:   c.Instance.ApplicantDepartmentID,
		DepartmentName: c.Instance.ApplicantDepartmentName,
	}
}
