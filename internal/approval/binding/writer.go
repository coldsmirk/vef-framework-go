package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Writer performs the engine-owned write-back of instance state onto the
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

// WriteBack projects the instance's current state onto the business table,
// targeting the row whose pk column matches the resolved BusinessRef. The
// trigger selects which columns are written (see approval.BindingTrigger for
// the linkage matrix); the status column is always written, the optional
// instance-id / started-at / finished-at columns only when the flow
// configures them. Skipped when the flow is not business-bound or the
// instance carries no BusinessRef. Misconfigured flows return
// ErrBindingMisconfigured; resolver failures propagate as transient errors
// so the outbox can retry.
func (w *Writer) WriteBack(ctx context.Context, db orm.DB, flow *approval.Flow, instance *approval.Instance, trigger approval.BindingTrigger) error {
	if flow.BindingMode != approval.BindingBusiness {
		return nil
	}

	if instance.BusinessRef == nil || strings.TrimSpace(*instance.BusinessRef) == "" {
		return nil
	}

	if flow.BusinessTable == nil || flow.BusinessPKField == nil || flow.BusinessStatusField == nil {
		return fmt.Errorf("%w: flow %q missing table/pk/status configuration", ErrBindingMisconfigured, flow.ID)
	}

	table := strings.TrimSpace(*flow.BusinessTable)
	pkField := strings.TrimSpace(*flow.BusinessPKField)
	statusField := strings.TrimSpace(*flow.BusinessStatusField)

	if table == "" || pkField == "" || statusField == "" {
		return fmt.Errorf("%w: flow %q has blank table/pk/status", ErrBindingMisconfigured, flow.ID)
	}

	// The status column is always part of the projection; the optional
	// columns join per the trigger's row in the linkage matrix.
	setColumns := []string{statusField}
	setValues := []any{string(instance.Status)}

	if trigger == approval.BindingTriggerStarted {
		if col, ok := optionalColumn(flow.BusinessInstanceIDField); ok {
			setColumns = append(setColumns, col)
			setValues = append(setValues, instance.ID)
		}

		if col, ok := optionalColumn(flow.BusinessStartedAtField); ok {
			setColumns = append(setColumns, col)
			setValues = append(setValues, startedAt(instance))
		}
	}

	// started / resubmitted clear the finished-at column (the instance is
	// running again — or still — so a value left over from a previous round
	// must not linger); completed stamps the instance's finish time.
	if trigger == approval.BindingTriggerStarted ||
		trigger == approval.BindingTriggerCompleted ||
		trigger == approval.BindingTriggerResubmitted {
		if col, ok := optionalColumn(flow.BusinessFinishedAtField); ok {
			setColumns = append(setColumns, col)
			setValues = append(setValues, finishedAt(instance))
		}
	}

	// Defense-in-depth: even though CreateFlow/UpdateFlow already enforce
	// the same regex, reject any identifier that does not match here so
	// rows persisted before the validator existed (or smuggled in via a
	// direct DB write) cannot turn fmt.Sprintf into a SQL injection vector.
	for _, ident := range append([]string{table, pkField}, setColumns...) {
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

	// Values are bound as parameters; only whitelisted identifiers are
	// interpolated.
	assignments := make([]string, len(setColumns))
	for i, col := range setColumns {
		assignments[i] = col + " = ?"
	}

	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", table, strings.Join(assignments, ", "), pkField)
	if _, err := db.NewRaw(sql, append(setValues, recordID)...).Exec(ctx); err != nil {
		return fmt.Errorf("write business state (%s): %w", trigger, err)
	}

	return nil
}

// optionalColumn unwraps an optional binding column, reporting whether it is
// configured (non-nil and non-blank).
func optionalColumn(field *string) (string, bool) {
	if field == nil {
		return "", false
	}

	trimmed := strings.TrimSpace(*field)

	return trimmed, trimmed != ""
}

// startedAt is the value projected into the started-at column: the instance
// creation time, falling back to now for instances built outside the audited
// insert path (defensive; the start transaction always stamps CreatedAt).
func startedAt(instance *approval.Instance) timex.DateTime {
	if instance.CreatedAt.IsZero() {
		return timex.Now()
	}

	return instance.CreatedAt
}

// finishedAt is the value projected into the finished-at column. A nil
// Instance.FinishedAt writes SQL NULL — exactly what started / resubmitted
// need to clear a leftover value from a previous round.
func finishedAt(instance *approval.Instance) any {
	if instance.FinishedAt == nil {
		return nil
	}

	return *instance.FinishedAt
}
