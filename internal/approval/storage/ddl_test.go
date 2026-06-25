package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
)

func TestBuildPhysicalTableName(t *testing.T) {
	const versionID = "c0v1d2e3f4g5h6i7j8k9" // XID-shaped: 20 chars from [0-9a-v]

	t.Run("sanitizes the readable code and suffixes the version id", func(t *testing.T) {
		name, err := buildPhysicalTableName("Leave-Request", versionID)
		require.NoError(t, err)
		assert.Equal(t, "apv_form_leave_request_"+versionID, name)
		assert.NoError(t, approval.ValidateBusinessIdentifier(name), "generated name must be a safe identifier")
	})

	t.Run("collapses invalid runs and lowercases", func(t *testing.T) {
		name, err := buildPhysicalTableName("My  Flow!!@#Code", versionID)
		require.NoError(t, err)
		assert.Equal(t, "apv_form_my_flow_code_"+versionID, name)
	})

	t.Run("omits an all-punctuation code but stays valid", func(t *testing.T) {
		name, err := buildPhysicalTableName("@@@", versionID)
		require.NoError(t, err)
		assert.Equal(t, "apv_form_"+versionID, name)
		assert.NoError(t, approval.ValidateBusinessIdentifier(name))
	})

	t.Run("truncates an oversize code but keeps the full version id", func(t *testing.T) {
		name, err := buildPhysicalTableName(strings.Repeat("a", 200), versionID)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(name), identifierMaxLen)
		assert.True(t, strings.HasPrefix(name, "apv_form_"))
		assert.True(t, strings.HasSuffix(name, "_"+versionID), "the unique version id is never truncated away")
		assert.NoError(t, approval.ValidateBusinessIdentifier(name))
	})

	t.Run("distinct versions of the same flow code never collide", func(t *testing.T) {
		// Two flows that share a code (e.g. the same code in two tenants) produce
		// two versions with distinct ids — and therefore distinct physical tables,
		// closing the cross-tenant projection-sharing hole.
		a, err := buildPhysicalTableName("leave", "aaaaaaaaaaaaaaaaaaaa")
		require.NoError(t, err)
		b, err := buildPhysicalTableName("leave", "bbbbbbbbbbbbbbbbbbbb")
		require.NoError(t, err)
		assert.NotEqual(t, a, b)
	})
}

