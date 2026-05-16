package approval

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/coldsmirk/vef-framework-go/orm"
)

// BusinessBindingHook bridges the approval engine with the host application's
// business tables when Flow.BindingMode is BindingBusiness. It is narrowly
// scoped to the "approval row ↔ business row" plumbing; broader lifecycle
// extension goes through InstanceLifecycleHook instead.
//
// Two lifecycle moments matter:
//
//   - OnInstanceCreated runs inside the start_instance transaction so the
//     host can resolve / create the business row and return the primary
//     key that the engine stores in Instance.BusinessRecordID. Returning
//     an error rolls back the entire instance creation.
//
//   - WriteBackStatus runs asynchronously via the binding Listener (which
//     subscribes to InstanceCompletedEvent) so the host can stamp the
//     final approval decision onto its own business table. A non-nil
//     error does NOT roll back the approval — the workflow has already
//     decided. Instead the listener publishes InstanceBindingFailedEvent
//     so the host can retry (saga / outbox compensation).
//
// Hosts override the default implementation by binding their own
// BusinessBindingHook into the FX container, typically through
// vef.SupplyBusinessBindingHook.
type BusinessBindingHook interface {
	// OnInstanceCreated returns the business primary key (BusinessRecordID)
	// to persist on the instance. Returning empty string indicates the host
	// has nothing to bind (engine stores nil).
	OnInstanceCreated(ctx context.Context, db orm.DB, flow *Flow, instance *Instance) (businessRecordID string, err error)
	// WriteBackStatus writes the final approval status back to the
	// business table. Called asynchronously from the binding Listener
	// after InstanceCompletedEvent fires. Implementations should be
	// idempotent — the listener may retry through the outbox.
	WriteBackStatus(ctx context.Context, db orm.DB, flow *Flow, instance *Instance, finalStatus InstanceStatus) error
}

// businessIdentifierPattern restricts business_table / business_pk_field /
// business_status_field / business_title_field to safe SQL identifiers.
// The default BusinessBindingHook interpolates these values into a raw
// `UPDATE %s SET %s = ? WHERE %s = ?` template, so anything outside this
// whitelist (spaces, quotes, semicolons, brackets, sub-selects) could open
// a SQL injection vector. PostgreSQL allows up to 63 characters; we follow
// the same bound.
var businessIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// ErrInvalidBusinessIdentifier is returned by ValidateBusinessIdentifier
// for values that do not match a SQL-safe identifier pattern. Hosts that
// implement BusinessBindingHook should bubble this up (or wrap it) so
// admin-side flow CRUD surfaces a meaningful error to operators.
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
