package dbx

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/driver/pgdriver"
)

// TestContainsAny tests the containsAny helper function.
func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		substrings []string
		expected   bool
	}{
		{"MatchFirst", "duplicate key value", []string{"duplicate key", "unique"}, true},
		{"MatchSecond", "unique constraint failed", []string{"duplicate key", "unique"}, true},
		{"NoMatch", "some other error", []string{"duplicate key", "unique"}, false},
		{"EmptyMessage", "", []string{"duplicate key"}, false},
		{"EmptySubstrings", "duplicate key", []string{}, false},
		{"BothEmpty", "", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.message, tt.substrings...)
			assert.Equal(t, tt.expected, result, "Should return expected result")
		})
	}
}

// TestIsDuplicateKeyError tests IsDuplicateKeyError with various error types.
func TestIsDuplicateKeyError(t *testing.T) {
	t.Run("NilError", func(t *testing.T) {
		assert.False(t, IsDuplicateKeyError(nil), "Nil error should return false")
	})

	t.Run("PostgresUniqueViolation", func(t *testing.T) {
		pgErr := pgdriver.Error{}
		// pgdriver.Error fields are not directly settable, test via fallback
		assert.False(t, IsDuplicateKeyError(pgErr), "Empty pgdriver error should not be duplicate key")
	})

	t.Run("MySQLDupEntry", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}
		assert.True(t, IsDuplicateKeyError(mysqlErr), "MySQL error 1062 should be duplicate key")
	})

	t.Run("MySQLDupUnique", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1169, Message: "Duplicate unique"}
		assert.True(t, IsDuplicateKeyError(mysqlErr), "MySQL error 1169 should be duplicate key")
	})

	t.Run("MySQLNonDupError", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1045, Message: "Access denied"}
		assert.False(t, IsDuplicateKeyError(mysqlErr), "MySQL non-dup error should return false")
	})

	t.Run("FallbackDuplicateKey", func(t *testing.T) {
		err := errors.New("ERROR: duplicate key value violates unique constraint")
		assert.True(t, IsDuplicateKeyError(err), "Fallback duplicate key message should match")
	})

	t.Run("FallbackUniqueViolation", func(t *testing.T) {
		err := errors.New("unique violation occurred")
		assert.True(t, IsDuplicateKeyError(err), "Fallback unique violation message should match")
	})

	t.Run("FallbackDuplicateEntry", func(t *testing.T) {
		err := errors.New("Duplicate entry '1' for key 'PRIMARY'")
		assert.True(t, IsDuplicateKeyError(err), "Fallback duplicate entry message should match")
	})

	t.Run("FallbackSQLiteUniqueConstraintFailed", func(t *testing.T) {
		err := errors.New("UNIQUE constraint failed: users.email")
		assert.True(t, IsDuplicateKeyError(err), "SQLite unique constraint failed should match")
	})

	t.Run("FallbackSQLServerPrimaryKey", func(t *testing.T) {
		err := errors.New("Violation of PRIMARY KEY constraint 'PK_Users'")
		assert.True(t, IsDuplicateKeyError(err), "SQL Server primary key violation should match")
	})

	t.Run("FallbackSQLServerUniqueKey", func(t *testing.T) {
		err := errors.New("Violation of UNIQUE KEY constraint 'UQ_Users_Email'")
		assert.True(t, IsDuplicateKeyError(err), "SQL Server unique key violation should match")
	})

	t.Run("FallbackSQLServerCannotInsert", func(t *testing.T) {
		err := errors.New("Cannot insert duplicate key row in object 'dbo.Users'")
		assert.True(t, IsDuplicateKeyError(err), "SQL Server cannot insert duplicate key should match")
	})

	t.Run("FallbackOracleORA00001", func(t *testing.T) {
		err := errors.New("ORA-00001: unique constraint (SCHEMA.UK_USERS) violated")
		assert.True(t, IsDuplicateKeyError(err), "Oracle ORA-00001 should match")
	})

	t.Run("FallbackOracleUniqueConstraintViolated", func(t *testing.T) {
		err := errors.New("unique constraint SCHEMA.UK violated")
		assert.True(t, IsDuplicateKeyError(err), "Oracle unique constraint violated should match")
	})

	t.Run("UnrelatedError", func(t *testing.T) {
		err := errors.New("connection refused")
		assert.False(t, IsDuplicateKeyError(err), "Unrelated error should return false")
	})

	t.Run("WrappedMySQLError", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}
		wrapped := errors.Join(errors.New("insert failed"), mysqlErr)
		assert.True(t, IsDuplicateKeyError(wrapped), "Wrapped MySQL error should be detected")
	})
}

