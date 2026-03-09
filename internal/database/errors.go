package database

import (
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
)

var (
	ErrUnsupportedDBKind  = errors.New("unsupported database type")
	errPingFailed         = errors.New("database ping failed")
	errVersionQueryFailed = errors.New("database version query failed")
)

type DatabaseError struct {
	Kind    config.DBKind
	Op      string
	Err     error
	Context map[string]any
}

func (e *DatabaseError) Error() string {
	if len(e.Context) > 0 {
		return fmt.Sprintf("database error [%s] during %s: %v (context: %+v)", e.Kind, e.Op, e.Err, e.Context)
	}

	return fmt.Sprintf("database error [%s] during %s: %v", e.Kind, e.Op, e.Err)
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}

func newDatabaseError(dbKind config.DBKind, operation string, err error, context map[string]any) *DatabaseError {
	return &DatabaseError{
		Kind:    dbKind,
		Op:      operation,
		Err:     err,
		Context: context,
	}
}

func wrapPingError(dbKind config.DBKind, err error) error {
	return newDatabaseError(dbKind, "ping", fmt.Errorf("%w: %w", errPingFailed, err), nil)
}

func wrapVersionQueryError(dbKind config.DBKind, err error) error {
	return newDatabaseError(dbKind, "version_query", fmt.Errorf("%w: %w", errVersionQueryFailed, err), nil)
}

func newUnsupportedDBKindError(dbKind config.DBKind) error {
	return newDatabaseError(dbKind, "validation", ErrUnsupportedDBKind, map[string]any{
		"supported_types": []config.DBKind{config.SQLite, config.Postgres, config.MySQL},
	})
}
