// Package storage implements the form-data storage strategies for the approval
// module. A FlowVersion's StorageMode selects, at publish time, where the
// instances' form data physically lives:
//
//   - StorageJSON keeps form data only in apv_instance.form_data (JSONB). The
//     JSONStorage strategy is a no-op for both publish and write — the command
//     handlers already persist that column.
//
//   - StorageTable additionally projects each instance's form data into
//     dedicated physical tables generated for that version (a main table plus
//     one child table per detail-table field, immutable once published).
//     apv_instance.form_data stays populated so every existing read path
//     (validation, engine, get_instance_detail) keeps working unchanged; the
//     physical tables are the structured, queryable copy.
//
// The Dispatcher picks the strategy from version.StorageMode and is the only
// type the command handlers depend on.
package storage

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// FormStorage abstracts where an approval flow's form data is physically
// stored. Implementations are selected per version by StorageMode.
//
// Provisioning a version's storage is split into two phases with different
// transactional requirements. ProvisionTable runs OUTSIDE the publish
// transaction because DDL cannot take part in a transaction on MySQL (CREATE
// TABLE triggers an implicit commit of the in-flight transaction); RecordMetadata
// and Write run INSIDE their caller's transaction so metadata and projection
// rows commit or roll back with the surrounding operation.
type FormStorage interface {
	// ProvisionTable creates whatever durable physical structure the mode
	// needs (TableStorage issues the DDL statements). It runs OUTSIDE the publish
	// transaction, on its own connection: a DDL statement implicitly commits
	// the active transaction on MySQL, so issuing it inside the publish
	// transaction would silently break that transaction's atomicity. It must be
	// idempotent (CREATE TABLE IF NOT EXISTS) so a retry after a rolled-back
	// publish reuses the structure. JSONStorage is a no-op.
	ProvisionTable(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion) error

	// RecordMetadata persists the generated structure's metadata
	// (apv_form_table / apv_form_table_column). It runs INSIDE the publish
	// transaction so the version's published state and its recorded schema
	// commit or roll back together. It must be idempotent: republishing — or a
	// retry after a partial failure — must not error or duplicate metadata.
	// JSONStorage is a no-op.
	RecordMetadata(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion) error

	// Write projects a single instance's form data into the mode's storage.
	// It runs after the instance row and its form_data column are already
	// persisted, so JSONStorage has nothing to do. formData is the full,
	// already-validated form payload for the instance. It is idempotent per
	// instance: a re-write (resubmit) replaces the prior projection rather than
	// appending to it.
	Write(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion, instanceID string, formData map[string]any) error
}

// Dispatcher routes FormStorage calls to the JSON or Table strategy based on a
// version's StorageMode. An unknown or empty StorageMode resolves to JSON so a
// version created before storage modes were plumbed (or with a blank value)
// keeps the original JSON-only behavior.
type Dispatcher struct {
	json  FormStorage
	table FormStorage
}

// NewDispatcher builds a Dispatcher over the two concrete strategies.
func NewDispatcher(jsonStorage *JSONStorage, tableStorage *TableStorage) *Dispatcher {
	return &Dispatcher{json: jsonStorage, table: tableStorage}
}

// For returns the FormStorage strategy for a storage mode.
func (d *Dispatcher) For(mode approval.StorageMode) FormStorage {
	if mode == approval.StorageTable {
		return d.table
	}

	return d.json
}

// ProvisionTable dispatches to the strategy for version.StorageMode. The caller
// must invoke it OUTSIDE the publish transaction (see FormStorage.ProvisionTable).
func (d *Dispatcher) ProvisionTable(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion) error {
	return d.For(version.StorageMode).ProvisionTable(ctx, db, flow, version)
}

// RecordMetadata dispatches to the strategy for version.StorageMode. The caller
// must invoke it INSIDE the publish transaction so metadata commits with the
// version's published state.
func (d *Dispatcher) RecordMetadata(ctx context.Context, db orm.DB, flow *approval.Flow, version *approval.FlowVersion) error {
	return d.For(version.StorageMode).RecordMetadata(ctx, db, flow, version)
}

// SyncInstanceProjection refreshes the table-mode physical projection for an
// instance from its current form data. It resolves the storage mode by loading
// the instance's version, so a caller that has just mutated form_data need not
// thread the version or flow through: one call after persisting form_data keeps
// the physical projection in lockstep with apv_instance.form_data. JSON mode is
// a no-op. Every form_data mutation funnels through this single entry point, so
// no write path can forget to refresh the projection.
func (d *Dispatcher) SyncInstanceProjection(ctx context.Context, db orm.DB, instance *approval.Instance) error {
	var version approval.FlowVersion

	version.ID = instance.FlowVersionID
	if err := db.NewSelect().
		Model(&version).
		Select("storage_mode").
		WherePK().
		Scan(ctx); err != nil {
		return fmt.Errorf("load version storage mode for projection: %w", err)
	}

	return d.For(version.StorageMode).Write(ctx, db, &approval.Flow{}, &version, instance.ID, instance.FormData)
}
