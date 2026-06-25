package storage

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// JSONStorage is the StorageJSON strategy. Form data lives solely in
// apv_instance.form_data (JSONB), which the start/resubmit handlers already
// write, so both lifecycle hooks are no-ops. It exists so the Dispatcher can
// treat JSON and Table uniformly.
type JSONStorage struct{}

// NewJSONStorage constructs a JSONStorage.
func NewJSONStorage() *JSONStorage { return &JSONStorage{} }

// ProvisionTable is a no-op: JSON mode needs no extra physical structure.
func (*JSONStorage) ProvisionTable(context.Context, orm.DB, *approval.Flow, *approval.FlowVersion) error {
	return nil
}

// RecordMetadata is a no-op: JSON mode records no projection metadata.
func (*JSONStorage) RecordMetadata(context.Context, orm.DB, *approval.Flow, *approval.FlowVersion) error {
	return nil
}

// Write is a no-op: the instance's form_data column is the storage.
func (*JSONStorage) Write(context.Context, orm.DB, *approval.Flow, *approval.FlowVersion, string, map[string]any) error {
	return nil
}
