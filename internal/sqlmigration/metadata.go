package sqlmigration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/orm"
)

var errInvalidIndexMetadata = errors.New("sqlmigration: invalid index metadata")

// ColumnKind is the dialect-neutral classification of a column's declared
// type: schema verifiers compare requirements against it instead of against
// raw dialect type names.
type ColumnKind string

// Dialect-neutral column kinds. A declaration outside this vocabulary
// surfaces verbatim (lowercased) so capability mismatches name what the
// schema actually contains.
const (
	// ColumnVarchar is a length-bounded character column; Column.MaxLength
	// carries the bound where the dialect exposes one.
	ColumnVarchar ColumnKind = "varchar"
	// ColumnTimestamp is a wall-clock timestamp without timezone.
	ColumnTimestamp ColumnKind = "timestamp"
	// ColumnInt64 is a signed 64-bit integer; unsigned MySQL declarations
	// deliberately do not normalize to it.
	ColumnInt64 ColumnKind = "int64"
	// ColumnInt32 is a signed 32-bit integer; unsigned MySQL declarations
	// deliberately do not normalize to it.
	ColumnInt32 ColumnKind = "int32"
	// ColumnBool is a boolean (tinyint(1) on MySQL).
	ColumnBool ColumnKind = "bool"
	// ColumnJSON is a JSON document column (jsonb on Postgres).
	ColumnJSON ColumnKind = "json"
	// ColumnText is an unbounded character column.
	ColumnText ColumnKind = "text"
)

// Column is the observed metadata of one table column, normalized across
// dialects. LoadTableColumns keys it by column name.
type Column struct {
	// Kind is the dialect-neutral type classification.
	Kind ColumnKind
	// Nullable reports whether the column accepts NULL.
	Nullable bool
	// MaxLength is the declared character bound of a varchar column; zero
	// when unbounded or not exposed by the dialect (SQLite reports it only
	// through the declared type text).
	MaxLength int
}

// Index is the observed metadata of one table index. Capability checks
// match on Columns and Unique; Name exists for diagnostics only.
type Index struct {
	// Name is the index's schema name.
	Name string
	// Columns is the exact key column sequence.
	Columns []string
	// Unique reports whether the index enforces uniqueness.
	Unique bool
}

// columnMetadataRow is the raw scan shape shared by the dialect column
// queries; each dialect aliases its catalog columns onto it.
type columnMetadataRow struct {
	Name       string `bun:"column_name"`
	DataType   string `bun:"data_type"`
	NativeType string `bun:"native_type"`
	ColumnType string `bun:"column_type"`
	IsNullable int    `bun:"is_nullable"`
	MaxLength  int    `bun:"max_length"`
}

// LoadTableColumns returns the table's columns keyed by name, with types
// normalized into the ColumnKind vocabulary. The probe follows the
// connection's active schema, like every sqlmigration metadata query.
func LoadTableColumns(
	ctx context.Context,
	db orm.DB,
	kind config.DBKind,
	table string,
) (map[string]Column, error) {
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
		return nil, fmt.Errorf("%w %q", ErrUnsupportedDBKind, kind)
	}

	var rows []columnMetadataRow
	if err := db.NewRaw(query, table).Scan(ctx, &rows); err != nil {
		return nil, err
	}

	columns := make(map[string]Column, len(rows))
	for _, row := range rows {
		columnKind, maxLength := normalizeColumnType(kind, row)
		columns[row.Name] = Column{
			Kind:      columnKind,
			Nullable:  row.IsNullable != 0,
			MaxLength: maxLength,
		}
	}

	return columns, nil
}

