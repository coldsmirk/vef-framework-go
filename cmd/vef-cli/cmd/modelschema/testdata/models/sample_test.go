// Test-file fixture: models declared in _test.go files are not part of the
// primary package, so directory mode must ignore this file entirely instead of
// failing on it (the historical per-file loader errored here).
package models

import (
	"github.com/coldsmirk/vef-framework-go/orm"
)

// TestOnlyModel must never get a generated schema file.
type TestOnlyModel struct {
	orm.BaseModel `bun:"table:test_only_models"`

	ID string `bun:"id,pk"`
}
