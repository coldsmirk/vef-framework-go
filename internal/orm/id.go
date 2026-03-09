package orm

import (
	"reflect"

	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/id"
)

// IDHandler generates unique IDs for string primary key fields on insert.
type IDHandler struct{}

// OnInsert generates an ID for zero-valued string PK fields.
func (*IDHandler) OnInsert(_ *BunInsertQuery, _ *schema.Table, field *schema.Field, _ any, value reflect.Value) {
	if field.IsPK && field.IndirectType.Kind() == reflect.String && value.IsZero() {
		value.SetString(id.Generate())
	}
}

func (*IDHandler) Name() string {
	return ColumnID
}
