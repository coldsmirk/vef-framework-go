package migration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ErrSchemaOutdated means the enabled store lacks schema capabilities required
// by this version or still contains a retired timeline column.
var ErrSchemaOutdated = errors.New("cron store schema is outdated")

var errInvalidIndexMetadata = errors.New("invalid cron store index metadata")

type schemaColumnKind string

const (
	columnVarchar   schemaColumnKind = "varchar"
	columnTimestamp schemaColumnKind = "timestamp"
	columnInt64     schemaColumnKind = "int64"
	columnInt32     schemaColumnKind = "int32"
	columnBool      schemaColumnKind = "bool"
	columnJSON      schemaColumnKind = "json"
	columnText      schemaColumnKind = "text"
)

type schemaColumn struct {
	table     string
	name      string
	kind      schemaColumnKind
	nullable  bool
	maxLength int
}

var requiredColumns = []schemaColumn{
	{table: "crn_schedule", name: "id", kind: columnVarchar, maxLength: 32},
	{table: "crn_schedule", name: "created_at", kind: columnTimestamp},
	{table: "crn_schedule", name: "updated_at", kind: columnTimestamp},
	{table: "crn_schedule", name: "created_by", kind: columnVarchar, maxLength: 32},
	{table: "crn_schedule", name: "updated_by", kind: columnVarchar, maxLength: 32},
	{table: "crn_schedule", name: "name", kind: columnVarchar, maxLength: 128},
	{table: "crn_schedule", name: "job_name", kind: columnVarchar, maxLength: 128},
	{table: "crn_schedule", name: "kind", kind: columnVarchar, maxLength: 16},
	{table: "crn_schedule", name: "expr", kind: columnText},
	{table: "crn_schedule", name: "timezone", kind: columnVarchar, maxLength: 64},
	{table: "crn_schedule", name: "every_ms", kind: columnInt64},
	{table: "crn_schedule", name: "fire_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_schedule", name: "starts_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_schedule", name: "ends_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_schedule", name: "anchor_at_unix_ms", kind: columnInt64},
	{table: "crn_schedule", name: "params", kind: columnJSON, nullable: true},
	{table: "crn_schedule", name: "misfire_policy", kind: columnVarchar, maxLength: 16},
	{table: "crn_schedule", name: "concurrency_policy", kind: columnVarchar, maxLength: 16},
	{table: "crn_schedule", name: "recover", kind: columnBool},
	{table: "crn_schedule", name: "timeout_ms", kind: columnInt64},
	{table: "crn_schedule", name: "is_enabled", kind: columnBool},
	{table: "crn_schedule", name: "next_fire_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_schedule", name: "last_fire_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_fire_request", name: "id", kind: columnVarchar, maxLength: 32},
	{table: "crn_fire_request", name: "schedule_id", kind: columnVarchar, maxLength: 32},
	{table: "crn_fire_request", name: "kind", kind: columnVarchar, maxLength: 16},
	{table: "crn_fire_request", name: "scheduled_at_unix_ms", kind: columnInt64},
	{table: "crn_fire_request", name: "source_run_id", kind: columnVarchar, nullable: true, maxLength: 32},
	{table: "crn_run", name: "id", kind: columnVarchar, maxLength: 32},
	{table: "crn_run", name: "created_at", kind: columnTimestamp},
	{table: "crn_run", name: "created_by", kind: columnVarchar, maxLength: 32},
	{table: "crn_run", name: "schedule_id", kind: columnVarchar, maxLength: 32},
	{table: "crn_run", name: "schedule_name", kind: columnVarchar, maxLength: 128},
	{table: "crn_run", name: "job_name", kind: columnVarchar, maxLength: 128},
	{table: "crn_run", name: "scheduled_at_unix_ms", kind: columnInt64},
	{table: "crn_run", name: "claimed_at_unix_ms", kind: columnInt64},
	{table: "crn_run", name: "status", kind: columnVarchar, maxLength: 16},
	{table: "crn_run", name: "node_id", kind: columnText},
	{table: "crn_run", name: "started_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_run", name: "finished_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_run", name: "duration_ms", kind: columnInt64},
	{table: "crn_run", name: "heartbeat_at_unix_ms", kind: columnInt64, nullable: true},
	{table: "crn_run", name: "error", kind: columnText},
	{table: "crn_run", name: "missed_count", kind: columnInt32},
}

