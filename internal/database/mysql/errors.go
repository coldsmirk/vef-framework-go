package mysql

import "errors"

var ErrMySQLDatabaseRequired = errors.New("database name is required for MySQL")
