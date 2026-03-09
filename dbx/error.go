package dbx

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun/driver/pgdriver"
)

// PostgreSQL error codes.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// MySQL error numbers.
const (
	mysqlDupEntry        = 1062
	mysqlDupUnique       = 1169
	mysqlRowIsReferenced = 1451
	mysqlNoReferencedRow = 1452
)

// containsAny checks if the message contains any of the given substrings.
func containsAny(message string, substrings ...string) bool {
	for _, s := range substrings {
		if strings.Contains(message, s) {
			return true
		}
	}

	return false
}

// IsDuplicateKeyError checks if the error is a duplicate key error.
// It first checks database-specific error codes for reliability,
// then falls back to message matching for compatibility.
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	// PostgreSQL: code 23505 (unique_violation)
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == pgUniqueViolation
	}

	// MySQL: 1062 (ER_DUP_ENTRY) or 1169 (ER_DUP_UNIQUE)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlDupEntry || mysqlErr.Number == mysqlDupUnique
	}

	// Fallback: message matching for SQLite, SQL Server, Oracle
	message := strings.ToLower(err.Error())

	return containsAny(message,
		// PostgreSQL
		"duplicate key", "unique violation",
		// MySQL
		"duplicate entry",
		// SQLite
		"unique constraint failed",
		// SQL Server
		"violation of primary key constraint",
		"violation of unique key constraint",
		"cannot insert duplicate key",
		// Oracle (ORA-00001)
		"ora-00001",
	) || (strings.Contains(message, "unique constraint") && strings.Contains(message, "violated"))
}

// IsForeignKeyError checks if the error is a foreign key constraint error.
// It first checks database-specific error codes for reliability,
// then falls back to message matching for compatibility.
func IsForeignKeyError(err error) bool {
	if err == nil {
		return false
	}

	// PostgreSQL: code 23503 (foreign_key_violation)
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == pgForeignKeyViolation
	}

	// MySQL: 1451 (ER_ROW_IS_REFERENCED) or 1452 (ER_NO_REFERENCED_ROW)
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlRowIsReferenced || mysqlErr.Number == mysqlNoReferencedRow
	}

	// Fallback: message matching for SQLite, SQL Server, Oracle
	message := strings.ToLower(err.Error())

	// Common foreign key error patterns.
	hasFKPattern := containsAny(message,
		// PostgreSQL
		"violates foreign key constraint", "foreign key violation",
		// MySQL
		"a foreign key constraint fails",
		"cannot add or update a child row",
		"cannot delete or update a parent row",
		// SQLite
		"foreign key constraint failed",
		"sqlite_constraint_foreignkey",
		"foreign key mismatch",
		// SQL Server
		"conflicted with the foreign key constraint",
		"statement conflicted with the foreign key",
		// Oracle (ORA-02291, ORA-02292)
		"ora-02291", "ora-02292",
	)

	// Oracle: integrity constraint violated with parent/child key.
	hasOracleIntegrityPattern := strings.Contains(message, "integrity constraint") &&
		strings.Contains(message, "violated") &&
		(strings.Contains(message, "parent key not found") || strings.Contains(message, "child record found"))

	return hasFKPattern || hasOracleIntegrityPattern
}