func TestValidateTableFormSchema(t *testing.T) {
	t.Run("accepts a clean schema", func(t *testing.T) {
		err := ValidateTableFormSchema(&approval.FormDefinition{
			Fields: []approval.FormFieldDefinition{
				{Key: "reason", Kind: approval.FieldTextarea},
				{Key: "amount", Kind: approval.FieldNumber},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("nil schema is valid", func(t *testing.T) {
		assert.NoError(t, ValidateTableFormSchema(nil))
	})

	t.Run("rejects an unmappable leading-digit key", func(t *testing.T) {
		err := ValidateTableFormSchema(&approval.FormDefinition{
			Fields: []approval.FormFieldDefinition{{Key: "123abc", Kind: approval.FieldInput}},
		})
		assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
	})

	t.Run("rejects a reserved built-in column key", func(t *testing.T) {
		err := ValidateTableFormSchema(&approval.FormDefinition{
			Fields: []approval.FormFieldDefinition{{Key: "instance_id", Kind: approval.FieldInput}},
		})
		assert.True(t, errors.Is(err, ErrReservedColumnName))
	})

	t.Run("rejects two keys that sanitize to the same column", func(t *testing.T) {
		err := ValidateTableFormSchema(&approval.FormDefinition{
			Fields: []approval.FormFieldDefinition{
				{Key: "a.b", Kind: approval.FieldInput},
				{Key: "a-b", Kind: approval.FieldInput},
			},
		})
		assert.True(t, errors.Is(err, ErrDuplicateColumnName))
	})
}

func TestBuildColumnSpecs(t *testing.T) {
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldTextarea},
			{Key: "amount", Kind: approval.FieldNumber},
			{Key: "start_date", Kind: approval.FieldDate},
			{Key: "attachments", Kind: approval.FieldUpload},
		},
	}

	specs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)

	// id, instance_id, 4 fields, created_at.
	require.Len(t, specs, 7)
	assert.Equal(t, "id", specs[0].Name)
	assert.Equal(t, "instance_id", specs[1].Name)
	assert.Equal(t, "created_at", specs[len(specs)-1].Name)

	byName := map[string]columnSpec{}
	for _, s := range specs {
		byName[s.Name] = s
	}

	assert.Equal(t, "TEXT", byName["reason"].Type)
	assert.Equal(t, "NUMERIC", byName["amount"].Type)
	// Date fields project into a TEXT column (bound as plain ISO strings), not a
	// temporal type, so they round-trip losslessly and identically cross-dialect.
	assert.Equal(t, "TEXT", byName["start_date"].Type)
	assert.Equal(t, "TEXT", byName["attachments"].Type)
	assert.True(t, byName["reason"].IsNullable, "form columns are nullable")
	require.NotNil(t, byName["reason"].SourceFieldKey)
	assert.Equal(t, "reason", *byName["reason"].SourceFieldKey)
}

func columnsByName(specs []columnSpec) map[string]columnSpec {
	m := make(map[string]columnSpec, len(specs))
	for _, s := range specs {
		m[s.Name] = s
	}

	return m
}

func TestBuildColumnSpecsPreciseColumnTypes(t *testing.T) {
	maxLen := 64
	scale := 2
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{
				Key: "name", Kind: approval.FieldInput, ColumnType: approval.ColumnString, IsRequired: true,
				Validation: &approval.ValidationRule{MaxLength: &maxLen},
			},
			{Key: "bio", Kind: approval.FieldTextarea, ColumnType: approval.ColumnText},
			{Key: "qty", Kind: approval.FieldNumber, ColumnType: approval.ColumnInteger},
			{Key: "price", Kind: approval.FieldNumber, ColumnType: approval.ColumnDecimal, Scale: &scale},
			{Key: "active", Kind: approval.FieldInput, ColumnType: approval.ColumnBoolean},
			{Key: "birthday", Kind: approval.FieldDate, ColumnType: approval.ColumnDate},
			{Key: "created", Kind: approval.FieldDate, ColumnType: approval.ColumnDatetime},
			{Key: "tags", Kind: approval.FieldSelect, ColumnType: approval.ColumnJSON},
		},
	}

	specs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)

	byName := columnsByName(specs)

	assert.Equal(t, "VARCHAR(64)", byName["name"].Type)
	assert.False(t, byName["name"].IsNullable, "required field → NOT NULL")
	assert.Equal(t, "TEXT", byName["bio"].Type)
	assert.True(t, byName["bio"].IsNullable, "optional field → nullable")
	assert.Equal(t, "BIGINT", byName["qty"].Type)
	assert.Equal(t, "NUMERIC(38,2)", byName["price"].Type)
	assert.Equal(t, "BOOLEAN", byName["active"].Type)
	assert.Equal(t, "DATE", byName["birthday"].Type)
	assert.Equal(t, "TIMESTAMP", byName["created"].Type)
	assert.Equal(t, "JSONB", byName["tags"].Type)
}

func TestBuildColumnSpecsDialectVariants(t *testing.T) {
	maxLen := 32
	scale := 2
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "name", ColumnType: approval.ColumnString, Validation: &approval.ValidationRule{MaxLength: &maxLen}},
			{Key: "price", ColumnType: approval.ColumnDecimal, Scale: &scale},
			{Key: "birthday", ColumnType: approval.ColumnDate},
			{Key: "created", ColumnType: approval.ColumnDatetime},
			{Key: "tags", ColumnType: approval.ColumnJSON},
		},
	}

	mysqlSpecs, err := buildColumnSpecs(config.MySQL, schema)
	require.NoError(t, err)

	mysql := columnsByName(mysqlSpecs)
	assert.Equal(t, "VARCHAR(32)", mysql["name"].Type)
	assert.Equal(t, "DECIMAL(38,2)", mysql["price"].Type)
	assert.Equal(t, "DATE", mysql["birthday"].Type)
	assert.Equal(t, "DATETIME", mysql["created"].Type)
	assert.Equal(t, "JSON", mysql["tags"].Type)

	// SQLite has no native varchar-length / decimal / date / json: each degrades
	// to a lossless TEXT (or NUMERIC affinity) column.
	sqliteSpecs, err := buildColumnSpecs(config.SQLite, schema)
	require.NoError(t, err)

	sqlite := columnsByName(sqliteSpecs)
	assert.Equal(t, "TEXT", sqlite["name"].Type)
	assert.Equal(t, "NUMERIC", sqlite["price"].Type)
	assert.Equal(t, "TEXT", sqlite["birthday"].Type)
	assert.Equal(t, "TEXT", sqlite["created"].Type)
	assert.Equal(t, "TEXT", sqlite["tags"].Type)
}