var obsoleteTimelineColumns = []schemaColumn{
	{table: "crn_schedule", name: "fire_at"},
	{table: "crn_schedule", name: "starts_at"},
	{table: "crn_schedule", name: "ends_at"},
	{table: "crn_schedule", name: "next_fire_at"},
	{table: "crn_schedule", name: "last_fire_at"},
	{table: "crn_fire_request", name: "scheduled_at_ms"},
	{table: "crn_run", name: "scheduled_at"},
	{table: "crn_run", name: "started_at"},
	{table: "crn_run", name: "finished_at"},
	{table: "crn_run", name: "heartbeat_at"},
}

type schemaIndex struct {
	table   string
	name    string
	columns []string
	unique  bool
}

var requiredIndexes = []schemaIndex{
	{
		table:   "crn_schedule",
		name:    "pk_crn_schedule",
		columns: []string{"id"},
		unique:  true,
	},
	{
		table:   "crn_schedule",
		name:    "uk_crn_schedule__name",
		columns: []string{"name"},
		unique:  true,
	},
	{
		table:   "crn_schedule",
		name:    "idx_crn_schedule__is_enabled_next_fire_at_unix_ms",
		columns: []string{"is_enabled", "next_fire_at_unix_ms"},
	},
	{
		table:   "crn_fire_request",
		name:    "pk_crn_fire_request",
		columns: []string{"id"},
		unique:  true,
	},
	{
		table:   "crn_fire_request",
		name:    "uk_crn_fire_request__source_run_id",
		columns: []string{"source_run_id"},
		unique:  true,
	},
	{
		table:   "crn_fire_request",
		name:    "idx_crn_fire_request__schedule_id_scheduled_at_unix_ms",
		columns: []string{"schedule_id", "scheduled_at_unix_ms", "id"},
	},
	{
		table:   "crn_run",
		name:    "pk_crn_run",
		columns: []string{"id"},
		unique:  true,
	},
	{
		table:   "crn_run",
		name:    "idx_crn_run__schedule_id_scheduled_at_unix_ms",
		columns: []string{"schedule_id", "scheduled_at_unix_ms"},
	},
	{
		table:   "crn_run",
		name:    "idx_crn_run__status_heartbeat_at_unix_ms",
		columns: []string{"status", "heartbeat_at_unix_ms"},
	},
	{
		table:   "crn_run",
		name:    "idx_crn_run__schedule_id_status",
		columns: []string{"schedule_id", "status"},
	},
	{
		table:   "crn_run",
		name:    "idx_crn_run__finished_at_unix_ms",
		columns: []string{"finished_at_unix_ms"},
	},
	{
		// The id tie-breaker matches the run list's canonical
		// (claimed_at_unix_ms DESC, id DESC) ordering, so paging stays an
		// index walk instead of a top-N sort.
		table:   "crn_run",
		name:    "idx_crn_run__claimed_at_unix_ms",
		columns: []string{"claimed_at_unix_ms", "id"},
	},
}

// Verify checks schema capability without changing it. Enabled deployments
// call this even when auto migration is off so a stale schema fails at boot.
func Verify(ctx context.Context, db orm.DB, kind config.DBKind) error {
	if !supportsMetadata(kind) {
		return fmt.Errorf("%w %q", sqlmigration.ErrUnsupportedDBKind, kind)
	}

	columnsByTable := make(map[string]map[string]schemaColumnMetadata, len(expectedTables))
	indexesByTable := make(map[string][]schemaIndexMetadata, len(expectedTables))

	for _, table := range expectedTables {
		exists, err := tableExists(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("verify table %s: %w", table, err)
		}

		if !exists {
			return outdated("missing table %s", table)
		}

		columns, err := loadTableColumns(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("load columns for %s: %w", table, err)
		}

		columnsByTable[table] = columns

		indexes, err := loadTableIndexes(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("load indexes for %s: %w", table, err)
		}

		indexesByTable[table] = indexes
	}

	if err := verifyColumnCapabilities(columnsByTable); err != nil {
		return err
	}

	for _, index := range requiredIndexes {
		if !hasIndexCapability(indexesByTable[index.table], index) {
			qualifier := ""
			if index.unique {
				qualifier = "unique "
			}

			return outdated(
				"missing %sindex %s on %s(%v)",
				qualifier,
				index.name,
				index.table,
				index.columns,
			)
		}
	}

	return nil
}

