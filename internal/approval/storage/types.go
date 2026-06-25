package storage

import (
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
)

// sqlTypes holds the dialect-specific SQL fragments the DDL generator needs.
// Keeping them in one struct per dialect mirrors how the migration scripts
// differ only in column types / defaults, and makes adding a dialect a matter
// of one more typesFor case. Length/precision-parameterized types (VARCHAR(n),
// DECIMAL(38,s)) are rendered by varcharType / decimalType rather than stored
// here, since they depend on the field's configuration.
type sqlTypes struct {
	// pk is the column type for the table's own primary key (id).
	pk string
	// id is the column type for foreign-key-shaped columns (instance_id).
	// Mirrors the VARCHAR(32) id columns used throughout the apv_* schema.
	id string
	// text backs ColumnText, a length-less ColumnString, and the kind-based
	// fallback for any non-number field.
	text string
	// number is the kind-based fallback for the number field (legacy path,
	// used when a field carries no explicit ColumnType).
	number string
	// integer backs ColumnInteger (whole numbers).
	integer string
	// boolean backs ColumnBoolean.
	boolean string
	// date backs ColumnDate (calendar date, no time component).
	date string
	// datetime backs ColumnDatetime (date + time).
	datetime string
	// json backs ColumnJSON (arrays / composite values).
	json string
	// timestamp is the type of the built-in created_at column.
	timestamp string
	// now is the dialect default expression for created_at.
	now string
}

// pkConstraint renders the inline primary-key constraint for the id column.
// Postgres names the constraint to match the rest of the schema's convention;
// MySQL and SQLite use the bare PRIMARY KEY keyword. The dialect is passed
// explicitly rather than inferred from another field's value.
func pkConstraint(kind config.DBKind, tableName string) string {
	if kind == config.Postgres {
		return fmt.Sprintf("CONSTRAINT pk_%s PRIMARY KEY", tableName)
	}

	return "PRIMARY KEY"
}

var (
	postgresTypes = sqlTypes{
		pk:        "VARCHAR(32)",
		id:        "VARCHAR(32)",
		text:      "TEXT",
		number:    "NUMERIC",
		integer:   "BIGINT",
		boolean:   "BOOLEAN",
		date:      "DATE",
		datetime:  "TIMESTAMP",
		json:      "JSONB",
		timestamp: "TIMESTAMP",
		now:       "LOCALTIMESTAMP",
	}

	mysqlTypes = sqlTypes{
		pk:        "VARCHAR(32)",
		id:        "VARCHAR(32)",
		text:      "TEXT",
		number:    "DECIMAL(38,10)",
		integer:   "BIGINT",
		boolean:   "BOOLEAN",
		date:      "DATE",
		datetime:  "DATETIME",
		json:      "JSON",
		timestamp: "DATETIME",
		now:       "CURRENT_TIMESTAMP",
	}

	sqliteTypes = sqlTypes{
		pk:      "TEXT",
		id:      "TEXT",
		text:    "TEXT",
		number:  "NUMERIC",
		integer: "INTEGER",
		boolean: "INTEGER",
		// SQLite has no native date/time/json types; values are stored as ISO
		// strings / JSON text, so these project into TEXT (the same lossless
		// round-trip the json storage mode uses).
		date:      "TEXT",
		datetime:  "TEXT",
		json:      "TEXT",
		timestamp: "TIMESTAMP",
		// SQLite's CURRENT_TIMESTAMP yields UTC; the static apv_* tables default
		// created_at to local time, so match them with datetime('now','localtime').
		now: "(datetime('now', 'localtime'))",
	}
)

// typesFor returns the SQL type set for a dialect, or the zero sqlTypes for an
// unsupported one (callers treat the zero value as "unsupported").
func typesFor(kind config.DBKind) sqlTypes {
	switch kind {
	case config.Postgres:
		return postgresTypes
	case config.MySQL:
		return mysqlTypes
	case config.SQLite:
		return sqliteTypes
	default:
		return sqlTypes{}
	}
}

// maxMySQLVarcharChars is the largest VARCHAR length MySQL can declare for a
// single column — 65535 bytes ÷ 4 bytes/char (utf8mb4). maxPostgresVarcharChars
// is PostgreSQL's hard VARCHAR(n) ceiling. A declared length beyond a dialect's
// ceiling makes CREATE TABLE fail outright.
const (
	maxMySQLVarcharChars    = 16383
	maxPostgresVarcharChars = 10485760
)

