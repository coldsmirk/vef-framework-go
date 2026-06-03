package database

import (
	"errors"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/config"
)

var (
	errUnsupportedDBKind  = errors.New("unsupported database type")
	errVersionQueryFailed = errors.New("database version query failed")
)

func wrapVersionQueryError(dbKind config.DBKind, err error) error {
	return fmt.Errorf("%w [%s]: %w", errVersionQueryFailed, dbKind, err)
}

func newUnsupportedDBKindError(dbKind config.DBKind) error {
	return fmt.Errorf("%w: %s", errUnsupportedDBKind, dbKind)
}
