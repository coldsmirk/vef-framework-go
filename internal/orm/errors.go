package orm

import (
	"errors"

	"github.com/coldsmirk/vef-framework-go/dbx"
	"github.com/coldsmirk/vef-framework-go/result"
)

var (
	ErrSubQuery                    = errors.New("cannot execute a subquery directly; use it as part of a parent query")
	ErrAggregateMissingArgs        = errors.New("aggregate function requires at least one argument")
	ErrDialectUnsupportedOperation = errors.New("operation not supported by current database dialect")
	ErrDialectHandlerMissing       = errors.New("no dialect handler available for requested operation")
	ErrMissingColumnOrExpression   = errors.New("order clause requires at least one column or expression")
	ErrCaseMissingWhen             = errors.New("case expression requires at least one WHEN clause")
	ErrModelMustBePointerToStruct  = errors.New("model must be a pointer to struct")
	ErrPrimaryKeyUnsupportedType   = errors.New("unsupported primary key type")

	// ErrUnsupportedDialect is returned by DialectFor when no bun dialect is
	// registered for the requested config.DBKind.
	ErrUnsupportedDialect = errors.New("orm: unsupported database dialect")

	// ErrRunOnConnectionInTx is returned when RunOnConnection is invoked on a
	// transaction-scoped DB, whose transaction already owns its connection.
	ErrRunOnConnectionInTx = errors.New("orm: RunOnConnection cannot be used within a transaction")

	// ErrNoCommitScope is returned by OnCommit when the context carries no
	// open RunInTx / RunInReadOnlyTx scope to hang the callback on — either
	// none was ever opened (including manual BeginTx transactions, which
	// hand back a Tx but no context to carry the collector) or the one that
	// was has already committed. Reporting is deliberate: running the
	// callback immediately would downgrade "after this work is durable" to
	// "right now", the opposite of what every caller asks for.
	ErrNoCommitScope = errors.New("orm: OnCommit called outside an open RunInTx scope")
)

// translateWriteError converts database-specific errors to framework errors.
// It handles duplicate key and foreign key violations with appropriate logging.
func translateWriteError(err error) error {
	if err == nil {
		return nil
	}

	if dbx.IsDuplicateKeyError(err) {
		logger.Warnf("Record already exists: %v", err)

		return result.ErrRecordAlreadyExists
	}

	if dbx.IsForeignKeyError(err) {
		logger.Warnf("Foreign key violation: %v", err)

		return result.ErrForeignKeyViolation
	}

	return err
}

// translateDeleteError converts database-specific errors to framework errors for delete operations.
// It only handles foreign key violations since duplicate key errors don't apply to deletes.
func translateDeleteError(err error) error {
	if err == nil {
		return nil
	}

	if dbx.IsForeignKeyError(err) {
		logger.Warnf("Foreign key violation: %v", err)

		return result.ErrForeignKeyViolation
	}

	return err
}