type schemaColumnMetadata struct {
	kind      schemaColumnKind
	nullable  bool
	maxLength int
}

func verifyColumnCapabilities(columnsByTable map[string]map[string]schemaColumnMetadata) error {
	for _, expected := range requiredColumns {
		actual, exists := columnsByTable[expected.table][expected.name]
		if !exists {
			return outdated("missing column %s.%s", expected.table, expected.name)
		}

		if actual.kind != expected.kind {
			return outdated(
				"column %s.%s has type %s, want %s",
				expected.table,
				expected.name,
				actual.kind,
				expected.kind,
			)
		}

		if actual.nullable != expected.nullable {
			return outdated(
				"column %s.%s nullable=%t, want nullable=%t",
				expected.table,
				expected.name,
				actual.nullable,
				expected.nullable,
			)
		}

		// Narrower columns truncate persisted identifiers; wider ones lose no
		// capability, so a DBA-widened column stays acceptable.
		if expected.maxLength > 0 && actual.maxLength < expected.maxLength {
			return outdated(
				"column %s.%s has length %d, want at least %d",
				expected.table,
				expected.name,
				actual.maxLength,
				expected.maxLength,
			)
		}
	}

	for _, obsolete := range obsoleteTimelineColumns {
		if _, exists := columnsByTable[obsolete.table][obsolete.name]; exists {
			return outdated("obsolete column %s.%s is present", obsolete.table, obsolete.name)
		}
	}

	return nil
}

func tableExists(ctx context.Context, db orm.DB, kind config.DBKind, table string) (bool, error) {
	query := ""
	switch kind {
	case config.Postgres:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?"
	case config.MySQL:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
	case config.SQLite:
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?"
	default:
		return false, fmt.Errorf("%w %q", sqlmigration.ErrUnsupportedDBKind, kind)
	}

	return metadataExists(ctx, db, query, table)
}

type schemaColumnRow struct {
	Name       string `bun:"column_name"`
	DataType   string `bun:"data_type"`
	NativeType string `bun:"native_type"`
	ColumnType string `bun:"column_type"`
	IsNullable int    `bun:"is_nullable"`
	MaxLength  int    `bun:"max_length"`
}

func loadTableColumns(
	ctx context.Context,
	db orm.DB,
	kind config.DBKind,
	table string,
) (map[string]schemaColumnMetadata, error) {
	query := ""

	switch kind {
	case config.Postgres:
		query = `SELECT column_name,
       data_type,
       udt_name AS native_type,
       '' AS column_type,
       CASE WHEN is_nullable = 'YES' THEN 1 ELSE 0 END AS is_nullable,
       COALESCE(character_maximum_length, 0) AS max_length
FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = ?
ORDER BY ordinal_position`

	case config.MySQL:
		query = "SELECT COLUMN_NAME AS `column_name`,\n" +
			"       DATA_TYPE AS `data_type`,\n" +
			"       '' AS `native_type`,\n" +
			"       COLUMN_TYPE AS `column_type`,\n" +
			"       CASE WHEN IS_NULLABLE = 'YES' THEN 1 ELSE 0 END AS `is_nullable`,\n" +
			"       COALESCE(CHARACTER_MAXIMUM_LENGTH, 0) AS `max_length`\n" +
			`FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = ?
ORDER BY ORDINAL_POSITION`

	case config.SQLite:
		query = `SELECT name AS column_name,
       type AS data_type,
       type AS native_type,
       type AS column_type,
       CASE WHEN "notnull" = 0 THEN 1 ELSE 0 END AS is_nullable,
       0 AS max_length
FROM pragma_table_info(?)
ORDER BY cid`

	default:
		return nil, fmt.Errorf("%w %q", sqlmigration.ErrUnsupportedDBKind, kind)
	}

	var rows []schemaColumnRow
	if err := db.NewRaw(query, table).Scan(ctx, &rows); err != nil {
		return nil, err
	}

	columns := make(map[string]schemaColumnMetadata, len(rows))
	for _, row := range rows {
		columnKind, maxLength := normalizeColumnType(kind, row)
		columns[row.Name] = schemaColumnMetadata{
			kind:      columnKind,
			nullable:  row.IsNullable != 0,
			maxLength: maxLength,
		}
	}

	return columns, nil
}