func TestColumnTypeFallbackAndStringWithoutLength(t *testing.T) {
	scale := 0
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			// No ColumnType: falls back to the kind-derived type (legacy shape).
			{Key: "legacy_num", Kind: approval.FieldNumber},
			{Key: "legacy_txt", Kind: approval.FieldInput},
			// ColumnString without a maxLength projects into lossless TEXT.
			{Key: "code", ColumnType: approval.ColumnString},
			// ColumnDecimal with nil/zero scale → scale 0.
			{Key: "whole", ColumnType: approval.ColumnDecimal, Scale: &scale},
		},
	}

	specs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)

	byName := columnsByName(specs)

	assert.Equal(t, "NUMERIC", byName["legacy_num"].Type, "no ColumnType → kind fallback")
	assert.Equal(t, "TEXT", byName["legacy_txt"].Type)
	assert.Equal(t, "TEXT", byName["code"].Type, "string without maxLength → TEXT")
	assert.Equal(t, "NUMERIC(38,0)", byName["whole"].Type)
}

func TestColumnTypeBoundsDegradeBeyondDialectLimits(t *testing.T) {
	// A MaxLength beyond a dialect's VARCHAR ceiling degrades to the unbounded
	// TEXT type instead of emitting a CREATE TABLE the dialect would reject; the
	// application-layer MaxLength check still enforces the bound, so only the
	// physical column type changes, not the validated length. An over-large
	// decimal scale clamps to the cross-dialect-safe maximum (30) for the same
	// reason — MySQL rejects a larger scale outright.
	midLen := 20000       // above MySQL's 16383 ceiling, below Postgres's 10485760
	pgOverLen := 20000000 // above Postgres's 10485760 ceiling
	bigScale := 40        // above MySQL's max DECIMAL scale of 30

	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "mid", ColumnType: approval.ColumnString, Validation: &approval.ValidationRule{MaxLength: &midLen}},
			{Key: "deep", ColumnType: approval.ColumnDecimal, Scale: &bigScale},
		},
	}

	mysqlSpecs, err := buildColumnSpecs(config.MySQL, schema)
	require.NoError(t, err)

	mysql := columnsByName(mysqlSpecs)
	assert.Equal(t, "TEXT", mysql["mid"].Type, "length beyond MySQL VARCHAR ceiling → TEXT")
	assert.Equal(t, "DECIMAL(38,30)", mysql["deep"].Type, "scale clamped to MySQL's max of 30")

	pgSpecs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)

	pg := columnsByName(pgSpecs)
	assert.Equal(t, "VARCHAR(20000)", pg["mid"].Type, "20000 is within Postgres's VARCHAR ceiling")
	assert.Equal(t, "NUMERIC(38,30)", pg["deep"].Type, "scale clamped to 30")

	pgOverSchema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "vast", ColumnType: approval.ColumnString, Validation: &approval.ValidationRule{MaxLength: &pgOverLen}},
		},
	}
	pgOverSpecs, err := buildColumnSpecs(config.Postgres, pgOverSchema)
	require.NoError(t, err)
	assert.Equal(t, "TEXT", columnsByName(pgOverSpecs)["vast"].Type, "length beyond Postgres VARCHAR ceiling → TEXT")
}

func TestBuildColumnSpecsSanitizesInjectionShapedKey(t *testing.T) {
	// An injection-shaped key is sanitized into a safe identifier (the value
	// can never reach DDL un-validated), not passed through verbatim.
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "drop table users;--", Kind: approval.FieldInput},
		},
	}

	specs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)

	var fieldCol *columnSpec
	for i := range specs {
		if specs[i].SourceFieldKey != nil {
			fieldCol = &specs[i]
		}
	}

	require.NotNil(t, fieldCol)
	assert.Equal(t, "drop_table_users_", fieldCol.Name)
	assert.NoError(t, approval.ValidateBusinessIdentifier(fieldCol.Name))
}

func TestBuildColumnSpecsRejectsUnmappableFieldKey(t *testing.T) {
	// A key that sanitizes to a leading-digit identifier cannot be a valid SQL
	// identifier and must be rejected rather than silently coerced.
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "123abc", Kind: approval.FieldInput},
		},
	}

	_, err := buildColumnSpecs(config.Postgres, schema)
	require.Error(t, err, "a field key that cannot map to a safe identifier must be rejected")
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestBuildColumnSpecsRejectsReservedColumn(t *testing.T) {
	for _, reserved := range []string{"id", "instance_id", "created_at"} {
		schema := &approval.FormDefinition{
			Fields: []approval.FormFieldDefinition{{Key: reserved, Kind: approval.FieldInput}},
		}

		_, err := buildColumnSpecs(config.Postgres, schema)
		require.Errorf(t, err, "field key %q collides with a built-in column", reserved)
		assert.True(t, errors.Is(err, ErrReservedColumnName), "expected reserved-column error for %q", reserved)
	}
}

