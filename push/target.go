package push

// TargetKind enumerates the recipient selectors a push can address.
type TargetKind string

const (
	// TargetUsers selects the connections of specific user IDs.
	TargetUsers TargetKind = "users"
	// TargetRoles selects the connections whose principal holds any of the roles.
	TargetRoles TargetKind = "roles"
	// TargetBroadcast selects every live connection.
	TargetBroadcast TargetKind = "broadcast"
)

// Target selects push recipients as data, not predicates. Multiple targets on
// one Push are unioned.
type Target struct {
	Kind   TargetKind `json:"kind"`
	Values []string   `json:"values,omitempty"`
}

// ToUsers targets the live connections of the given user IDs.
func ToUsers(userIDs ...string) Target {
	return Target{Kind: TargetUsers, Values: userIDs}
}

// ToRoles targets every connection whose principal holds at least one of the
// given roles (snapshotted at handshake).
func ToRoles(roles ...string) Target {
	return Target{Kind: TargetRoles, Values: roles}
}

// Broadcast targets every live connection.
func Broadcast() Target {
	return Target{Kind: TargetBroadcast}
}
