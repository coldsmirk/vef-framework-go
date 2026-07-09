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

// mainTableColumns adapts the single-table call shape most DDL tests assert
// on: it resolves the full table-spec set and returns the main projection
// table's columns.
func mainTableColumns(kind config.DBKind, fields []approval.FormFieldDefinition) ([]columnSpec, error) {
	tables, err := buildTableSpecs(kind, "demo", "d95k65k5cmcbd4737ag0", fields)
	if err != nil {
		return nil, err
	}

	return tables[0].Columns, nil
}

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
	t.Run("accepts a clean field list", func(t *testing.T) {
		err := ValidateTableFormSchema([]approval.FormFieldDefinition{
			{Key: "reason", Kind: approval.FieldTextarea},
			{Key: "amount", Kind: approval.FieldNumber},
		})
		assert.NoError(t, err)
	})

	t.Run("nil field list is valid", func(t *testing.T) {
		assert.NoError(t, ValidateTableFormSchema(nil))
	})

	t.Run("rejects an unmappable leading-digit key", func(t *testing.T) {
		err := ValidateTableFormSchema([]approval.FormFieldDefinition{{Key: "123abc", Kind: approval.FieldInput}})
		assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
	})

	t.Run("rejects a reserved built-in column key", func(t *testing.T) {
		err := ValidateTableFormSchema([]approval.FormFieldDefinition{{Key: "instance_id", Kind: approval.FieldInput}})
		assert.True(t, errors.Is(err, ErrReservedColumnName))
	})

	t.Run("rejects two keys that sanitize to the same column", func(t *testing.T) {
		err := ValidateTableFormSchema([]approval.FormFieldDefinition{
			{Key: "a.b", Kind: approval.FieldInput},
			{Key: "a-b", Kind: approval.FieldInput},
		})
		assert.True(t, errors.Is(err, ErrDuplicateColumnName))
	})
}

func TestBuildColumnSpecs(t *testing.T) {
	fields := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldTextarea},
		{Key: "amount", Kind: approval.FieldNumber},
		{Key: "start_date", Kind: approval.FieldDate},
		{Key: "attachments", Kind: approval.FieldUpload},
	}

	specs, err := mainTableColumns(config.Postgres, fields)
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
	fields := []approval.FormFieldDefinition{
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
	}

	specs, err := mainTableColumns(config.Postgres, fields)
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
	fields := []approval.FormFieldDefinition{
		{Key: "name", ColumnType: approval.ColumnString, Validation: &approval.ValidationRule{MaxLength: &maxLen}},
		{Key: "price", ColumnType: approval.ColumnDecimal, Scale: &scale},
		{Key: "birthday", ColumnType: approval.ColumnDate},
		{Key: "created", ColumnType: approval.ColumnDatetime},
		{Key: "tags", ColumnType: approval.ColumnJSON},
	}

	mysqlSpecs, err := mainTableColumns(config.MySQL, fields)
	require.NoError(t, err)

	mysql := columnsByName(mysqlSpecs)
	assert.Equal(t, "VARCHAR(32)", mysql["name"].Type)
	assert.Equal(t, "DECIMAL(38,2)", mysql["price"].Type)
	assert.Equal(t, "DATE", mysql["birthday"].Type)
	assert.Equal(t, "DATETIME", mysql["created"].Type)
	assert.Equal(t, "JSON", mysql["tags"].Type)

	// SQLite has no native varchar-length / decimal / date / json: each degrades
	// to a lossless TEXT (or NUMERIC affinity) column.
	sqliteSpecs, err := mainTableColumns(config.SQLite, fields)
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
	fields := []approval.FormFieldDefinition{
		// No ColumnType: falls back to the kind-derived type (legacy shape).
		{Key: "legacy_num", Kind: approval.FieldNumber},
		{Key: "legacy_txt", Kind: approval.FieldInput},
		// ColumnString without a maxLength projects into lossless TEXT.
		{Key: "code", ColumnType: approval.ColumnString},
		// ColumnDecimal with nil/zero scale → scale 0.
		{Key: "whole", ColumnType: approval.ColumnDecimal, Scale: &scale},
	}

	specs, err := mainTableColumns(config.Postgres, fields)
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

	fields := []approval.FormFieldDefinition{
		{Key: "mid", ColumnType: approval.ColumnString, Validation: &approval.ValidationRule{MaxLength: &midLen}},
		{Key: "deep", ColumnType: approval.ColumnDecimal, Scale: &bigScale},
	}

	mysqlSpecs, err := mainTableColumns(config.MySQL, fields)
	require.NoError(t, err)

	mysql := columnsByName(mysqlSpecs)
	assert.Equal(t, "TEXT", mysql["mid"].Type, "length beyond MySQL VARCHAR ceiling → TEXT")
	assert.Equal(t, "DECIMAL(38,30)", mysql["deep"].Type, "scale clamped to MySQL's max of 30")

	pgSpecs, err := mainTableColumns(config.Postgres, fields)
	require.NoError(t, err)

	pg := columnsByName(pgSpecs)
	assert.Equal(t, "VARCHAR(20000)", pg["mid"].Type, "20000 is within Postgres's VARCHAR ceiling")
	assert.Equal(t, "NUMERIC(38,30)", pg["deep"].Type, "scale clamped to 30")

	pgOverFields := []approval.FormFieldDefinition{
		{Key: "vast", ColumnType: approval.ColumnString, Validation: &approval.ValidationRule{MaxLength: &pgOverLen}},
	}
	pgOverSpecs, err := mainTableColumns(config.Postgres, pgOverFields)
	require.NoError(t, err)
	assert.Equal(t, "TEXT", columnsByName(pgOverSpecs)["vast"].Type, "length beyond Postgres VARCHAR ceiling → TEXT")
}