// varcharType renders a bounded string column. SQLite has no length-enforced
// VARCHAR (its type affinity ignores the bound), so a string with a length maps
// to TEXT there — identical storage, one fewer dialect quirk. A length beyond
// the dialect's VARCHAR ceiling also degrades to TEXT rather than emitting a
// CREATE TABLE the dialect would reject; the application-layer MaxLength check
// still enforces the intended bound, so no length semantics are lost.
func varcharType(kind config.DBKind, length int) string {
	if kind == config.SQLite {
		return "TEXT"
	}

	ceiling := maxPostgresVarcharChars
	if kind == config.MySQL {
		ceiling = maxMySQLVarcharChars
	}

	if length > ceiling {
		return "TEXT"
	}

	return fmt.Sprintf("VARCHAR(%d)", length)
}

// maxDecimalScale caps the fractional digits of a generated DECIMAL/NUMERIC
// column. MySQL rejects a scale above 30; PostgreSQL allows up to the precision
// (38 here). Clamping to 30 keeps the column shape identical across dialects and
// never fails CREATE TABLE — 30 fractional digits exceeds any realistic field.
const maxDecimalScale = 30

// decimalType renders a fixed-point column with the given scale. SQLite has no
// exact decimal type, so it falls back to NUMERIC affinity (values are not
// stored with enforced scale); Postgres/MySQL get an exact NUMERIC/DECIMAL. An
// over-large scale is clamped to maxDecimalScale so it cannot fail MySQL DDL.
func decimalType(kind config.DBKind, scale int) string {
	if scale > maxDecimalScale {
		scale = maxDecimalScale
	}

	switch kind {
	case config.SQLite:
		return "NUMERIC"
	case config.MySQL:
		return fmt.Sprintf("DECIMAL(38,%d)", scale)
	default:
		return fmt.Sprintf("NUMERIC(38,%d)", scale)
	}
}

// columnTypeFor resolves the physical column type for a form field. It prefers
// the field's explicit, dialect-independent ColumnType (the form designer's
// inference / user override), mapping it to a concrete SQL type for the dialect.
// A field with no ColumnType (a schema authored before the field existed) falls
// back to the coarse kind-based mapping so its column shape is unchanged.
func columnTypeFor(kind config.DBKind, t sqlTypes, field approval.FormFieldDefinition) string {
	if field.ColumnType == "" {
		return sqlTypeForKind(t, field.Kind)
	}

	switch field.ColumnType {
	case approval.ColumnString:
		// VARCHAR(maxLength) when a bound is declared; otherwise falls through to
		// the lossless TEXT fallback below.
		if field.Validation != nil && field.Validation.MaxLength != nil && *field.Validation.MaxLength > 0 {
			return varcharType(kind, *field.Validation.MaxLength)
		}
	case approval.ColumnInteger:
		return t.integer
	case approval.ColumnDecimal:
		scale := 0
		if field.Scale != nil && *field.Scale > 0 {
			scale = *field.Scale
		}

		return decimalType(kind, scale)

	case approval.ColumnBoolean:
		return t.boolean
	case approval.ColumnDate:
		return t.date
	case approval.ColumnDatetime:
		return t.datetime
	case approval.ColumnJSON:
		return t.json
	}

	// ColumnText, a ColumnString without a length bound, and any unrecognized
	// type all project into a lossless TEXT column. The identifier is validated
	// elsewhere, so an unknown type never reaches DDL un-checked.
	return t.text
}

// sqlTypeForKind maps a form field kind to its physical column type in the given
// dialect. It is the fallback used when a field carries no explicit ColumnType.
// Unknown / unhandled kinds intentionally fall back to text so a schema with a
// kind the storage layer does not recognize still produces a valid, lossless
// column rather than failing generation.
func sqlTypeForKind(t sqlTypes, kind approval.FieldKind) string {
	if kind == approval.FieldNumber {
		return t.number
	}

	// Every other field kind — input/textarea/select/upload/date and any kind the
	// storage layer does not recognize — projects into a TEXT column. Date values
	// in particular are validated and bound as plain ISO strings (exactly how
	// apv_instance.form_data holds them), so TEXT is lossless, identical
	// cross-dialect, and free of the empty-string-vs-temporal-column rejection a
	// real DATE/TIMESTAMP type would impose on an optional, unfilled date.
	return t.text
}