func normalizeColumnType(kind config.DBKind, row schemaColumnRow) (schemaColumnKind, int) {
	var (
		normalized schemaColumnKind
		maxLength  int
		ok         bool
	)

	switch kind {
	case config.Postgres:
		normalized, maxLength, ok = normalizePostgresColumnType(row)
	case config.MySQL:
		normalized, maxLength, ok = normalizeMySQLColumnType(row)
	case config.SQLite:
		normalized, maxLength, ok = normalizeSQLiteColumnType(row)
	}

	if ok {
		return normalized, maxLength
	}

	// An unrecognized declaration surfaces verbatim so the capability
	// mismatch names what the schema actually contains.
	return schemaColumnKind(strings.ToLower(strings.TrimSpace(row.DataType))), row.MaxLength
}

func normalizePostgresColumnType(row schemaColumnRow) (schemaColumnKind, int, bool) {
	dataType := strings.ToLower(strings.TrimSpace(row.DataType))
	nativeType := strings.ToLower(strings.TrimSpace(row.NativeType))

	switch {
	case dataType == "character varying" || nativeType == "varchar":
		return columnVarchar, row.MaxLength, true
	case dataType == "timestamp without time zone" || nativeType == "timestamp":
		return columnTimestamp, 0, true
	case dataType == "bigint" || nativeType == "int8":
		return columnInt64, 0, true
	case dataType == "integer" || nativeType == "int4":
		return columnInt32, 0, true
	case dataType == "boolean" || nativeType == "bool":
		return columnBool, 0, true
	case dataType == "jsonb" || nativeType == "jsonb":
		return columnJSON, 0, true
	case dataType == "text" || nativeType == "text":
		return columnText, 0, true
	default:
		return "", 0, false
	}
}

func normalizeMySQLColumnType(row schemaColumnRow) (schemaColumnKind, int, bool) {
	dataType := strings.ToLower(strings.TrimSpace(row.DataType))
	columnType := strings.ToLower(strings.TrimSpace(row.ColumnType))

	switch {
	case dataType == "varchar":
		return columnVarchar, row.MaxLength, true
	case dataType == "datetime":
		return columnTimestamp, 0, true
	case dataType == "bigint" && !strings.Contains(columnType, "unsigned"):
		return columnInt64, 0, true
	case dataType == "int" && !strings.Contains(columnType, "unsigned"):
		return columnInt32, 0, true
	case dataType == "tinyint" && columnType == "tinyint(1)":
		return columnBool, 0, true
	case dataType == "json":
		return columnJSON, 0, true
	case dataType == "text":
		return columnText, 0, true
	default:
		return "", 0, false
	}
}

func normalizeSQLiteColumnType(row schemaColumnRow) (schemaColumnKind, int, bool) {
	declared := strings.ToUpper(strings.TrimSpace(row.DataType))

	switch declared {
	case "TIMESTAMP":
		return columnTimestamp, 0, true
	case "BIGINT":
		return columnInt64, 0, true
	case "INTEGER":
		return columnInt32, 0, true
	case "BOOLEAN":
		return columnBool, 0, true
	case "JSONB":
		return columnJSON, 0, true
	case "TEXT":
		return columnText, 0, true
	default:
		if strings.HasPrefix(declared, "VARCHAR(") && strings.HasSuffix(declared, ")") {
			length, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(declared, "VARCHAR("), ")"))

			return columnVarchar, length, true
		}

		return "", 0, false
	}
}

type schemaIndexRow struct {
	IndexName string `bun:"index_name"`
	IsUnique  int    `bun:"is_unique"`
	Column    string `bun:"column_name"`
	Position  int    `bun:"position"`
}

