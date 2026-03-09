package orm

import (
	"fmt"

	"github.com/uptrace/bun/dialect"
)

// DataTypeKind identifies the SQL data type category.
type DataTypeKind int

const (
	DataTypeSmallInt DataTypeKind = iota
	DataTypeInteger
	DataTypeBigInt
	DataTypeNumeric
	DataTypeReal
	DataTypeDoublePrecision
	DataTypeVarChar
	DataTypeChar
	DataTypeText
	DataTypeBoolean
	DataTypeDate
	DataTypeTime
	DataTypeTimestamp
	DataTypeTimestampTZ
	DataTypeBinary
	DataTypeBlob
	DataTypeUUID
	DataTypeJSON
)

// dialectTypes maps each static DataTypeKind to its dialect-specific SQL string.
// Key order: [PostgreSQL, MySQL, SQLite].
var dialectTypes = map[DataTypeKind][3]string{
	DataTypeSmallInt:        {"SMALLINT", "SMALLINT", "INTEGER"},
	DataTypeInteger:         {"INTEGER", "INT", "INTEGER"},
	DataTypeBigInt:          {"BIGINT", "BIGINT", "INTEGER"},
	DataTypeReal:            {"REAL", "FLOAT", "REAL"},
	DataTypeDoublePrecision: {"DOUBLE PRECISION", "DOUBLE", "REAL"},
	DataTypeText:            {"TEXT", "TEXT", "TEXT"},
	DataTypeBoolean:         {"BOOLEAN", "BOOLEAN", "INTEGER"},
	DataTypeDate:            {"DATE", "DATE", "TEXT"},
	DataTypeTime:            {"TIME", "TIME", "TEXT"},
	DataTypeTimestamp:       {"TIMESTAMP", "DATETIME", "TEXT"},
	DataTypeTimestampTZ:     {"TIMESTAMPTZ", "DATETIME", "TEXT"},
	DataTypeBlob:            {"BYTEA", "LONGBLOB", "BLOB"},
	DataTypeUUID:            {"UUID", "VARCHAR(36)", "TEXT"},
	DataTypeJSON:            {"JSONB", "JSON", "TEXT"},
}

// dialectIndex returns the dialectTypes array index for a dialect.Name.
func dialectIndex(d dialect.Name) int {
	switch d {
	case dialect.MySQL:
		return 1
	case dialect.SQLite:
		return 2
	default: // PostgreSQL and others
		return 0
	}
}

// DataTypeDef represents a SQL data type definition with optional length, precision, and scale.
type DataTypeDef struct {
	kind      DataTypeKind
	length    int // VarChar(n), Char(n), Binary(n)
	precision int // Numeric(p, s)
	scale     int
}

// render produces the dialect-specific SQL type string.
func (d DataTypeDef) render(dialectName dialect.Name) string {
	// Parameterized types require format strings with length/precision.
	switch d.kind {
	case DataTypeNumeric:
		return renderNumeric(dialectName, d.precision, d.scale)
	case DataTypeVarChar:
		return renderWithLength(dialectName, "VARCHAR", d.length)
	case DataTypeChar:
		return renderWithLength(dialectName, "CHAR", d.length)
	case DataTypeBinary:
		return renderBinary(dialectName, d.length)
	}

	// Static types: look up from the dialect mapping table.
	if mapping, ok := dialectTypes[d.kind]; ok {
		return mapping[dialectIndex(dialectName)]
	}

	return "TEXT"
}

// DataTypeFactory provides factory methods for creating DataTypeDef instances.
type DataTypeFactory struct{}

// DataType is the global factory for creating data type definitions.
var DataType = DataTypeFactory{}

func (DataTypeFactory) SmallInt() DataTypeDef {
	return DataTypeDef{kind: DataTypeSmallInt}
}

func (DataTypeFactory) Integer() DataTypeDef {
	return DataTypeDef{kind: DataTypeInteger}
}

func (DataTypeFactory) BigInt() DataTypeDef {
	return DataTypeDef{kind: DataTypeBigInt}
}

func (DataTypeFactory) Numeric(precision, scale int) DataTypeDef {
	return DataTypeDef{kind: DataTypeNumeric, precision: precision, scale: scale}
}

func (DataTypeFactory) Real() DataTypeDef {
	return DataTypeDef{kind: DataTypeReal}
}

func (DataTypeFactory) DoublePrecision() DataTypeDef {
	return DataTypeDef{kind: DataTypeDoublePrecision}
}

func (DataTypeFactory) VarChar(length int) DataTypeDef {
	return DataTypeDef{kind: DataTypeVarChar, length: length}
}

func (DataTypeFactory) Char(length int) DataTypeDef {
	return DataTypeDef{kind: DataTypeChar, length: length}
}

func (DataTypeFactory) Text() DataTypeDef {
	return DataTypeDef{kind: DataTypeText}
}

func (DataTypeFactory) Boolean() DataTypeDef {
	return DataTypeDef{kind: DataTypeBoolean}
}

func (DataTypeFactory) Date() DataTypeDef {
	return DataTypeDef{kind: DataTypeDate}
}

func (DataTypeFactory) Time() DataTypeDef {
	return DataTypeDef{kind: DataTypeTime}
}

func (DataTypeFactory) Timestamp() DataTypeDef {
	return DataTypeDef{kind: DataTypeTimestamp}
}

func (DataTypeFactory) TimestampWithTimeZone() DataTypeDef {
	return DataTypeDef{kind: DataTypeTimestampTZ}
}

func (DataTypeFactory) Binary(length int) DataTypeDef {
	return DataTypeDef{kind: DataTypeBinary, length: length}
}

func (DataTypeFactory) Blob() DataTypeDef {
	return DataTypeDef{kind: DataTypeBlob}
}

func (DataTypeFactory) UUID() DataTypeDef {
	return DataTypeDef{kind: DataTypeUUID}
}

func (DataTypeFactory) JSON() DataTypeDef {
	return DataTypeDef{kind: DataTypeJSON}
}

// --- dialect-specific render helpers for parameterized types ---

func renderNumeric(d dialect.Name, precision, scale int) string {
	switch d {
	case dialect.SQLite:
		return "REAL"
	case dialect.MySQL:
		return fmt.Sprintf("DECIMAL(%d,%d)", precision, scale)
	default:
		return fmt.Sprintf("NUMERIC(%d,%d)", precision, scale)
	}
}

// renderWithLength renders a type with a length parameter, falling back to TEXT for SQLite.
func renderWithLength(d dialect.Name, typeName string, length int) string {
	if d == dialect.SQLite {
		return "TEXT"
	}

	return fmt.Sprintf("%s(%d)", typeName, length)
}

func renderBinary(d dialect.Name, length int) string {
	switch d {
	case dialect.PG:
		return "BYTEA"
	case dialect.SQLite:
		return "BLOB"
	default:
		return fmt.Sprintf("BINARY(%d)", length)
	}
}