func TestBuildColumnSpecsSanitizesInjectionShapedKey(t *testing.T) {
	// An injection-shaped key is sanitized into a safe identifier (the value
	// can never reach DDL un-validated), not passed through verbatim.
	fields := []approval.FormFieldDefinition{
		{Key: "drop table users;--", Kind: approval.FieldInput},
	}

	specs, err := mainTableColumns(config.Postgres, fields)
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
	fields := []approval.FormFieldDefinition{
		{Key: "123abc", Kind: approval.FieldInput},
	}

	_, err := mainTableColumns(config.Postgres, fields)
	require.Error(t, err, "a field key that cannot map to a safe identifier must be rejected")
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestBuildColumnSpecsRejectsReservedColumn(t *testing.T) {
	for _, reserved := range []string{"id", "instance_id", "created_at"} {
		fields := []approval.FormFieldDefinition{{Key: reserved, Kind: approval.FieldInput}}

		_, err := mainTableColumns(config.Postgres, fields)
		require.Errorf(t, err, "field key %q collides with a built-in column", reserved)
		assert.True(t, errors.Is(err, ErrReservedColumnName), "expected reserved-column error for %q", reserved)
	}
}

func TestBuildColumnSpecsRejectsDuplicateAfterSanitize(t *testing.T) {
	// "a.b" and "a-b" both sanitize to "a_b".
	fields := []approval.FormFieldDefinition{
		{Key: "a.b", Kind: approval.FieldInput},
		{Key: "a-b", Kind: approval.FieldInput},
	}

	_, err := mainTableColumns(config.Postgres, fields)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDuplicateColumnName))
}

func TestBuildColumnSpecsUnsupportedDialect(t *testing.T) {
	_, err := mainTableColumns(config.Oracle, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedDialect))
}

func TestRenderCreateTableShapes(t *testing.T) {
	fields := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput},
		{Key: "amount", Kind: approval.FieldNumber},
	}

	specs, err := mainTableColumns(config.Postgres, fields)
	require.NoError(t, err)

	createSQL, err := renderCreateTable(config.Postgres, tableSpec{Name: "apv_form_demo_v1", Columns: specs})
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
	fields := []approval.FormFieldDefinition{
		{
			Key: "name", ColumnType: approval.ColumnString, IsRequired: true,
			Validation: &approval.ValidationRule{MaxLength: &maxLen},
		},
		{Key: "note", ColumnType: approval.ColumnText},
		{Key: "when", ColumnType: approval.ColumnDatetime},
	}

	specs, err := mainTableColumns(config.Postgres, fields)
	require.NoError(t, err)
	createSQL, err := renderCreateTable(config.Postgres, tableSpec{Name: "apv_form_demo_v1", Columns: specs})
	require.NoError(t, err)

	// Required field → NOT NULL with its precise type; optional field → no NOT NULL.
	assert.Contains(t, createSQL, "name VARCHAR(64) NOT NULL")
	assert.Contains(t, createSQL, "note TEXT")
	assert.NotContains(t, createSQL, "note TEXT NOT NULL")
	assert.Contains(t, createSQL, "when TIMESTAMP")
}

