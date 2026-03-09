package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/dialect"
)

// TestDataTypeFactory verifies that each factory method produces the correct DataTypeDef.
func TestDataTypeFactory(t *testing.T) {
	t.Run("SimpleKinds", func(t *testing.T) {
		tests := []struct {
			name     string
			factory  func() DataTypeDef
			expected DataTypeKind
		}{
			{"SmallInt", DataType.SmallInt, DataTypeSmallInt},
			{"Integer", DataType.Integer, DataTypeInteger},
			{"BigInt", DataType.BigInt, DataTypeBigInt},
			{"Real", DataType.Real, DataTypeReal},
			{"DoublePrecision", DataType.DoublePrecision, DataTypeDoublePrecision},
			{"Text", DataType.Text, DataTypeText},
			{"Boolean", DataType.Boolean, DataTypeBoolean},
			{"Date", DataType.Date, DataTypeDate},
			{"Time", DataType.Time, DataTypeTime},
			{"Timestamp", DataType.Timestamp, DataTypeTimestamp},
			{"TimestampTZ", DataType.TimestampWithTimeZone, DataTypeTimestampTZ},
			{"Blob", DataType.Blob, DataTypeBlob},
			{"UUID", DataType.UUID, DataTypeUUID},
			{"JSON", DataType.JSON, DataTypeJSON},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dt := tt.factory()
				assert.Equal(t, tt.expected, dt.kind, "Should produce the correct data type kind")
			})
		}
	})

	t.Run("VarCharWithLength", func(t *testing.T) {
		dt := DataType.VarChar(100)
		assert.Equal(t, DataTypeVarChar, dt.kind, "Should produce VarChar kind")
		assert.Equal(t, 100, dt.length, "Should store the specified length")
	})

	t.Run("CharWithLength", func(t *testing.T) {
		dt := DataType.Char(10)
		assert.Equal(t, DataTypeChar, dt.kind, "Should produce Char kind")
		assert.Equal(t, 10, dt.length, "Should store the specified length")
	})

	t.Run("NumericWithPrecisionScale", func(t *testing.T) {
		dt := DataType.Numeric(10, 2)
		assert.Equal(t, DataTypeNumeric, dt.kind, "Should produce Numeric kind")
		assert.Equal(t, 10, dt.precision, "Should store the specified precision")
		assert.Equal(t, 2, dt.scale, "Should store the specified scale")
	})

	t.Run("BinaryWithLength", func(t *testing.T) {
		dt := DataType.Binary(16)
		assert.Equal(t, DataTypeBinary, dt.kind, "Should produce Binary kind")
		assert.Equal(t, 16, dt.length, "Should store the specified length")
	})
}

