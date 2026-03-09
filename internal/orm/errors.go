package orm

import (
	"errors"

	"github.com/coldsmirk/vef-framework-go/dbx"
	"github.com/coldsmirk/vef-framework-go/result"
)

var (
	ErrSubQuery                     = errors.New("cannot execute a subquery directly; use it as part of a parent query")
	ErrAggregateMissingArgs         = errors.New("aggregate function requires at least one argument")
	ErrDialectUnsupportedOperation  = errors.New("operation not supported by current database dialect")
	ErrAggregateUnsupportedFunction = errors.New("aggregate function not supported by current database dialect")
	ErrDialectHandlerMissing        = errors.New("no dialect handler available for requested operation")
	ErrMissingColumnOrExpression    = errors.New("order clause requires at least one column or expression")
	ErrModelMustBePointerToStruct   = errors.New("model must be a pointer to struct")
	ErrPrimaryKeyUnsupportedType    = errors.New("unsupported primary key type")
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
