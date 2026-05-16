// Package binding provides the default BusinessBindingHook implementation
// that wires the approval engine into the host application's business tables
// declared via Flow.BusinessTable / BusinessPkField / BusinessStatusField.
package binding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ErrBindingMisconfigured signals that a Flow with BindingMode=business is
// missing one of the required columns (business_table / business_pk_field /
// business_status_field). The engine surfaces this as the start failure or
// completion failure event depending on which hook returns it.
var ErrBindingMisconfigured = errors.New("approval: business binding misconfigured")

// DefaultHook writes the final InstanceStatus into the host's business table
// when Flow.BindingMode == BindingBusiness. It is intentionally minimal:
// hosts that need transactional outbox writes, multi-column updates, or
// cross-service calls should supply their own BusinessBindingHook via
// vef.SupplyBusinessBindingHook(...).
type DefaultHook struct{}

// NewDefaultHook constructs a DefaultHook.
func NewDefaultHook() approval.BusinessBindingHook { return new(DefaultHook) }

// OnInstanceCreated is a no-op for the default hook — the framework cannot
// know how to create a business row generically. Hosts override this method
// to allocate the business primary key. Returning empty string tells the
// engine to store NULL in Instance.BusinessRecordID.
func (*DefaultHook) OnInstanceCreated(context.Context, orm.DB, *approval.Flow, *approval.Instance) (string, error) {
	return "", nil
}

// OnInstanceCompleted writes finalStatus to the business table. Skipped when
// BindingMode != BindingBusiness or when BusinessRecordID is empty (the host
// never produced one). Misconfigured flows return ErrBindingMisconfigured.
func (*DefaultHook) OnInstanceCompleted(ctx context.Context, db orm.DB, flow *approval.Flow, instance *approval.Instance, finalStatus approval.InstanceStatus) error {
	if flow.BindingMode != approval.BindingBusiness {
		return nil
	}

	if instance.BusinessRecordID == nil || strings.TrimSpace(*instance.BusinessRecordID) == "" {
		return nil
	}

	if flow.BusinessTable == nil || flow.BusinessPkField == nil || flow.BusinessStatusField == nil {
		return fmt.Errorf("%w: flow %q missing table/pk/status configuration", ErrBindingMisconfigured, flow.ID)
	}

	table := strings.TrimSpace(*flow.BusinessTable)
	pkField := strings.TrimSpace(*flow.BusinessPkField)
	statusField := strings.TrimSpace(*flow.BusinessStatusField)

	if table == "" || pkField == "" || statusField == "" {
		return fmt.Errorf("%w: flow %q has blank table/pk/status", ErrBindingMisconfigured, flow.ID)
	}

	// Identifiers come from flow definition (admin-controlled DDL metadata),
	// not user input, but quoting via NewRaw with positional args still
	// shields any future misuse from injection.
	sql := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", table, statusField, pkField)
	if _, err := db.NewRaw(sql, string(finalStatus), *instance.BusinessRecordID).Exec(ctx); err != nil {
		return fmt.Errorf("write business status: %w", err)
	}

	return nil
}
