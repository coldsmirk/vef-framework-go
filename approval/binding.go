package approval

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// BusinessRefProvider supplies the business reference for a newly created
// instance when Flow.BindingMode is BindingBusiness. It exists for the one
// binding step the engine cannot do generically — resolving or allocating
// the business row — and runs inside the start_instance transaction, so
// returning an error rolls back the entire instance creation.
//
// The write-back of approval outcomes onto the business table is NOT part
// of this contract: it is engine-owned, configuration-driven, and not
// overridable. Hosts needing deeper integration register an
// InstanceLifecycleHook or subscribe to instance events — extensions add
// what the engine does not do; they do not replace what it does.
//
// Hosts register an implementation via vef.SupplyBusinessRefProvider.
type BusinessRefProvider interface {
	// OnInstanceCreated resolves or creates the business row bound to the
	// instance and returns its reference (see Instance.BusinessRef for the
	// shape contract). Returning an empty string indicates the host has
	// nothing to bind — e.g. the caller already supplied businessRef in the
	// start parameters. Runs inside the start_instance transaction; an
	// error rolls back instance creation.
	OnInstanceCreated(ctx context.Context, db orm.DB, flow *Flow, instance *Instance) (businessRef string, err error)
}

// BusinessRefResolver turns the opaque Instance.BusinessRef into the record
// identifier the engine-owned write-back matches against
// Flow.BusinessPkField (`WHERE pk_field = ?`). The default resolver returns
// the ref verbatim — correct when the ref is the business primary key
// itself. Hosts that encode composite refs (e.g. JSON) register a resolver
// that extracts the key value, which keeps the built-in write-back
// applicable to any ref shape.
//
// Hosts register an implementation via vef.SupplyBusinessRefResolver.
type BusinessRefResolver interface {
	// ResolveRecordID extracts the business primary-key value from
	// businessRef. Returning an error fails the write-back for this
	// instance (surfaced through InstanceBindingFailedEvent and retried).
	ResolveRecordID(ctx context.Context, flow *Flow, businessRef string) (string, error)
}

// businessIdentifierPattern restricts business_table / business_pk_field /
// business_status_field to safe SQL identifiers.
// The engine-owned write-back interpolates these values into a raw
// `UPDATE %s SET %s = ? WHERE %s = ?` template, so anything outside this
// whitelist (spaces, quotes, semicolons, brackets, sub-selects) could open
// a SQL injection vector. PostgreSQL allows up to 63 characters; we follow
// the same bound.
var businessIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// ErrInvalidBusinessIdentifier is returned by ValidateBusinessIdentifier
// for values that do not match a SQL-safe identifier pattern. Flow CRUD
// validation surfaces it to operators; the write-back re-checks the same
// rule as defense-in-depth.
var ErrInvalidBusinessIdentifier = errors.New("approval: invalid business identifier (must match ^[A-Za-z_][A-Za-z0-9_]{0,62}$)")

// ValidateBusinessIdentifier reports whether id is a safe SQL identifier
// for use as a table or column name in business binding interpolation.
// Empty / whitespace-only strings pass — the caller decides whether absence
// is itself an error (see Flow validation paths for the policy).
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
