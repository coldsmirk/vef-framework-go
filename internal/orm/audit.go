package orm

import (
	"reflect"

	"github.com/uptrace/bun/schema"

	collections "github.com/coldsmirk/go-collections"
)

// autoColumnHandlers manages audit fields (ID, timestamps, user tracking) on insert/update.
var autoColumnHandlers = []ColumnHandler{
	new(IDHandler),
	new(CreatedAtHandler),
	new(UpdatedAtHandler),
	new(CreatedByHandler),
	new(UpdatedByHandler),
}

// autoColumnPlanItem binds a schema field to the handler callback that sets it.
// apply is the per-direction terminal call (OnInsert or OnUpdate) captured at plan
// build time, so the slice-recursion driver is shared across insert and update.
type autoColumnPlanItem[Q any] struct {
	field *schema.Field
	apply func(query Q, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

var (
	insertAutoColumnPlanCache = collections.NewConcurrentHashMap[*schema.Table, []autoColumnPlanItem[*BunInsertQuery]]()
	updateAutoColumnPlanCache = collections.NewConcurrentHashMap[*schema.Table, []autoColumnPlanItem[*BunUpdateQuery]]()
)

// ColumnHandler provides the column name that the handler manages.
type ColumnHandler interface {
	// Name returns the database column name this handler manages (e.g., "id", "created_at").
	Name() string
}

// InsertColumnHandler manages columns automatically during insert operations.
type InsertColumnHandler interface {
	ColumnHandler
	// OnInsert sets the column value automatically when a new row is inserted.
	OnInsert(query *BunInsertQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// UpdateColumnHandler manages columns during both insert and update operations.
type UpdateColumnHandler interface {
	InsertColumnHandler
	// OnUpdate sets the column value automatically when an existing row is updated.
	OnUpdate(query *BunUpdateQuery, table *schema.Table, field *schema.Field, model any, value reflect.Value)
}

// processAutoColumns applies auto column handlers to a model before insert/update operations.
// Callers pass mv = reflect.Indirect(reflect.ValueOf(modelValue)), so a nil pointer model has
// already collapsed to an invalid Value and the single validity check covers it.
func processAutoColumns(query any, table *schema.Table, modelValue any, mv reflect.Value) {
	if !mv.IsValid() {
		return
	}

	switch q := query.(type) {
	case *BunInsertQuery:
		applyAutoColumns(q, table, modelValue, mv, getInsertAutoColumnPlan(table))
	case *BunUpdateQuery:
		applyAutoColumns(q, table, modelValue, mv, getUpdateAutoColumnPlan(table))
	}
}

func getInsertAutoColumnPlan(table *schema.Table) []autoColumnPlanItem[*BunInsertQuery] {
	plan, _ := insertAutoColumnPlanCache.GetOrCompute(table, func() []autoColumnPlanItem[*BunInsertQuery] {
		return buildAutoColumnPlan(table, func(h InsertColumnHandler) func(*BunInsertQuery, *schema.Table, *schema.Field, any, reflect.Value) {
			return h.OnInsert
		})
	})

	return plan
}

func getUpdateAutoColumnPlan(table *schema.Table) []autoColumnPlanItem[*BunUpdateQuery] {
	plan, _ := updateAutoColumnPlanCache.GetOrCompute(table, func() []autoColumnPlanItem[*BunUpdateQuery] {
		return buildAutoColumnPlan(table, func(h UpdateColumnHandler) func(*BunUpdateQuery, *schema.Table, *schema.Field, any, reflect.Value) {
			return h.OnUpdate
		})
	})

	return plan
}

// buildAutoColumnPlan builds the ordered auto-column plan for one direction. It
// keeps the handler/field iteration in one place; bind selects the typed terminal
// callback (OnInsert or OnUpdate) for each handler that implements H.
func buildAutoColumnPlan[Q any, H ColumnHandler](
	table *schema.Table,
	bind func(H) func(Q, *schema.Table, *schema.Field, any, reflect.Value),
) []autoColumnPlanItem[Q] {
	items := make([]autoColumnPlanItem[Q], 0, len(autoColumnHandlers))
	for _, handler := range autoColumnHandlers {
		typed, ok := handler.(H)
		if !ok {
			continue
		}

		field, ok := table.FieldMap[handler.Name()]
		if !ok {
			continue
		}

		items = append(items, autoColumnPlanItem[Q]{field: field, apply: bind(typed)})
	}

	return items
}

// applyAutoColumns runs an auto-column plan against a model, recursing into slice
// elements for batch operations. The per-direction terminal call lives on each
// plan item, so this driver is shared by insert and update.
func applyAutoColumns[Q any](
	query Q,
	table *schema.Table,
	modelValue any,
	mv reflect.Value,
	plan []autoColumnPlanItem[Q],
) {
	if mv.Kind() == reflect.Slice {
		for i := range mv.Len() {
			elem := mv.Index(i)
			if elem.Kind() == reflect.Pointer {
				if elem.IsNil() {
					continue
				}

				elem = elem.Elem()
			}

			if !elem.IsValid() {
				continue
			}

			applyAutoColumns(query, table, elem.Interface(), elem, plan)
		}

		return
	}

	for _, item := range plan {
		item.apply(query, table, item.field, modelValue, item.field.Value(mv))
	}
}
