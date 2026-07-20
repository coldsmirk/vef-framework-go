package migration

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/sqlmigration"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ErrSchemaOutdated means the enabled store lacks schema capabilities required
// by this version or still contains a retired timeline column.
var ErrSchemaOutdated = errors.New("cron store schema is outdated")

// columnRequirement declares one column capability the store depends on:
// its normalized type, nullability, and — for identifier columns — the
// minimum character width.
type columnRequirement struct {
	table     string
	name      string
	kind      sqlmigration.ColumnKind
	nullable  bool
	minLength int
}

var requiredColumns = []columnRequirement{
	{table: "crn_schedule", name: "id", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_schedule", name: "created_at", kind: sqlmigration.ColumnTimestamp},
	{table: "crn_schedule", name: "updated_at", kind: sqlmigration.ColumnTimestamp},
	{table: "crn_schedule", name: "created_by", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_schedule", name: "updated_by", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_schedule", name: "name", kind: sqlmigration.ColumnVarchar, minLength: 128},
	{table: "crn_schedule", name: "job_name", kind: sqlmigration.ColumnVarchar, minLength: 128},
	{table: "crn_schedule", name: "kind", kind: sqlmigration.ColumnVarchar, minLength: 16},
	{table: "crn_schedule", name: "expr", kind: sqlmigration.ColumnText},
	{table: "crn_schedule", name: "timezone", kind: sqlmigration.ColumnVarchar, minLength: 64},
	{table: "crn_schedule", name: "every_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_schedule", name: "fire_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_schedule", name: "starts_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_schedule", name: "ends_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_schedule", name: "anchor_at_unix_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_schedule", name: "params", kind: sqlmigration.ColumnJSON, nullable: true},
	{table: "crn_schedule", name: "misfire_policy", kind: sqlmigration.ColumnVarchar, minLength: 16},
	{table: "crn_schedule", name: "concurrency_policy", kind: sqlmigration.ColumnVarchar, minLength: 16},
	{table: "crn_schedule", name: "recover", kind: sqlmigration.ColumnBool},
	{table: "crn_schedule", name: "timeout_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_schedule", name: "is_enabled", kind: sqlmigration.ColumnBool},
	{table: "crn_schedule", name: "next_fire_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_schedule", name: "last_fire_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_fire_request", name: "id", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_fire_request", name: "schedule_id", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_fire_request", name: "kind", kind: sqlmigration.ColumnVarchar, minLength: 16},
	{table: "crn_fire_request", name: "scheduled_at_unix_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_fire_request", name: "source_run_id", kind: sqlmigration.ColumnVarchar, nullable: true, minLength: 32},
	{table: "crn_run", name: "id", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_run", name: "created_at", kind: sqlmigration.ColumnTimestamp},
	{table: "crn_run", name: "created_by", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_run", name: "schedule_id", kind: sqlmigration.ColumnVarchar, minLength: 32},
	{table: "crn_run", name: "schedule_name", kind: sqlmigration.ColumnVarchar, minLength: 128},
	{table: "crn_run", name: "job_name", kind: sqlmigration.ColumnVarchar, minLength: 128},
	{table: "crn_run", name: "scheduled_at_unix_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_run", name: "claimed_at_unix_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_run", name: "status", kind: sqlmigration.ColumnVarchar, minLength: 16},
	{table: "crn_run", name: "node_id", kind: sqlmigration.ColumnText},
	{table: "crn_run", name: "started_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_run", name: "finished_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_run", name: "duration_ms", kind: sqlmigration.ColumnInt64},
	{table: "crn_run", name: "heartbeat_at_unix_ms", kind: sqlmigration.ColumnInt64, nullable: true},
	{table: "crn_run", name: "error", kind: sqlmigration.ColumnText},
	{table: "crn_run", name: "missed_count", kind: sqlmigration.ColumnInt32},
}

// retiredColumn names a column an earlier schema revision carried; its
// presence marks a schema this version must not run against.
type retiredColumn struct {
	table string
	name  string
}

var retiredTimelineColumns = []retiredColumn{
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

// indexRequirement declares one index capability: an index with exactly
// these key columns (and uniqueness). Names matter only for diagnostics.
type indexRequirement struct {
	table   string
	name    string
	columns []string
	unique  bool
}

var requiredIndexes = []indexRequirement{
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
	columnsByTable := make(map[string]map[string]sqlmigration.Column, len(expectedTables))
	indexesByTable := make(map[string][]sqlmigration.Index, len(expectedTables))

	for _, table := range expectedTables {
		exists, err := sqlmigration.TableExists(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("verify table %s: %w", table, err)
		}

		if !exists {
			return outdated("missing table %s", table)
		}

		columns, err := sqlmigration.LoadTableColumns(ctx, db, kind, table)
		if err != nil {
			return fmt.Errorf("load columns for %s: %w", table, err)
		}

		columnsByTable[table] = columns

		indexes, err := sqlmigration.LoadTableIndexes(ctx, db, kind, table)
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

func verifyColumnCapabilities(columnsByTable map[string]map[string]sqlmigration.Column) error {
	for _, required := range requiredColumns {
		actual, exists := columnsByTable[required.table][required.name]
		if !exists {
			return outdated("missing column %s.%s", required.table, required.name)
		}

		if actual.Kind != required.kind {
			return outdated(
				"column %s.%s has type %s, want %s",
				required.table,
				required.name,
				actual.Kind,
				required.kind,
			)
		}

		if actual.Nullable != required.nullable {
			return outdated(
				"column %s.%s nullable=%t, want nullable=%t",
				required.table,
				required.name,
				actual.Nullable,
				required.nullable,
			)
		}

		// Narrower columns truncate persisted identifiers; wider ones lose no
		// capability, so a DBA-widened column stays acceptable.
		if required.minLength > 0 && actual.MaxLength < required.minLength {
			return outdated(
				"column %s.%s has length %d, want at least %d",
				required.table,
				required.name,
				actual.MaxLength,
				required.minLength,
			)
		}
	}

	for _, retired := range retiredTimelineColumns {
		if _, exists := columnsByTable[retired.table][retired.name]; exists {
			return outdated("obsolete column %s.%s is present", retired.table, retired.name)
		}
	}

	return nil
}

// hasIndexCapability matches by uniqueness and exact column sequence; index
// names are irrelevant to capability.
func hasIndexCapability(indexes []sqlmigration.Index, required indexRequirement) bool {
	return slices.ContainsFunc(indexes, func(index sqlmigration.Index) bool {
		return index.Unique == required.unique && slices.Equal(index.Columns, required.columns)
	})
}

func outdated(format string, args ...any) error {
	detail := fmt.Sprintf(format, args...)

	return fmt.Errorf(
		"%w: %s; recreate the cron store tables with the schema required by this version",
		ErrSchemaOutdated,
		detail,
	)
}
