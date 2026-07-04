package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Writer performs the engine-owned write-back of approval outcomes onto the
// host's business table when Flow.BindingMode == BindingBusiness. It is not
// an extension point: hosts influence which row it targets through
// approval.BusinessRefResolver and extend around it with lifecycle hooks or
// event subscriptions, but the write-back itself belongs to the engine.
type Writer struct {
	resolver approval.BusinessRefResolver
}

// NewWriter constructs the Writer around the ref resolver.
func NewWriter(resolver approval.BusinessRefResolver) *Writer {
	return &Writer{resolver: resolver}
}

// WriteBackStatus writes finalStatus into the business table's status column,
// targeting the row whose pk column matches the resolved BusinessRef.
// Skipped when the flow is not business-bound or the instance carries no
// BusinessRef. Misconfigured flows return ErrBindingMisconfigured; resolver
// failures propagate as transient errors so the outbox can retry.
func (w *Writer) WriteBackStatus(ctx context.Context, db orm.DB, flow *approval.Flow, instance *approval.Instance, finalStatus approval.InstanceStatus) error {
	if flow.BindingMode != approval.BindingBusiness {
		return nil
	}

	if instance.BusinessRef == nil || strings.TrimSpace(*instance.BusinessRef) == "" {
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

	// Defense-in-depth: even though CreateFlow/UpdateFlow already enforce
	// the same regex, reject any identifier that does not match here so
	// rows persisted before the validator existed (or smuggled in via a
	// direct DB write) cannot turn fmt.Sprintf into a SQL injection vector.
	for _, ident := range []string{table, pkField, statusField} {
		if err := approval.ValidateBusinessIdentifier(ident); err != nil {
			return fmt.Errorf("%w: flow %q identifier %q rejected: %w", ErrBindingMisconfigured, flow.ID, ident, err)
		}
	}

	recordID, err := w.resolver.ResolveRecordID(ctx, flow, *instance.BusinessRef)
	if err != nil {
		return fmt.Errorf("resolve business ref for flow %q: %w", flow.ID, err)
	}

	if strings.TrimSpace(recordID) == "" {
		return fmt.Errorf("%w: flow %q resolver produced an empty record id", ErrBindingMisconfigured, flow.ID)
	}

	// A single-column write-back: only the status column is updated. The
	// record id is bound as a parameter; only whitelisted identifiers are
	// interpolated.
	sql := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", table, statusField, pkField)
	if _, err := db.NewRaw(sql, string(finalStatus), recordID).Exec(ctx); err != nil {
		return fmt.Errorf("write business status: %w", err)
	}

	return nil
}