func TestRenderCreateTableRejectsUnsafeTableName(t *testing.T) {
	_, err := renderCreateTable(config.Postgres, tableSpec{Name: "apv_form demo; DROP TABLE x", Columns: nil})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidGeneratedIdentifier))
}

func TestRenderCreateTableSQLiteAndMySQL(t *testing.T) {
	fields := []approval.FormFieldDefinition{{Key: "n", Kind: approval.FieldNumber}}

	sqliteSpecs, err := mainTableColumns(config.SQLite, fields)
	require.NoError(t, err)
	sqliteCreate, err := renderCreateTable(config.SQLite, tableSpec{Name: "apv_form_demo_v1", Columns: sqliteSpecs})
	require.NoError(t, err)
	assert.Contains(t, sqliteCreate, "id TEXT PRIMARY KEY")
	assert.Contains(t, sqliteCreate, "instance_id TEXT NOT NULL UNIQUE")
	assert.Contains(t, sqliteCreate, "n NUMERIC")
	assert.Contains(t, sqliteCreate, "created_at TIMESTAMP NOT NULL DEFAULT (datetime('now', 'localtime'))")

	mysqlSpecs, err := mainTableColumns(config.MySQL, fields)
	require.NoError(t, err)
	mysqlCreate, err := renderCreateTable(config.MySQL, tableSpec{Name: "apv_form_demo_v1", Columns: mysqlSpecs})
	require.NoError(t, err)
	assert.Contains(t, mysqlCreate, "id VARCHAR(32) PRIMARY KEY")
	assert.Contains(t, mysqlCreate, "instance_id VARCHAR(32) NOT NULL UNIQUE")
	assert.Contains(t, mysqlCreate, "n DECIMAL(38,10)")
	assert.Contains(t, mysqlCreate, "created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP")
}

func TestBuildTableSpecsWithDetailTables(t *testing.T) {
	fields := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldTextarea},
		{Key: "items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{
			{Key: "name", Kind: approval.FieldInput, IsRequired: true},
			{Key: "qty", Kind: approval.FieldNumber},
		}},
	}

	tables, err := buildTableSpecs(config.Postgres, "demo", "d95k65k5cmcbd4737ag0", fields)
	require.NoError(t, err, "specs should resolve")
	require.Len(t, tables, 2, "one main table plus one child per table field")

	main, child := tables[0], tables[1]
	assert.False(t, main.IsChild(), "first spec is the main table")
	assert.Equal(t, "items", child.SourceFieldKey, "child spec carries its source field")
	assert.Equal(t, "apv_form_d95k65k5cmcbd4737ag0__items", child.Name,
		"child names anchor on the version id with a fixed suffix budget, independent of the flow code")

	mainNames := make([]string, len(main.Columns))
	for i, c := range main.Columns {
		mainNames[i] = c.Name
	}

	assert.Equal(t, []string{"id", "instance_id", "reason", "created_at"}, mainNames,
		"the main table projects only scalar fields")

	childNames := make([]string, len(child.Columns))
	for i, c := range child.Columns {
		childNames[i] = c.Name
	}

	assert.Equal(t, []string{"id", "instance_id", "row_index", "name", "qty", "created_at"}, childNames,
		"child tables carry row_index between the built-ins and the columns")
}

