// Second fixture file in the models package, proving that directory mode emits
// one schema file per source file from a single package load.
package models

import (
	"github.com/coldsmirk/vef-framework-go/orm"
)

// Widget is a minimal model living in a separate file from sample.go.
type Widget struct {
	orm.BaseModel `bun:"table:widgets,alias:w"`

	ID   string `bun:"id,pk"`
	Name string `bun:"name"`
}
