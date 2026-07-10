package binding

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/uptrace/bun/dialect"

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
// targeting the row whose configured key matches the resolved BusinessRef. The
// trigger selects which columns are written (see approval.BindingTrigger for
// the linkage matrix); the status column is always written, the optional
// instance-id / started-at / finished-at columns only when the flow
// configures them. Skipped when the flow is not business-bound or the
// instance carries no BusinessRef. Misconfigured flows return
// ErrBindingMisconfigured; resolver failures propagate for the listener to
// classify as permanent invalid refs or retryable host faults.
func (w *Writer) WriteBack(ctx context.Context, db orm.DB, flow *approval.Flow, instance *approval.Instance, trigger approval.BindingTrigger) error {
	if flow.BindingMode != approval.BindingBusiness {
		return nil
	}

	if instance.BusinessRef == nil || strings.TrimSpace(*instance.BusinessRef) == "" {
		return nil
	}

	config, err := NormalizeConfig(flow.BindingMode, flow.BusinessBinding)
	if err != nil {
		return fmt.Errorf("%w: flow %q: %w", ErrBindingMisconfigured, flow.ID, err)
	}

	resolvedFlow := *flow
	resolvedFlow.BusinessBinding = config

	// The status column is always part of the projection; the optional
	// columns join per the trigger's row in the linkage matrix.
	setColumns := []string{config.StatusColumn}
	setValues := []any{string(instance.Status)}

	if trigger == approval.BindingTriggerStarted {
		if config.InstanceIDColumn != nil {
			setColumns = append(setColumns, *config.InstanceIDColumn)
			setValues = append(setValues, instance.ID)
		}

		if config.StartedAtColumn != nil {
			setColumns = append(setColumns, *config.StartedAtColumn)
			setValues = append(setValues, startedAt(instance))
		}
	}

	// started / resubmitted clear the finished-at column (the instance is
	// running again — or still — so a value left over from a previous round
	// must not linger); completed stamps the instance's finish time.
	if trigger == approval.BindingTriggerStarted ||
		trigger == approval.BindingTriggerCompleted ||
		trigger == approval.BindingTriggerResubmitted {
		if config.FinishedAtColumn != nil {
			setColumns = append(setColumns, *config.FinishedAtColumn)
			setValues = append(setValues, finishedAt(instance))
		}
	}

	recordKey, err := w.resolver.ResolveRecordKey(ctx, &resolvedFlow, *instance.BusinessRef)
	if err != nil {
		return fmt.Errorf("resolve business ref for flow %q: %w", flow.ID, err)
	}

	recordKey, err = validateRecordKey(config, recordKey)
	if err != nil {
		return fmt.Errorf("flow %q: %w", flow.ID, err)
	}

	matches, err := lockBindingTarget(ctx, db, config, recordKey)
	if err != nil {
		return err
	}

	if matches == 0 {
		return fmt.Errorf("%w: flow %q", ErrBindingTargetMissing, flow.ID)
	}

	if matches > 1 {
		return fmt.Errorf("%w: flow %q", ErrBindingTargetNotUnique, flow.ID)
	}

	query := db.NewUpdate().Table(config.TableName)
	for i, column := range setColumns {
		query.Set(column, setValues[i])
	}

	result, err := query.Where(recordKeyCondition(config.KeyColumns, recordKey)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("write business state (%s): %w", trigger, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read business write-back result (%s): %w", trigger, err)
	}

	if affected > 1 {
		return fmt.Errorf("%w: flow %q updated %d rows", ErrBindingTargetNotUnique, flow.ID, affected)
	}

	return nil
}

func validateRecordKey(config *approval.BusinessBindingConfig, key approval.BusinessRecordKey) (approval.BusinessRecordKey, error) {
	if len(key) != len(config.KeyColumns) {
		return nil, fmt.Errorf("%w: expected %d key values, got %d", ErrInvalidBusinessRef, len(config.KeyColumns), len(key))
	}

	validated := make(approval.BusinessRecordKey, len(key))
	for _, column := range config.KeyColumns {
		value, ok := key[column]
		if !ok || value == nil {
			return nil, fmt.Errorf("%w: missing key column %q", ErrInvalidBusinessRef, column)
		}

		converted, err := driver.DefaultParameterConverter.ConvertValue(value)
		if err != nil {
			return nil, fmt.Errorf("%w: key column %q: %w", ErrInvalidBusinessRef, column, err)
		}

		validated[column] = converted
	}

	return validated, nil
}

func lockBindingTarget(ctx context.Context, db orm.DB, config *approval.BusinessBindingConfig, key approval.BusinessRecordKey) (int, error) {
	var matches []int

	query := db.NewSelect().
		Table(config.TableName).
		SelectExpr(func(eb orm.ExprBuilder) any { return eb.Literal(1) }).
		Where(recordKeyCondition(config.KeyColumns, key)).
		Limit(2)
	if query.Dialect().Name() != dialect.SQLite {
		query.ForUpdate()
	}

	if err := query.Scan(ctx, &matches); err != nil {
		return 0, fmt.Errorf("lock business binding target: %w", err)
	}

	return len(matches), nil
}

func recordKeyCondition(columns []string, key approval.BusinessRecordKey) func(orm.ConditionBuilder) {
	return func(cb orm.ConditionBuilder) {
		for _, column := range columns {
			cb.Equals(column, key[column])
		}
	}
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