// TestIsForeignKeyError tests IsForeignKeyError with various error types.
func TestIsForeignKeyError(t *testing.T) {
	t.Run("NilError", func(t *testing.T) {
		assert.False(t, IsForeignKeyError(nil), "Nil error should return false")
	})

	t.Run("PostgresForeignKeyViolation", func(t *testing.T) {
		pgErr := pgdriver.Error{}
		assert.False(t, IsForeignKeyError(pgErr), "Empty pgdriver error should not be foreign key")
	})

	t.Run("MySQLRowIsReferenced", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1451, Message: "Cannot delete or update a parent row"}
		assert.True(t, IsForeignKeyError(mysqlErr), "MySQL error 1451 should be foreign key")
	})

	t.Run("MySQLNoReferencedRow", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1452, Message: "Cannot add or update a child row"}
		assert.True(t, IsForeignKeyError(mysqlErr), "MySQL error 1452 should be foreign key")
	})

	t.Run("MySQLNonFKError", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1045, Message: "Access denied"}
		assert.False(t, IsForeignKeyError(mysqlErr), "MySQL non-FK error should return false")
	})

	t.Run("FallbackPostgresViolates", func(t *testing.T) {
		err := errors.New("violates foreign key constraint \"fk_orders_user_id\"")
		assert.True(t, IsForeignKeyError(err), "Postgres foreign key violation message should match")
	})

	t.Run("FallbackPostgresFKViolation", func(t *testing.T) {
		err := errors.New("foreign key violation on table orders")
		assert.True(t, IsForeignKeyError(err), "Postgres foreign key violation should match")
	})

	t.Run("FallbackMySQLConstraintFails", func(t *testing.T) {
		err := errors.New("a foreign key constraint fails (`db`.`orders`, CONSTRAINT `fk_user`)")
		assert.True(t, IsForeignKeyError(err), "MySQL foreign key constraint fails should match")
	})

	t.Run("FallbackMySQLCannotAddChild", func(t *testing.T) {
		err := errors.New("Cannot add or update a child row: a foreign key constraint fails")
		assert.True(t, IsForeignKeyError(err), "MySQL cannot add child row should match")
	})

	t.Run("FallbackMySQLCannotDeleteParent", func(t *testing.T) {
		err := errors.New("Cannot delete or update a parent row: a foreign key constraint fails")
		assert.True(t, IsForeignKeyError(err), "MySQL cannot delete parent row should match")
	})

	t.Run("FallbackSQLiteForeignKeyFailed", func(t *testing.T) {
		err := errors.New("FOREIGN KEY constraint failed")
		assert.True(t, IsForeignKeyError(err), "SQLite foreign key constraint failed should match")
	})

	t.Run("FallbackSQLiteConstraintForeignKey", func(t *testing.T) {
		err := errors.New("SQLITE_CONSTRAINT_FOREIGNKEY: foreign key mismatch")
		assert.True(t, IsForeignKeyError(err), "SQLite constraint foreign key should match")
	})

	t.Run("FallbackSQLiteForeignKeyMismatch", func(t *testing.T) {
		err := errors.New("foreign key mismatch - table1 referencing table2")
		assert.True(t, IsForeignKeyError(err), "SQLite foreign key mismatch should match")
	})

	t.Run("FallbackSQLServerConflicted", func(t *testing.T) {
		err := errors.New("The DELETE statement conflicted with the FOREIGN KEY constraint \"FK_Orders_Users\"")
		assert.True(t, IsForeignKeyError(err), "SQL Server conflicted should match")
	})

	t.Run("FallbackSQLServerStatementConflicted", func(t *testing.T) {
		err := errors.New("The INSERT statement conflicted with the FOREIGN KEY constraint")
		assert.True(t, IsForeignKeyError(err), "SQL Server statement conflicted should match")
	})

	t.Run("FallbackOracleORA02291", func(t *testing.T) {
		err := errors.New("ORA-02291: integrity constraint violated - parent key not found")
		assert.True(t, IsForeignKeyError(err), "Oracle ORA-02291 should match")
	})

	t.Run("FallbackOracleORA02292", func(t *testing.T) {
		err := errors.New("ORA-02292: integrity constraint violated - child record found")
		assert.True(t, IsForeignKeyError(err), "Oracle ORA-02292 should match")
	})

	t.Run("FallbackOracleIntegrityParentKey", func(t *testing.T) {
		err := errors.New("integrity constraint violated: parent key not found")
		assert.True(t, IsForeignKeyError(err), "Oracle integrity constraint parent key should match")
	})

	t.Run("FallbackOracleIntegrityChildRecord", func(t *testing.T) {
		err := errors.New("integrity constraint violated: child record found")
		assert.True(t, IsForeignKeyError(err), "Oracle integrity constraint child record should match")
	})

	t.Run("UnrelatedError", func(t *testing.T) {
		err := errors.New("connection refused")
		assert.False(t, IsForeignKeyError(err), "Unrelated error should return false")
	})

	t.Run("WrappedMySQLError", func(t *testing.T) {
		mysqlErr := &mysql.MySQLError{Number: 1451, Message: "Cannot delete parent row"}
		wrapped := errors.Join(errors.New("delete failed"), mysqlErr)
		assert.True(t, IsForeignKeyError(wrapped), "Wrapped MySQL error should be detected")
	})
}
