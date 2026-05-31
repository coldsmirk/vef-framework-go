package datasource

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/config"
)

// Provider supplies additional data sources during application startup. The
// framework calls Load on every registered provider after the primary and
// static (TOML) data sources are already in the registry; each returned Spec is
// then Register'd. Provider order is undefined, and a name collision (with TOML
// or another provider) is reported as a startup failure.
type Provider interface {
	// Name identifies the provider in error messages and logs. It does not need
	// to be globally unique but should be descriptive (for example "tenant-table"
	// or "vault-secrets").
	Name() string
	// Load returns the data sources this provider knows about. It runs once
	// during the FX startup phase and any error aborts boot.
	Load(ctx context.Context) ([]Spec, error)
}

// Spec is the (name, config) pair produced by a Provider and consumed by
// Registry.Reconcile.
type Spec struct {
	// Name is the registry key. Must be non-empty and not equal to PrimaryName.
	Name string
	// Cfg is the connection configuration applied when opening the source.
	Cfg config.DataSourceConfig
}