func normalizeColumnType(kind config.DBKind, row columnMetadataRow) (ColumnKind, int) {
	var (
		normalized ColumnKind
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
	return ColumnKind(strings.ToLower(strings.TrimSpace(row.DataType))), row.MaxLength
}

func normalizePostgresColumnType(row columnMetadataRow) (ColumnKind, int, bool) {
	dataType := strings.ToLower(strings.TrimSpace(row.DataType))
	nativeType := strings.ToLower(strings.TrimSpace(row.NativeType))

	switch {
	case dataType == "character varying" || nativeType == "varchar":
		return ColumnVarchar, row.MaxLength, true
	case dataType == "timestamp without time zone" || nativeType == "timestamp":
		return ColumnTimestamp, 0, true
	case dataType == "bigint" || nativeType == "int8":
		return ColumnInt64, 0, true
	case dataType == "integer" || nativeType == "int4":
		return ColumnInt32, 0, true
	case dataType == "boolean" || nativeType == "bool":
		return ColumnBool, 0, true
	case dataType == "jsonb" || nativeType == "jsonb":
		return ColumnJSON, 0, true
	case dataType == "text" || nativeType == "text":
		return ColumnText, 0, true
	default:
		return "", 0, false
	}
}

func normalizeMySQLColumnType(row columnMetadataRow) (ColumnKind, int, bool) {
	dataType := strings.ToLower(strings.TrimSpace(row.DataType))
	columnType := strings.ToLower(strings.TrimSpace(row.ColumnType))

	switch {
	case dataType == "varchar":
		return ColumnVarchar, row.MaxLength, true
	case dataType == "datetime":
		return ColumnTimestamp, 0, true
	case dataType == "bigint" && !strings.Contains(columnType, "unsigned"):
		return ColumnInt64, 0, true
	case dataType == "int" && !strings.Contains(columnType, "unsigned"):
		return ColumnInt32, 0, true
	case dataType == "tinyint" && columnType == "tinyint(1)":
		return ColumnBool, 0, true
	case dataType == "json":
		return ColumnJSON, 0, true
	case dataType == "text":
		return ColumnText, 0, true
	default:
		return "", 0, false
	}
}

func normalizeSQLiteColumnType(row columnMetadataRow) (ColumnKind, int, bool) {
	declared := strings.ToUpper(strings.TrimSpace(row.DataType))

	switch declared {
	case "TIMESTAMP":
		return ColumnTimestamp, 0, true
	case "BIGINT":
		return ColumnInt64, 0, true
	case "INTEGER":
		return ColumnInt32, 0, true
	case "BOOLEAN":
		return ColumnBool, 0, true
	case "JSONB":
		return ColumnJSON, 0, true
	case "TEXT":
		return ColumnText, 0, true
	default:
		if strings.HasPrefix(declared, "VARCHAR(") && strings.HasSuffix(declared, ")") {
			length, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(declared, "VARCHAR("), ")"))

			return ColumnVarchar, length, true
		}

		return "", 0, false
	}
}

// indexMetadataRow is the raw scan shape shared by the dialect index
// queries; each dialect aliases its catalog columns onto it.
type indexMetadataRow struct {
	IndexName string `bun:"index_name"`
	IsUnique  int    `bun:"is_unique"`
	Column    string `bun:"column_name"`
	Position  int    `bun:"position"`
}

// LoadTableIndexes returns the table's complete, non-partial indexes with
// their exact key column sequences. Partial and expression indexes are
// excluded — they cannot back a capability requirement — and MySQL prefix
// index parts surface as an empty column name so they never match one.
func LoadTableIndexes(
	ctx context.Context,
	db orm.DB,
	kind config.DBKind,
	table string,
) ([]Index, error) {
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
		return nil, fmt.Errorf("%w %q", ErrUnsupportedDBKind, kind)
	}

	var rows []indexMetadataRow
	if err := db.NewRaw(query, table).Scan(ctx, &rows); err != nil {
		return nil, err
	}

	indexes := make([]Index, 0)
	for _, row := range rows {
		if len(indexes) == 0 || indexes[len(indexes)-1].Name != row.IndexName {
			indexes = append(indexes, Index{
				Name:   row.IndexName,
				Unique: row.IsUnique != 0,
			})
		}

		index := &indexes[len(indexes)-1]
		if row.Position != len(index.Columns) {
			return nil, fmt.Errorf(
				"%w: index %s on %s has non-contiguous column position %d",
				errInvalidIndexMetadata,
				row.IndexName,
				table,
				row.Position,
			)
		}

		index.Columns = append(index.Columns, row.Column)
	}

	return indexes, nil
}
