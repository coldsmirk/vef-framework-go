package binding

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Writer applies one durable projection snapshot to its host business row. It
// always writes the complete configured state, so retries and out-of-order
// lifecycle notifications cannot combine columns from different revisions.
type Writer struct{}

// NewWriter constructs the engine-owned Writer.
func NewWriter() *Writer { return new(Writer) }

// Write applies projection.DesiredRevision to the bound business row. When a
// previous owner has already been applied, the instance-ID column is checked
// and included in the UPDATE predicate as a compare-and-set fence.
func (*Writer) Write(ctx context.Context, db orm.DB, projection *approval.BusinessProjection) error {
	if projection == nil || projection.Binding == nil {
		return fmt.Errorf("%w: projection has no binding", ErrProjectionStateInvalid)
	}

	config, err := NormalizeConfig(approval.BindingBusiness, projection.Binding)
	if err != nil {
		return fmt.Errorf("%w: projection %q: %w", ErrBindingMisconfigured, projection.ID, err)
	}

	recordKey, err := decodeRecordKey(projection.RecordKey)
	if err != nil {
		return fmt.Errorf("projection %q: %w", projection.ID, err)
	}

	recordKey, err = validateRecordKey(config, recordKey)
	if err != nil {
		return fmt.Errorf("projection %q: %w", projection.ID, err)
	}

	matches, err := lockBindingTarget(ctx, db, config, recordKey)
	if err != nil {
		return err
	}

	if matches == 0 {
		return fmt.Errorf("%w: projection %q", ErrBindingTargetMissing, projection.ID)
	}

	if matches > 1 {
		return fmt.Errorf("%w: projection %q", ErrBindingTargetNotUnique, projection.ID)
	}

	if projection.AppliedRevision > 0 {
		if projection.AppliedOwnerInstanceID == nil {
			return fmt.Errorf("%w: projection %q has an applied revision without an owner", ErrProjectionStateInvalid, projection.ID)
		}

		ownerMatches, err := bindingTargetOwnerMatches(ctx, db, config, recordKey, *projection.AppliedOwnerInstanceID)
		if err != nil {
			return err
		}

		if !ownerMatches {
			return fmt.Errorf("%w: projection %q expected owner %q", ErrBindingOwnershipConflict,
				projection.ID, *projection.AppliedOwnerInstanceID)
		}
	}

	status, err := projectedStatus(config, projection.DesiredStatus)
	if err != nil {
		return err
	}

	setColumns := []string{config.StatusColumn, *config.InstanceIDColumn}
	setValues := []any{status, projection.OwnerInstanceID}

	if config.StartedAtColumn != nil {
		setColumns = append(setColumns, *config.StartedAtColumn)
		setValues = append(setValues, projection.DesiredStartedAt)
	}

	if config.FinishedAtColumn != nil {
		var finishedAt any
		if projection.DesiredStatus.IsFinal() {
			if projection.DesiredFinishedAt == nil {
				return fmt.Errorf("%w: final projection %q has no finish time", ErrProjectionStateInvalid, projection.ID)
			}

			finishedAt = *projection.DesiredFinishedAt
		}

		setColumns = append(setColumns, *config.FinishedAtColumn)
		setValues = append(setValues, finishedAt)
	}

	query := db.NewUpdate().Table(config.TableName)
	for i, column := range setColumns {
		query.Set(column, setValues[i])
	}

	result, err := query.Where(func(cb orm.ConditionBuilder) {
		recordKeyCondition(config.KeyColumns, recordKey)(cb)

		if projection.AppliedRevision > 0 {
			cb.Equals(*config.InstanceIDColumn, *projection.AppliedOwnerInstanceID)
		}
	}).Exec(ctx)
	if err != nil {
		return fmt.Errorf("write business projection %q revision %d: %w", projection.ID, projection.DesiredRevision, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read business projection %q result: %w", projection.ID, err)
	}

	if affected > 1 {
		return fmt.Errorf("%w: projection %q updated %d rows", ErrBindingTargetNotUnique, projection.ID, affected)
	}

	return nil
}

func lockBindingTarget(ctx context.Context, db orm.DB, config *approval.BusinessBindingConfig, key approval.BusinessRecordKey) (int, error) {
	var matches []int

	query := db.NewSelect().
		Table(config.TableName).
		SelectExpr(func(eb orm.ExprBuilder) any { return eb.Literal(1) }).
		Where(recordKeyCondition(config.KeyColumns, key)).
		Limit(2)
	query.ForUpdate()

	if err := query.Scan(ctx, &matches); err != nil {
		return 0, fmt.Errorf("lock business binding target: %w", err)
	}

	return len(matches), nil
}

func bindingTargetOwnerMatches(
	ctx context.Context,
	db orm.DB,
	config *approval.BusinessBindingConfig,
	key approval.BusinessRecordKey,
	expectedOwner string,
) (bool, error) {
	matches, err := db.NewSelect().
		Table(config.TableName).
		Where(func(cb orm.ConditionBuilder) {
			recordKeyCondition(config.KeyColumns, key)(cb)
			cb.Equals(*config.InstanceIDColumn, expectedOwner)
		}).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("verify business binding owner: %w", err)
	}

	return matches, nil
}

func recordKeyCondition(columns []string, key approval.BusinessRecordKey) func(orm.ConditionBuilder) {
	return func(cb orm.ConditionBuilder) {
		for _, column := range columns {
			cb.Equals(column, key[column])
		}
	}
}

func projectedStatus(config *approval.BusinessBindingConfig, status approval.InstanceStatus) (string, error) {
	if mapped, ok := config.StatusMapping[status]; ok {
		if mapped == "" {
			return "", fmt.Errorf("%w: status %q maps to an empty value", ErrBindingMisconfigured, status)
		}

		return mapped, nil
	}

	if !isProjectableStatus(status) {
		return "", fmt.Errorf("%w: unknown instance status %q", ErrProjectionStateInvalid, status)
	}

	return status.String(), nil
}