func TestRenderTableStatementsForChild(t *testing.T) {
	fields := []approval.FormFieldDefinition{
		{Key: "items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{
			{Key: "qty", Kind: approval.FieldNumber},
		}},
	}

	t.Run("PostgresAddsSeparateIndex", func(t *testing.T) {
		tables, err := buildTableSpecs(config.Postgres, "demo", "d95k65k5cmcbd4737ag0", fields)
		require.NoError(t, err, "specs should resolve")

		statements, err := renderTableStatements(config.Postgres, tables[1])
		require.NoError(t, err, "child statements should render")
		require.Len(t, statements, 2, "CREATE TABLE plus the instance_id index")
		assert.Contains(t, statements[0], "row_index", "child DDL declares row_index")
		assert.NotContains(t, statements[0], "instance_id VARCHAR(32) NOT NULL UNIQUE",
			"child instance_id must not be unique — many rows per instance")
		assert.Contains(t, statements[1], "CREATE INDEX IF NOT EXISTS", "index statement must be idempotent")
		assert.Contains(t, statements[1], "(instance_id)", "index covers the lookup key")
	})

	t.Run("MySQLInlinesTheIndex", func(t *testing.T) {
		tables, err := buildTableSpecs(config.MySQL, "demo", "d95k65k5cmcbd4737ag0", fields)
		require.NoError(t, err, "specs should resolve")

		statements, err := renderTableStatements(config.MySQL, tables[1])
		require.NoError(t, err, "child statements should render")
		require.Len(t, statements, 1, "MySQL has no CREATE INDEX IF NOT EXISTS; the index is inlined")
		assert.Contains(t, statements[0], "INDEX idx_instance_id (instance_id)", "inline index keeps the DDL idempotent")
	})

	t.Run("MainTableKeepsUniqueInstance", func(t *testing.T) {
		tables, err := buildTableSpecs(config.Postgres, "demo", "d95k65k5cmcbd4737ag0", fields)
		require.NoError(t, err, "specs should resolve")

		statements, err := renderTableStatements(config.Postgres, tables[0])
		require.NoError(t, err, "main statements should render")
		require.Len(t, statements, 1, "the main table needs no extra index")
		assert.Contains(t, statements[0], "instance_id VARCHAR(32) NOT NULL UNIQUE",
			"one row per instance stays database-enforced")
	})
}

func TestValidateTableFormSchemaChildRules(t *testing.T) {
	t.Run("RejectsReservedChildColumn", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{
			{Key: "items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{
				{Key: "row_index", Kind: approval.FieldNumber},
			}},
		}
		assert.ErrorIs(t, ValidateTableFormSchema(fields), ErrReservedColumnName,
			"child columns must not shadow the built-in row_index")
	})

	t.Run("RejectsTableKeysSanitizingToSameName", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{
			{Key: "line items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "a", Kind: approval.FieldInput}}},
			{Key: "line-items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "a", Kind: approval.FieldInput}}},
		}
		assert.ErrorIs(t, ValidateTableFormSchema(fields), ErrDuplicateColumnName,
			"two table fields must not map to the same child table name")
	})

	t.Run("AllowsSameColumnKeyAcrossTables", func(t *testing.T) {
		fields := []approval.FormFieldDefinition{
			{Key: "amount", Kind: approval.FieldNumber},
			{Key: "a", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "amount", Kind: approval.FieldNumber}}},
			{Key: "b", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "amount", Kind: approval.FieldNumber}}},
		}
		assert.NoError(t, ValidateTableFormSchema(fields),
			"column pools are per table — the same key may appear in different tables and the main form")
	})
}

func TestChildTableNamingIsFlowCodeIndependent(t *testing.T) {
	fields := []approval.FormFieldDefinition{
		{Key: "items", Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{
			{Key: "qty", Kind: approval.FieldNumber},
		}},
	}

	// A long, entirely ordinary flow code consumes the main table's whole
	// readable budget; child tables must not be starved by it.
	tables, err := buildTableSpecs(config.Postgres, "marketing_budget_approval_request_process", "d95k65k5cmcbd4737ag0", fields)
	require.NoError(t, err, "a long flow code must not block child table generation")
	require.Len(t, tables, 2, "main plus child")
	assert.Equal(t, "apv_form_d95k65k5cmcbd4737ag0__items", tables[1].Name,
		"the child name budget is fixed and never depends on the flow code")
	assert.NoError(t, approval.ValidateBusinessIdentifier(tables[1].Name))
}

func TestValidateTableFormSchemaDeduplicatesTruncatedSuffixes(t *testing.T) {
	long := func(tail string) string { return "expense_details_breakdown_lines_" + tail }
	fields := []approval.FormFieldDefinition{
		{Key: long("january"), Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "a", Kind: approval.FieldInput}}},
		{Key: long("february"), Kind: approval.FieldTable, Columns: []approval.FormFieldDefinition{{Key: "a", Kind: approval.FieldInput}}},
	}

	assert.ErrorIs(t, ValidateTableFormSchema(fields), ErrDuplicateColumnName,
		"keys colliding after suffix truncation must fail at deploy, not silently share one physical table at publish")
}