func TestBuildColumnSpecsRejectsDuplicateAfterSanitize(t *testing.T) {
	// "a.b" and "a-b" both sanitize to "a_b".
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "a.b", Kind: approval.FieldInput},
			{Key: "a-b", Kind: approval.FieldInput},
		},
	}

	_, err := buildColumnSpecs(config.Postgres, schema)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDuplicateColumnName))
}

func TestBuildColumnSpecsUnsupportedDialect(t *testing.T) {
	_, err := buildColumnSpecs(config.Oracle, &approval.FormDefinition{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedDialect))
}

func TestRenderCreateTableShapes(t *testing.T) {
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldInput},
			{Key: "amount", Kind: approval.FieldNumber},
		},
	}

	specs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)

	createSQL, err := renderCreateTable(config.Postgres, "apv_form_demo_v1", specs)
	require.NoError(t, err)

	assert.Contains(t, createSQL, "CREATE TABLE IF NOT EXISTS apv_form_demo_v1")
	assert.Contains(t, createSQL, "id VARCHAR(32) CONSTRAINT pk_apv_form_demo_v1 PRIMARY KEY")
	// instance_id is UNIQUE: one projection row per instance, enforced by the DB.
	assert.Contains(t, createSQL, "instance_id VARCHAR(32) NOT NULL UNIQUE")
	assert.Contains(t, createSQL, "reason TEXT")
	assert.Contains(t, createSQL, "amount NUMERIC")
	assert.Contains(t, createSQL, "created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP")
}

func TestRenderCreateTablePreciseTypesAndNotNull(t *testing.T) {
	maxLen := 64
	schema := &approval.FormDefinition{
		Fields: []approval.FormFieldDefinition{
			{
				Key: "name", ColumnType: approval.ColumnString, IsRequired: true,
				Validation: &approval.ValidationRule{MaxLength: &maxLen},
			},
			{Key: "note", ColumnType: approval.ColumnText},
			{Key: "when", ColumnType: approval.ColumnDatetime},
		},
	}

	specs, err := buildColumnSpecs(config.Postgres, schema)
	require.NoError(t, err)
	createSQL, err := renderCreateTable(config.Postgres, "apv_form_demo_v1", specs)
	require.NoError(t, err)

	// Required field → NOT NULL with its precise type; optional field → no NOT NULL.
	assert.Contains(t, createSQL, "name VARCHAR(64) NOT NULL")
	assert.Contains(t, createSQL, "note TEXT")
	assert.NotContains(t, createSQL, "note TEXT NOT NULL")
	assert.Contains(t, createSQL, "when TIMESTAMP")
}

func TestRenderCreateTableRejectsUnsafeTableName(t *testing.T) {
	_, err := renderCreateTable(config.Postgres, "apv_form demo; DROP TABLE x", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestRenderCreateTableSQLiteAndMySQL(t *testing.T) {
	schema := &approval.FormDefinition{Fields: []approval.FormFieldDefinition{{Key: "n", Kind: approval.FieldNumber}}}

	sqliteSpecs, err := buildColumnSpecs(config.SQLite, schema)
	require.NoError(t, err)
	sqliteCreate, err := renderCreateTable(config.SQLite, "apv_form_demo_v1", sqliteSpecs)
	require.NoError(t, err)
	assert.Contains(t, sqliteCreate, "id TEXT PRIMARY KEY")
	assert.Contains(t, sqliteCreate, "instance_id TEXT NOT NULL UNIQUE")
	assert.Contains(t, sqliteCreate, "n NUMERIC")
	assert.Contains(t, sqliteCreate, "created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime'))")

	mysqlSpecs, err := buildColumnSpecs(config.MySQL, schema)
	require.NoError(t, err)
	mysqlCreate, err := renderCreateTable(config.MySQL, "apv_form_demo_v1", mysqlSpecs)
	require.NoError(t, err)
	assert.Contains(t, mysqlCreate, "id VARCHAR(32) PRIMARY KEY")
	assert.Contains(t, mysqlCreate, "instance_id VARCHAR(32) NOT NULL UNIQUE")
	assert.Contains(t, mysqlCreate, "n DECIMAL(38,10)")
	assert.Contains(t, mysqlCreate, "created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP")
}
