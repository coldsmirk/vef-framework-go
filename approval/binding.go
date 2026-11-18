package approval

import (
	"context"
	"regexp"
	"strings"
)

// BusinessBindingConfig describes the business row targeted by a flow and the
// columns that receive approval lifecycle state. KeyColumns must exactly match
// a non-null primary or unique key on TableName.
type BusinessBindingConfig struct {
	TableName    string   `json:"tableName"`
	KeyColumns   []string `json:"keyColumns"`
	StatusColumn string   `json:"statusColumn"`
	// InstanceIDColumn is mandatory for business bindings. The projector uses
	// it as a compare-and-set fence so a stale instance cannot overwrite the
	// state owned by a newer approval round.
	InstanceIDColumn *string `json:"instanceIdColumn,omitempty"`
	StartedAtColumn  *string `json:"startedAtColumn,omitempty"`
	FinishedAtColumn *string `json:"finishedAtColumn,omitempty"`
	// StatusMapping translates approval instance statuses into host business
	// status values. Missing entries fall back to the InstanceStatus string.
	StatusMapping map[InstanceStatus]string `json:"statusMapping,omitempty"`
}

// BusinessRecordKey maps every configured key column to the value resolved
// from an instance's opaque BusinessRef.
type BusinessRecordKey map[string]any

// BindingProjectionStatus is the durable convergence state of one business
// record projection.
type BindingProjectionStatus string

const (
	BindingProjectionPending    BindingProjectionStatus = "pending"
	BindingProjectionProcessing BindingProjectionStatus = "processing"
	BindingProjectionApplied    BindingProjectionStatus = "applied"
	BindingProjectionFailed     BindingProjectionStatus = "failed"
)

// BusinessRefResolver turns the opaque Instance.BusinessRef into the record
// key the engine-owned write-back matches against. The returned key must name
// exactly the flow's configured BusinessBinding.KeyColumns. The default
// resolver treats a single-column ref verbatim and decodes a multi-column ref
// from a JSON object. Hosts with another ref shape register a custom resolver.
//
// Hosts register an implementation via vef.SupplyBusinessRefResolver.
type BusinessRefResolver interface {
	// ResolveRecordKey extracts the configured business record key from the
	// instance's BusinessRef, which is never blank here. The instance carries
	// the tenant it runs in and the applicant, so a resolver can scope the key
	// to that tenant — a shared flow's binding serves every tenant, and its
	// Flow.TenantID is the owning tenant, not the instance's — and refuse a
	// reference the applicant may not bind; the ref may come straight from the
	// client. Returning an error fails the start, ErrInvalidBusinessRef for a
	// malformed reference.
	ResolveRecordKey(ctx context.Context, flow *Flow, instance *Instance) (BusinessRecordKey, error)
}

// BindingTrigger identifies the lifecycle moment associated with a failed
// business projection. Projection correctness does not depend on triggers:
// every write applies the latest full desired state. The trigger remains part
// of InstanceBindingFailedEvent so operators can identify the action that
// produced that desired state.
type BindingTrigger string

const (
	BindingTriggerStarted     BindingTrigger = "started"
	BindingTriggerCompleted   BindingTrigger = "completed"
	BindingTriggerReturned    BindingTrigger = "returned"
	BindingTriggerWithdrawn   BindingTrigger = "withdrawn"
	BindingTriggerResubmitted BindingTrigger = "resubmitted"
)

// businessIdentifierPattern restricts business binding table and column names
// to simple SQL identifiers. PostgreSQL allows up to 63 characters; we follow
// the same bound across supported databases.
var businessIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// ValidateBusinessIdentifier reports whether id is a safe SQL identifier
// for use as a dynamic table or column name in business binding queries,
// returning ErrInvalidBusinessIdentifier when it is not. Empty /
// whitespace-only strings pass — the caller decides whether absence is
// itself an error (see Flow validation paths for the policy).
func ValidateBusinessIdentifier(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}

	if !businessIdentifierPattern.MatchString(trimmed) {
		return ErrInvalidBusinessIdentifier
	}

	return nil
}