type schemaIndexMetadata struct {
	name    string
	columns []string
	unique  bool
}

// hasIndexCapability matches by uniqueness and exact column sequence; index
// names are irrelevant to capability.
func hasIndexCapability(indexes []schemaIndexMetadata, expected schemaIndex) bool {
	return slices.ContainsFunc(indexes, func(index schemaIndexMetadata) bool {
		return index.unique == expected.unique && slices.Equal(index.columns, expected.columns)
	})
}

func loadTableIndexes(
	ctx context.Context,
	db orm.DB,
	kind config.DBKind,
	table string,
) ([]schemaIndexMetadata, error) {
	query := ""

	switch kind {
	case config.Postgres:
		query = `SELECT idx.relname AS index_name,
       CASE WHEN ix.indisunique THEN 1 ELSE 0 END AS is_unique,
       att.attname AS column_name,
       ord.ordinality - 1 AS position
FROM pg_class AS tbl
JOIN pg_namespace AS ns ON ns.oid = tbl.relnamespace
JOIN pg_index AS ix ON ix.indrelid = tbl.oid
JOIN pg_class AS idx ON idx.oid = ix.indexrelid
CROSS JOIN LATERAL unnest(ix.indkey::smallint[]) WITH ORDINALITY AS ord(attnum, ordinality)
JOIN pg_attribute AS att ON att.attrelid = tbl.oid AND att.attnum = ord.attnum
WHERE ns.nspname = current_schema()
  AND tbl.relname = ?
  AND ix.indisvalid
  AND ix.indpred IS NULL
  AND ix.indexprs IS NULL
  AND ord.ordinality <= ix.indnkeyatts
ORDER BY idx.relname, ord.ordinality`

	case config.MySQL:
		query = "SELECT INDEX_NAME AS `index_name`,\n" +
			"       CASE WHEN NON_UNIQUE = 0 THEN 1 ELSE 0 END AS `is_unique`,\n" +
			"       CASE WHEN COLUMN_NAME IS NULL OR SUB_PART IS NOT NULL THEN '' ELSE COLUMN_NAME END AS `column_name`,\n" +
			"       SEQ_IN_INDEX - 1 AS `position`\n" +
			`FROM information_schema.statistics
WHERE table_schema = DATABASE() AND table_name = ?
ORDER BY index_name, seq_in_index`

	case config.SQLite:
		query = `SELECT il.name AS index_name,
       il."unique" AS is_unique,
       COALESCE(ii.name, '') AS column_name,
       ii.seqno AS position
FROM pragma_index_list(?) AS il
JOIN pragma_index_info(il.name) AS ii
WHERE il.partial = 0
ORDER BY il.name, ii.seqno`

	default:
		return nil, fmt.Errorf("%w %q", sqlmigration.ErrUnsupportedDBKind, kind)
	}

	var rows []schemaIndexRow
	if err := db.NewRaw(query, table).Scan(ctx, &rows); err != nil {
		return nil, err
	}

	indexes := make([]schemaIndexMetadata, 0)
	for _, row := range rows {
		if len(indexes) == 0 || indexes[len(indexes)-1].name != row.IndexName {
			indexes = append(indexes, schemaIndexMetadata{
				name:   row.IndexName,
				unique: row.IsUnique != 0,
			})
		}

		index := &indexes[len(indexes)-1]
		if row.Position != len(index.columns) {
			return nil, fmt.Errorf(
				"%w: index %s on %s has non-contiguous column position %d",
				errInvalidIndexMetadata,
				row.IndexName,
				table,
				row.Position,
			)
		}

		index.columns = append(index.columns, row.Column)
	}

	return indexes, nil
}

func metadataExists(ctx context.Context, db orm.DB, query string, args ...any) (bool, error) {
	var count int
	if err := db.NewRaw(query, args...).Scan(ctx, &count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func supportsMetadata(kind config.DBKind) bool {
	return kind == config.Postgres || kind == config.MySQL || kind == config.SQLite
}

func outdated(format string, args ...any) error {
	detail := fmt.Sprintf(format, args...)

	return fmt.Errorf(
		"%w: %s; recreate the cron store tables with the schema required by this version",
		ErrSchemaOutdated,
		detail,
	)
}
