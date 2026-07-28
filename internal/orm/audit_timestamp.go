package orm

import (
	"reflect"

	"github.com/uptrace/bun/schema"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// CreatedAtHandler sets created_at timestamps on insert.
type CreatedAtHandler struct{}

func (*CreatedAtHandler) OnInsert(_ *BunInsertQuery, _ *schema.Table, _ *schema.Field, _ any, value reflect.Value) {
	if value.IsZero() {
		value.Set(reflect.ValueOf(timex.Now()))
	}
}

func (*CreatedAtHandler) Name() string {
	return ColumnCreatedAt
}

// UpdatedAtHandler sets updated_at timestamps on insert and update.
type UpdatedAtHandler struct{}

func (ua *UpdatedAtHandler) OnUpdate(query *BunUpdateQuery, _ *schema.Table, _ *schema.Field, _ any, value reflect.Value) {
	switch query.resolveAutoColumnTarget(ua.Name(), value) {
	case autoColumnOnModel:
		value.Set(reflect.ValueOf(timex.Now()))
	case autoColumnOnQuery:
		query.Set(ua.Name(), timex.Now())
	case autoColumnSkip:
	}
}

func (*UpdatedAtHandler) OnInsert(_ *BunInsertQuery, _ *schema.Table, _ *schema.Field, _ any, value reflect.Value) {
	if value.IsZero() {
		value.Set(reflect.ValueOf(timex.Now()))
	}
}

func (*UpdatedAtHandler) Name() string {
	return ColumnUpdatedAt
}