// TestDataTypeRender verifies dialect-specific SQL type rendering for all data types.
func TestDataTypeRender(t *testing.T) {
	dialects := []struct {
		name  dialect.Name
		label string
	}{
		{dialect.PG, "PostgreSQL"},
		{dialect.MySQL, "MySQL"},
		{dialect.SQLite, "SQLite"},
	}

	tests := []struct {
		name     string
		dt       DataTypeDef
		expected map[dialect.Name]string
	}{
		{
			name: "SmallInt",
			dt:   DataType.SmallInt(),
			expected: map[dialect.Name]string{
				dialect.PG: "SMALLINT", dialect.MySQL: "SMALLINT", dialect.SQLite: "INTEGER",
			},
		},
		{
			name: "Integer",
			dt:   DataType.Integer(),
			expected: map[dialect.Name]string{
				dialect.PG: "INTEGER", dialect.MySQL: "INT", dialect.SQLite: "INTEGER",
			},
		},
		{
			name: "BigInt",
			dt:   DataType.BigInt(),
			expected: map[dialect.Name]string{
				dialect.PG: "BIGINT", dialect.MySQL: "BIGINT", dialect.SQLite: "INTEGER",
			},
		},
		{
			name: "Numeric",
			dt:   DataType.Numeric(10, 2),
			expected: map[dialect.Name]string{
				dialect.PG: "NUMERIC(10,2)", dialect.MySQL: "DECIMAL(10,2)", dialect.SQLite: "REAL",
			},
		},
		{
			name: "Real",
			dt:   DataType.Real(),
			expected: map[dialect.Name]string{
				dialect.PG: "REAL", dialect.MySQL: "FLOAT", dialect.SQLite: "REAL",
			},
		},
		{
			name: "DoublePrecision",
			dt:   DataType.DoublePrecision(),
			expected: map[dialect.Name]string{
				dialect.PG: "DOUBLE PRECISION", dialect.MySQL: "DOUBLE", dialect.SQLite: "REAL",
			},
		},
		{
			name: "VarChar",
			dt:   DataType.VarChar(100),
			expected: map[dialect.Name]string{
				dialect.PG: "VARCHAR(100)", dialect.MySQL: "VARCHAR(100)", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "Char",
			dt:   DataType.Char(10),
			expected: map[dialect.Name]string{
				dialect.PG: "CHAR(10)", dialect.MySQL: "CHAR(10)", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "Text",
			dt:   DataType.Text(),
			expected: map[dialect.Name]string{
				dialect.PG: "TEXT", dialect.MySQL: "TEXT", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "Boolean",
			dt:   DataType.Boolean(),
			expected: map[dialect.Name]string{
				dialect.PG: "BOOLEAN", dialect.MySQL: "BOOLEAN", dialect.SQLite: "INTEGER",
			},
		},
		{
			name: "Date",
			dt:   DataType.Date(),
			expected: map[dialect.Name]string{
				dialect.PG: "DATE", dialect.MySQL: "DATE", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "Time",
			dt:   DataType.Time(),
			expected: map[dialect.Name]string{
				dialect.PG: "TIME", dialect.MySQL: "TIME", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "Timestamp",
			dt:   DataType.Timestamp(),
			expected: map[dialect.Name]string{
				dialect.PG: "TIMESTAMP", dialect.MySQL: "DATETIME", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "TimestampWithTimeZone",
			dt:   DataType.TimestampWithTimeZone(),
			expected: map[dialect.Name]string{
				dialect.PG: "TIMESTAMPTZ", dialect.MySQL: "DATETIME", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "Binary",
			dt:   DataType.Binary(16),
			expected: map[dialect.Name]string{
				dialect.PG: "BYTEA", dialect.MySQL: "BINARY(16)", dialect.SQLite: "BLOB",
			},
		},
		{
			name: "Blob",
			dt:   DataType.Blob(),
			expected: map[dialect.Name]string{
				dialect.PG: "BYTEA", dialect.MySQL: "LONGBLOB", dialect.SQLite: "BLOB",
			},
		},
		{
			name: "UUID",
			dt:   DataType.UUID(),
			expected: map[dialect.Name]string{
				dialect.PG: "UUID", dialect.MySQL: "VARCHAR(36)", dialect.SQLite: "TEXT",
			},
		},
		{
			name: "JSON",
			dt:   DataType.JSON(),
			expected: map[dialect.Name]string{
				dialect.PG: "JSONB", dialect.MySQL: "JSON", dialect.SQLite: "TEXT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, d := range dialects {
				t.Run(d.label, func(t *testing.T) {
					result := tt.dt.render(d.name)
					assert.Equal(t, tt.expected[d.name], result,
						"Should render correct SQL type for %s", d.label)
				})
			}
		})
	}
}

// TestDataTypeRenderUnknownKind verifies that unknown data type kinds fall back to TEXT.
func TestDataTypeRenderUnknownKind(t *testing.T) {
	dt := DataTypeDef{kind: DataTypeKind(999)}
	assert.Equal(t, "TEXT", dt.render(dialect.PG), "Unknown kind should fall back to TEXT")
}
